use serde::Deserialize;
use tauri::{
    menu::{Menu, MenuItem, PredefinedMenuItem},
    tray::TrayIconBuilder,
    AppHandle, Manager,
};

struct TrayMenu {
    show: MenuItem<tauri::Wry>,
    quit: MenuItem<tauri::Wry>,
}

#[derive(Deserialize)]
struct Messages {
    desktop: DesktopMessages,
}

#[derive(Deserialize)]
struct DesktopMessages {
    tray: TrayMessages,
}

#[derive(Deserialize)]
struct TrayMessages {
    show: String,
    quit: String,
}

fn messages(locale: &str) -> Result<TrayMessages, serde_json::Error> {
    let catalog = if locale == "zh-CN" {
        include_str!("../../src/locales/zh-CN.json")
    } else {
        include_str!("../../src/locales/en-US.json")
    };
    Ok(serde_json::from_str::<Messages>(catalog)?.desktop.tray)
}

pub fn show_main_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}

pub fn init(app: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    let labels = messages("en-US")?;
    let show = MenuItem::with_id(app, "show-main", labels.show, true, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit-app", labels.quit, true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&show, &PredefinedMenuItem::separator(app)?, &quit])?;
    TrayIconBuilder::with_id("main")
        .icon(
            app.default_window_icon()
                .ok_or("missing_tray_icon")?
                .clone(),
        )
        .tooltip("One Desktop")
        .menu(&menu)
        .show_menu_on_left_click(true)
        .on_menu_event(|app, event| match event.id.as_ref() {
            "show-main" => show_main_window(app),
            "quit-app" => super::request_exit(app.clone()),
            _ => {}
        })
        .build(app)?;
    app.manage(TrayMenu { show, quit });
    Ok(())
}

pub fn set_locale(app: &AppHandle, locale: &str) -> Result<(), String> {
    let labels = messages(locale).map_err(|e| e.to_string())?;
    let menu = app.state::<TrayMenu>();
    menu.show.set_text(labels.show).map_err(|e| e.to_string())?;
    menu.quit.set_text(labels.quit).map_err(|e| e.to_string())?;
    Ok(())
}
