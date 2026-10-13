use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    io::{BufRead, BufReader, Read, Write},
    process::{Command, Stdio},
    sync::{mpsc, Arc, Mutex},
    time::{Duration, Instant},
};
use tauri::{AppHandle, Emitter};
use uuid::Uuid;

#[derive(Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Manifest {
    pub version: String,
    pub commit: String,
    pub channel: String,
    pub protocol: u32,
    pub api: u32,
    pub target: String,
    pub sha256: String,
}
pub fn manifest() -> Manifest {
    serde_json::from_str(include_str!("../bundle-manifest.json")).expect("build manifest")
}

#[derive(Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Snapshot {
    pub sequence: u64,
    pub status: String,
    pub generation: String,
    pub port: u16,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub token: String,
    pub error: String,
}
pub enum Control {
    Stop(mpsc::SyncSender<()>),
    Quiesce(mpsc::SyncSender<usize>),
    Resume,
    Retry,
}
pub struct Backend {
    pub snapshot: Arc<Mutex<Snapshot>>,
    pub control: mpsc::Sender<Control>,
}
fn publish(app: &AppHandle, state: &Arc<Mutex<Snapshot>>, status: &str, error: &str) {
    let mut s = state.lock().unwrap();
    s.sequence += 1;
    s.status = status.into();
    s.error = error.into();
    if status != "httpReady" {
        s.token.clear();
        s.port = 0;
    }
    // Event payloads must never contain session credentials.
    let _ = app.emit_to(
        "main",
        "backend-state",
        json!({"sequence":s.sequence,"status":s.status,"generation":s.generation,"error":s.error}),
    );
}

pub fn start(app: AppHandle) -> Backend {
    let state = Arc::new(Mutex::new(Snapshot {
        status: "starting".into(),
        ..Default::default()
    }));
    let (tx, rx) = mpsc::channel();
    let shared = state.clone();
    std::thread::spawn(move || supervise(app, shared, rx));
    Backend {
        snapshot: state,
        control: tx,
    }
}

fn supervise(app: AppHandle, state: Arc<Mutex<Snapshot>>, controls: mpsc::Receiver<Control>) {
    loop {
        publish(&app, &state, "starting", "");
        let result = run_child(&app, &state, &controls);
        match result {
            Ok(true) => return,
            Ok(false) => {
                continue;
            }
            Err(error) => {
                publish(&app, &state, "failed", &error);

                // Recovery is explicit: a deliberate retry creates a new session.
                // Never automatically replay operations after a crash.
                loop {
                    match controls.recv() {
                        Ok(Control::Retry) => break,
                        Ok(Control::Stop(done)) => {
                            let _ = done.send(());
                            return;
                        }
                        Ok(Control::Quiesce(done)) => {
                            let _ = done.send(0);
                        }
                        Ok(_) => (),
                        Err(_) => return,
                    }
                }
            }
        }
    }
}

fn run_child(
    app: &AppHandle,
    state: &Arc<Mutex<Snapshot>>,
    controls: &mpsc::Receiver<Control>,
) -> Result<bool, String> {
    let meta = manifest();
    let exe = std::env::current_exe().map_err(|e| e.to_string())?;
    let path = exe.parent().ok_or("backend_path")?.join(if cfg!(windows) {
        "one-api.exe"
    } else {
        "one-api"
    });
    // Tauri places externalBin next to the native executable, including inside
    // AppImage/usr/bin. Never search PATH or download a replacement here.
    if !path.is_file() {
        return Err("backend_missing".into());
    }
    let generation = Uuid::new_v4().to_string();
    let mut random = [0u8; 32];
    getrandom::fill(&mut random).map_err(|e| e.to_string())?;
    let token: String = random.iter().map(|byte| format!("{byte:02x}")).collect();
    let origin = if tauri::is_dev() {
        "http://127.0.0.1:5173"
    } else if cfg!(windows) {
        "http://tauri.localhost"
    } else {
        "tauri://localhost"
    };
    let mut child = OwnedChild(
        Command::new(&path)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .map_err(|e| e.to_string())?,
    );
    let mut input = child.stdin.take().ok_or("backend_stdin")?;
    // Dedicated reader has a bounded frame and channel, even for a broken binary.
    let output = child.stdout.take().ok_or("backend_stdout")?;
    let (lines_tx, lines) = mpsc::sync_channel(16);
    std::thread::spawn(move || {
        let mut reader = BufReader::new(output);
        loop {
            let mut line = String::new();
            match reader.by_ref().take(16385).read_line(&mut line) {
                Ok(0) | Err(_) => break,
                Ok(_) if line.len() > 16384 || !line.ends_with('\n') => break,
                Ok(_) => {
                    if lines_tx.send(line).is_err() {
                        break;
                    }
                }
            }
        }
    });
    let init = json!({"type":"bootstrap","protocol":meta.protocol,"token":token,"generation":generation,"origin":origin});
    if writeln!(input, "{init}").is_err() {
        let _ = child.kill();
        let _ = child.wait();
        return Err("backend_handshake".into());
    }
    let started = Instant::now();
    let mut ready = false;
    let mut pending_quiesce = None;
    let mut stop: Option<(Instant, mpsc::SyncSender<()>)> = None;
    loop {
        while let Ok(line) = lines.try_recv() {
            let value: Value = serde_json::from_str(&line).map_err(|_| "backend_protocol")?;
            match value["type"].as_str() {
                Some("ready") if !ready => {
                    let port = value["port"].as_u64().filter(|p| *p > 0 && *p <= 65535);
                    if port.is_none()
                        || value["generation"] != generation
                        || value["version"] != meta.version
                        || value["commit"] != meta.commit
                        || value["protocol"] != meta.protocol
                        || value["api"] != meta.api
                        || value["channel"] != meta.channel
                    {
                        let _ = child.kill();
                        let _ = child.wait();
                        return Err("backend_identity".into());
                    }
                    {
                        let mut s = state.lock().unwrap();
                        s.generation = generation.clone();
                        s.token = token.clone();
                        s.port = port.unwrap() as u16;
                    }
                    publish(app, state, "httpReady", "");
                    ready = true;
                }
                Some("quiesced") => {
                    if let Some(reply) = pending_quiesce.take() {
                        let reply: mpsc::SyncSender<usize> = reply;
                        let _ = reply.send(value["active"].as_u64().unwrap_or(1) as usize);
                    }
                }
                _ => (),
            }
        }
        if let Some(status) = child.try_wait().map_err(|e| e.to_string())? {
            publish(app, state, "stopped", "");
            if let Some((_, reply)) = stop.take() {
                let _ = reply.send(());
                return Ok(true);
            }
            return Err(format!("backend_exited ({status})"));
        }
        if !ready && started.elapsed() > Duration::from_secs(15) {
            let _ = child.kill();
            let _ = child.wait();
            return Err("backend_timeout".into());
        }
        if stop
            .as_ref()
            .is_some_and(|(time, _)| time.elapsed() > Duration::from_secs(12))
        {
            let _ = child.kill();
            let _ = child.wait();
            if let Some((_, reply)) = stop.take() {
                let _ = reply.send(());
            }
            return Ok(true);
        }
        match controls.recv_timeout(Duration::from_millis(30)) {
            Ok(Control::Quiesce(reply)) => {
                pending_quiesce = Some(reply);
                let _ = writeln!(input, "{{\"type\":\"quiesce\"}}");
            }
            Ok(Control::Resume) => {
                let _ = writeln!(input, "{{\"type\":\"resume\"}}");
            }
            Ok(Control::Stop(reply)) => {
                publish(app, state, "stopping", "");
                let _ = writeln!(input, "{{\"type\":\"shutdown\"}}");
                stop = Some((Instant::now(), reply));
            }
            Ok(Control::Retry) => (),
            Err(mpsc::RecvTimeoutError::Disconnected) => {
                drop(input);
                let _ = child.wait();
                return Ok(true);
            }
            Err(mpsc::RecvTimeoutError::Timeout) => (),
        }
    }
}

struct OwnedChild(std::process::Child);
impl std::ops::Deref for OwnedChild {
    type Target = std::process::Child;
    fn deref(&self) -> &Self::Target {
        &self.0
    }
}
impl std::ops::DerefMut for OwnedChild {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}
impl Drop for OwnedChild {
    fn drop(&mut self) {
        if self.0.try_wait().ok().flatten().is_none() {
            let _ = self.0.kill();
        }
        let _ = self.0.wait();
    }
}
