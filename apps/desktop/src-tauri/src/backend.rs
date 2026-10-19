use serde::{Deserialize, Serialize};
use std::{
    process::Command,
    sync::{mpsc, Arc, Mutex},
};
use tauri::{AppHandle, Emitter};

#[derive(Clone, Deserialize, Serialize)]
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
    #[serde(skip)]
    pub storage_root: std::path::PathBuf,
}
#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Connection {
    url: String,
    token: String,
    generation: String,
    port: u16,
    storage_root: std::path::PathBuf,
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
    let _=app.emit_to("main","backend-state",serde_json::json!({"sequence":s.sequence,"status":s.status,"generation":s.generation,"error":s.error}));
}
pub fn bundled_binary(name: &str) -> Result<std::path::PathBuf, String> {
    let exe = std::env::current_exe().map_err(|e| e.to_string())?;
    let file = if cfg!(windows) {
        format!("{name}.exe")
    } else {
        name.into()
    };
    let path = exe.parent().ok_or("backend_path")?.join(file);
    if !path.is_file() {
        return Err("backend_missing".into());
    };
    Ok(path)
}
fn connect(app: &AppHandle, state: &Arc<Mutex<Snapshot>>) -> Result<(), String> {
    publish(app, state, "starting", "");
    let mut command = Command::new(bundled_binary("one-api")?);
    command.arg("ensure");
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        command.creation_flags(0x08000000);
    }
    let output = command.output().map_err(|e| e.to_string())?;
    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
    }
    let conn: Connection =
        serde_json::from_slice(&output.stdout).map_err(|_| "backend_protocol")?;
    if conn.url != format!("http://127.0.0.1:{}", conn.port)
        || conn.token.len() != 64
        || conn.generation.is_empty()
        || !conn.storage_root.is_absolute()
    {
        return Err("backend_identity".into());
    }
    {
        let mut s = state.lock().unwrap();
        s.generation = conn.generation;
        s.port = conn.port;
        s.token = conn.token;
        s.storage_root = conn.storage_root;
    }
    publish(app, state, "httpReady", "");
    Ok(())
}
pub fn start(app: AppHandle) -> Backend {
    let state = Arc::new(Mutex::new(Snapshot {
        status: "starting".into(),
        ..Default::default()
    }));
    let (tx, rx) = mpsc::channel();
    let shared = state.clone();
    std::thread::spawn(move || {
        while let Ok(control) = rx.recv() {
            match control {
                Control::Stop(done) => {
                    publish(&app, &shared, "stopped", "");
                    let _ = done.send(());
                    return;
                }
                Control::Quiesce(done) => {
                    let _ = done.send(0);
                }
                Control::Resume => {}
                Control::Retry => {
                    if let Err(error) = connect(&app, &shared) {
                        publish(&app, &shared, "failed", &error)
                    }
                }
            }
        }
    });
    Backend {
        snapshot: state,
        control: tx,
    }
}
