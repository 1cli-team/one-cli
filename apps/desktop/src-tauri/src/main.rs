#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]
mod backend;
mod lifecycle;
mod updates;
use backend::{Backend, Control};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    mpsc, Mutex,
};
use std::time::Duration;
use tauri::{Manager, WebviewWindow};
use tauri_plugin_dialog::{DialogExt, MessageDialogButtons};
use tauri_plugin_opener::OpenerExt;

struct NativeState {
    locale: Mutex<String>,
    closing: AtomicBool,
}
fn trusted(window: &WebviewWindow) -> Result<(), String> {
    let url = window.url().map_err(|e| e.to_string())?;
    let valid = if tauri::is_dev() {
        url.origin().ascii_serialization() == "http://127.0.0.1:5173"
    } else {
        url.scheme() == "tauri" && url.host_str() == Some("localhost")
            || cfg!(windows) && url.origin().ascii_serialization() == "http://tauri.localhost"
    };
    if window.label() != "main" || !valid {
        return Err("untrusted_window".into());
    }
    Ok(())
}
#[tauri::command]
fn get_backend_connection(
    window: WebviewWindow,
    state: tauri::State<Backend>,
) -> Result<backend::Snapshot, String> {
    trusted(&window)?;
    Ok(state.snapshot.lock().unwrap().clone())
}
#[tauri::command]
fn retry_backend(window: WebviewWindow, state: tauri::State<Backend>) -> Result<(), String> {
    trusted(&window)?;
    state
        .control
        .send(Control::Retry)
        .map_err(|e| e.to_string())
}
#[tauri::command]
fn set_native_locale(
    window: WebviewWindow,
    app: tauri::AppHandle,
    state: tauri::State<NativeState>,
    locale: String,
) -> Result<(), String> {
    trusted(&window)?;
    if locale != "zh-CN" && locale != "en-US" {
        return Err("invalid_locale".into());
    };
    lifecycle::set_locale(&app, &locale)?;
    *state.locale.lock().unwrap() = locale;
    Ok(())
}
#[tauri::command]
async fn choose_workspace(
    window: WebviewWindow,
    app: tauri::AppHandle,
) -> Result<Option<String>, String> {
    trusted(&window)?;
    tauri::async_runtime::spawn_blocking(move || {
        app.dialog()
            .file()
            .blocking_pick_folder()
            .map(|p| {
                p.into_path()
                    .map(|p| p.to_string_lossy().to_string())
                    .map_err(|e| e.to_string())
            })
            .transpose()
    })
    .await
    .map_err(|e| e.to_string())?
}
#[tauri::command]
fn open_external(window: WebviewWindow, app: tauri::AppHandle, url: String) -> Result<(), String> {
    trusted(&window)?;
    let parsed: tauri::Url = url.parse().map_err(|_| "invalid_url")?;
    if !["https", "http"].contains(&parsed.scheme())
        || parsed.host_str().is_none()
        || !parsed.username().is_empty()
        || parsed.password().is_some()
    {
        return Err("invalid_url".into());
    }
    app.opener()
        .open_url(url, None::<&str>)
        .map_err(|e| e.to_string())
}
// Called off the main thread, so native dialogs and the window remain responsive.
fn stop_backend(app: &tauri::AppHandle) -> Result<bool, String> {
    let backend = app.state::<Backend>();
    let (tx, rx) = mpsc::sync_channel(1);
    backend
        .control
        .send(Control::Quiesce(tx))
        .map_err(|e| e.to_string())?;
    let active = rx
        .recv_timeout(Duration::from_secs(4))
        .map_err(|_| "backend_busy_timeout")?;
    if active > 0 {
        let zh = *app.state::<NativeState>().locale.lock().unwrap() == "zh-CN";
        let approved = app
            .dialog()
            .message(if zh {
                "当前仍有操作正在执行。结束操作并退出？"
            } else {
                "Operations are still running. Stop them and exit?"
            })
            .title("One Desktop")
            .buttons(MessageDialogButtons::OkCancelCustom(
                if zh {
                    "结束并退出"
                } else {
                    "Stop and exit"
                }
                .into(),
                if zh { "取消" } else { "Cancel" }.into(),
            ))
            .blocking_show();
        if !approved {
            let _ = backend.control.send(Control::Resume);
            return Ok(false);
        }
    }
    let (tx, rx) = mpsc::sync_channel(1);
    backend
        .control
        .send(Control::Stop(tx))
        .map_err(|e| e.to_string())?;
    rx.recv_timeout(Duration::from_secs(15))
        .map_err(|_| "backend_stop_timeout")?;
    Ok(true)
}
fn request_exit(app: tauri::AppHandle) {
    if app
        .state::<NativeState>()
        .closing
        .swap(true, Ordering::SeqCst)
    {
        return;
    }
    std::thread::spawn(move || match stop_backend(&app) {
        Ok(true) => app.exit(0),
        Ok(false) => {
            app.state::<NativeState>()
                .closing
                .store(false, Ordering::SeqCst);
        }
        Err(error) => {
            app.state::<NativeState>()
                .closing
                .store(false, Ordering::SeqCst);
            app.dialog()
                .message(error)
                .title("One Desktop")
                .blocking_show();
        }
    });
}
fn main() {
    let app = tauri::Builder::default()
        .plugin(
            tauri_plugin_single_instance::Builder::new()
                // D-Bus name elements cannot start with a digit, unlike bundle IDs.
                .dbus_id("team.onecli.desktop")
                .callback(|app, _, _| {
                    lifecycle::show_main_window(app);
                })
                .build(),
        )
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .manage(NativeState {
            locale: Mutex::new("en-US".into()),
            closing: AtomicBool::new(false),
        })
        .manage(updates::Updates::default())
        .setup(|app| {
            lifecycle::init(app.handle())?;
            if let Some(key) = updates::public_key() {
                app.handle()
                    .plugin(tauri_plugin_updater::Builder::new().pubkey(key).build())?;
            }
            let config = app.config().app.windows[0].clone();
            tauri::WebviewWindowBuilder::from_config(app, &config)?
                .on_navigation(|url| {
                    if tauri::is_dev() {
                        url.origin().ascii_serialization() == "http://127.0.0.1:5173"
                    } else {
                        url.scheme() == "tauri" && url.host_str() == Some("localhost")
                            || cfg!(windows)
                                && url.origin().ascii_serialization() == "http://tauri.localhost"
                    }
                })
                .build()?;
            app.manage(backend::start(app.handle().clone()));
            Ok(())
        })
        .on_page_load(|webview, _| {
            let _ = webview;
        })
        .on_window_event(|w, e| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = e {
                api.prevent_close();
                let _ = w.hide();
            }
        })
        .invoke_handler(tauri::generate_handler![
            get_backend_connection,
            retry_backend,
            set_native_locale,
            choose_workspace,
            open_external,
            updates::check_update,
            updates::install_update,
            updates::confirm_health
        ])
        .build(tauri::generate_context!())
        .expect("One Desktop initialization failed");
    app.run(|app, event| match event {
        #[cfg(target_os = "macos")]
        tauri::RunEvent::Reopen { .. } => lifecycle::show_main_window(app),
        tauri::RunEvent::ExitRequested { api, .. } => {
            if !app.state::<NativeState>().closing.load(Ordering::SeqCst) {
                api.prevent_exit();
                request_exit(app.clone());
            }
        }
        _ => {}
    });
}
