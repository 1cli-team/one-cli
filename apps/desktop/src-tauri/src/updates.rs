use crate::{backend, stop_backend, trusted};
use serde::Serialize;
use std::{
    fs,
    sync::{
        atomic::{AtomicBool, Ordering},
        Mutex,
    },
};
use tauri::{Manager, WebviewWindow};
use tauri_plugin_updater::UpdaterExt;

pub fn public_key() -> Option<&'static str> {
    option_env!("ONE_DESKTOP_UPDATER_PUBLIC_KEY")
        .filter(|key| !key.is_empty() && backend::manifest().channel == "release")
}

#[derive(Default)]
pub struct Updates {
    busy: AtomicBool,
    candidate: Mutex<Option<tauri_plugin_updater::Update>>,
}
struct Busy<'a>(&'a AtomicBool);
impl Drop for Busy<'_> {
    fn drop(&mut self) {
        self.0.store(false, Ordering::SeqCst)
    }
}
#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateInfo {
    enabled: bool,
    version: Option<String>,
    notes: Option<String>,
    reason: Option<String>,
}
#[tauri::command]
pub async fn check_update(
    window: WebviewWindow,
    app: tauri::AppHandle,
) -> Result<UpdateInfo, String> {
    trusted(&window)?;
    let state = app.state::<Updates>();
    if state.busy.swap(true, Ordering::SeqCst) {
        return Err("update_busy".into());
    }
    let _busy = Busy(&state.busy);
    let Some(key) = public_key() else {
        return Ok(UpdateInfo {
            enabled: false,
            version: None,
            notes: None,
            reason: Some("update_unconfigured".into()),
        });
    };
    let updater = app
        .updater_builder()
        .pubkey(key)
        .endpoints(vec![
            "https://github.com/1cli-team/one-cli/releases/latest/download/desktop-latest.json"
                .parse()
                .unwrap(),
        ])
        .map_err(|e| e.to_string())?
        .build()
        .map_err(|e| e.to_string())?;
    let update = updater.check().await.map_err(|e| e.to_string())?;
    let info = UpdateInfo {
        enabled: true,
        version: update.as_ref().map(|u| u.version.clone()),
        notes: update.as_ref().and_then(|u| u.body.clone()),
        reason: None,
    };
    *state.candidate.lock().unwrap() = update;
    Ok(info)
}
#[tauri::command]
pub async fn install_update(window: WebviewWindow, app: tauri::AppHandle) -> Result<(), String> {
    trusted(&window)?;
    let state = app.state::<Updates>();
    if state.busy.swap(true, Ordering::SeqCst) {
        return Err("update_busy".into());
    }
    let _busy = Busy(&state.busy);
    // Enabled only for a platform with a verified recovery procedure. This is a
    // release input, never an IPC flag the renderer can turn on.
    if option_env!("ONE_DESKTOP_RECOVERY_VERIFIED") != Some("1") {
        return Err("update_recovery_required".into());
    }
    let update = state
        .candidate
        .lock()
        .unwrap()
        .take()
        .ok_or("update_check_required")?;
    let bytes = update
        .download(|_, _| {}, || {})
        .await
        .map_err(|e| e.to_string())?; // Official signature verification.
    let handle = app.clone();
    if !tauri::async_runtime::spawn_blocking(move || stop_backend(&handle))
        .await
        .map_err(|e| e.to_string())??
    {
        return Ok(());
    }
    let dir = storage_directory(&app)?;
    fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    let transaction = serde_json::json!({"from":backend::manifest().version,"to":update.version,"phase":"installing"});
    fs::write(dir.join("update-transaction.json"), transaction.to_string())
        .map_err(|e| e.to_string())?;
    let handle = app.clone();
    tauri::async_runtime::spawn_blocking(move || {
        update.install(bytes).map_err(|e| e.to_string())?;
        handle.restart();
    })
    .await
    .map_err(|e| e.to_string())?
}
#[tauri::command]
pub fn confirm_health(
    window: WebviewWindow,
    app: tauri::AppHandle,
    generation: String,
) -> Result<(), String> {
    trusted(&window)?;
    let backend = app.state::<backend::Backend>();
    let s = backend.snapshot.lock().unwrap();
    if s.status != "httpReady" || s.generation != generation {
        return Err("backend_session_changed".into());
    }
    let dir = s.storage_root.join("state").join("desktop");
    let path = dir.join("update-transaction.json");
    if path.exists() {
        let mut record: serde_json::Value =
            serde_json::from_slice(&fs::read(&path).map_err(|e| e.to_string())?)
                .map_err(|e| e.to_string())?;
        if record["to"] == backend::manifest().version {
            record["phase"] = "healthy".into();
            fs::write(path, record.to_string()).map_err(|e| e.to_string())?;
        }
    }
    Ok(())
}

// Go is the single authority for ONE_HOME; retain its root after quiescing the backend.
fn storage_directory(app: &tauri::AppHandle) -> Result<std::path::PathBuf, String> {
    let backend = app.state::<backend::Backend>();
    let snapshot = backend.snapshot.lock().unwrap();
    if !snapshot.storage_root.is_absolute() {
        return Err("backend_storage".into());
    }
    Ok(snapshot.storage_root.join("state").join("desktop"))
}
