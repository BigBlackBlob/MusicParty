use serde::Serialize;
use std::fs::{create_dir_all, File, OpenOptions};
use std::io::Write;
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::Mutex;
use tauri::{AppHandle, Manager, State};

#[derive(Default)]
struct ProcessState {
    backend: Option<Child>,
    netease: Option<Child>,
    status: Option<DesktopHostStatus>,
}

fn bootstrap_log(message: &str) {
    let path = std::env::temp_dir().join("musicparty-desktop-bootstrap.log");
    if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(path) {
        let _ = writeln!(file, "{message}");
    }
}

#[derive(Serialize, Clone)]
#[serde(rename_all = "camelCase")]
struct DesktopHostStatus {
    hosting: bool,
    backend_url: String,
    invite_url: String,
    profile_dir: String,
    backend_port: u16,
    netease_api_url: String,
    netease_api_port: u16,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct DesktopLanAddress {
    label: String,
    url: String,
}

#[tauri::command]
fn desktop_status(state: State<'_, Mutex<ProcessState>>) -> Result<Option<DesktopHostStatus>, String> {
    let guard = state.lock().map_err(|_| "desktop state lock poisoned".to_string())?;
    Ok(guard.status.clone())
}

#[tauri::command]
fn desktop_lan_addresses(state: State<'_, Mutex<ProcessState>>) -> Result<Vec<DesktopLanAddress>, String> {
    let guard = state.lock().map_err(|_| "desktop state lock poisoned".to_string())?;
    let Some(status) = &guard.status else {
        return Ok(Vec::new());
    };

    Ok(vec![DesktopLanAddress {
        label: "Local machine".to_string(),
        url: status.invite_url.clone(),
    }])
}

#[tauri::command]
fn start_desktop_host(
    app: AppHandle,
    state: State<'_, Mutex<ProcessState>>,
    profile_dir: Option<String>,
    backend_jar: Option<String>,
    lan_base_url: Option<String>,
) -> Result<DesktopHostStatus, String> {
    start_desktop_host_inner(&app, &state, profile_dir, backend_jar, lan_base_url)
}

fn start_desktop_host_inner(
    app: &AppHandle,
    state: &Mutex<ProcessState>,
    profile_dir: Option<String>,
    backend_jar: Option<String>,
    lan_base_url: Option<String>,
) -> Result<DesktopHostStatus, String> {
    bootstrap_log("start_desktop_host_inner: begin");
    let mut guard = state.lock().map_err(|_| "desktop state lock poisoned".to_string())?;
    if let Some(status) = &guard.status {
        bootstrap_log("start_desktop_host_inner: already running");
        return Ok(status.clone());
    }

    let profile = profile_dir
        .map(PathBuf::from)
        .unwrap_or_else(|| default_profile_dir(app));
    bootstrap_log(&format!("profile_dir={}", profile.display()));
    create_profile_dirs(&profile)?;
    bootstrap_log("profile dirs created");

    let backend_port = free_port()?;
    let netease_port = free_port()?;
    let backend_url = format!("http://127.0.0.1:{backend_port}");
    let lan_url = normalize_base_url(lan_base_url).unwrap_or_else(|| backend_url.clone());
    let invite_url = format!("{}/room/lobby", lan_url);
    let netease_api_url = format!("http://127.0.0.1:{netease_port}");

    let backend_path = backend_jar
        .map(PathBuf::from)
        .unwrap_or_else(|| bundled_backend_jar(app));
    bootstrap_log(&format!("backend_jar={}", backend_path.display()));
    bootstrap_log(&format!("backend_jar_exists={}", backend_path.exists()));

    let backend_log = File::create(profile.join("logs").join("backend.log"))
        .map_err(|error| format!("create backend log failed: {error}"))?;

    let backend = Command::new("java")
        .arg("-jar")
        .arg(&backend_path)
        .env("SERVER_PORT", backend_port.to_string())
        .env("APP_MODE", "desktop-host")
        .env("MUSICPARTY_PROFILE_DIR", &profile)
        .env("BASE_URL", &backend_url)
        .env("DESKTOP_LAN_BASE_URL", &lan_url)
        .env("DESKTOP_NETEASE_API_PORT", netease_port.to_string())
        .env("NETEASE_API_URL", &netease_api_url)
        .env("YOUTUBE_ENABLED", "false")
        .env("SQUIDIFY_ENABLED", "false")
        .env("DB_PATH", profile.join("data").join("musicparty.db"))
        .env("LOCAL_LIBRARY_PATH", profile.join("local-library"))
        .env("CACHE_DIR", profile.join("cache"))
        .current_dir(&profile)
        .stdout(Stdio::from(backend_log.try_clone().map_err(|error| error.to_string())?))
        .stderr(Stdio::from(backend_log))
        .spawn()
        .map_err(|error| format!("start MusicParty backend failed: {error}"))?;
    bootstrap_log("backend process spawned");

    let status = DesktopHostStatus {
        hosting: true,
        backend_url,
        invite_url,
        profile_dir: profile.to_string_lossy().to_string(),
        backend_port,
        netease_api_url,
        netease_api_port: netease_port,
    };

    guard.backend = Some(backend);
    guard.status = Some(status.clone());

    match start_netease_process(&profile, netease_port) {
        Ok(netease) => {
            bootstrap_log("netease process spawned");
            guard.netease = Some(netease);
        }
        Err(error) => {
            bootstrap_log(&format!("netease process unavailable: {error}"));
        }
    }

    Ok(status)
}

fn start_netease_process(profile: &Path, netease_port: u16) -> Result<Child, String> {
    let netease_log = File::create(profile.join("logs").join("netease-api.log"))
        .map_err(|error| format!("create netease log failed: {error}"))?;
    Command::new(npx_command())
        .arg("-y")
        .arg("NeteaseCloudMusicApi@latest")
        .env("PORT", netease_port.to_string())
        .env("API_PORT", netease_port.to_string())
        .current_dir(profile)
        .stdout(Stdio::from(netease_log.try_clone().map_err(|error| error.to_string())?))
        .stderr(Stdio::from(netease_log))
        .spawn()
        .map_err(|error| format!("start NeteaseCloudMusicApi failed: {error}"))
}

fn npx_command() -> &'static str {
    if cfg!(windows) {
        "npx.cmd"
    } else {
        "npx"
    }
}

#[tauri::command]
fn stop_desktop_host(state: State<'_, Mutex<ProcessState>>) -> Result<(), String> {
    let mut guard = state.lock().map_err(|_| "desktop state lock poisoned".to_string())?;
    stop_child(&mut guard.backend);
    stop_child(&mut guard.netease);
    guard.status = None;
    Ok(())
}

fn stop_child(child: &mut Option<Child>) {
    if let Some(mut process) = child.take() {
        let _ = process.kill();
        let _ = process.wait();
    }
}

fn create_profile_dirs(profile: &Path) -> Result<(), String> {
    for path in [
        profile.to_path_buf(),
        profile.join("data"),
        profile.join("cache"),
        profile.join("logs"),
        profile.join("local-library"),
    ] {
        create_dir_all(&path).map_err(|error| format!("create {} failed: {error}", path.display()))?;
    }
    Ok(())
}

fn free_port() -> Result<u16, String> {
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    listener.local_addr().map(|addr| addr.port()).map_err(|error| error.to_string())
}

fn default_profile_dir(app: &AppHandle) -> PathBuf {
    app.path()
        .app_data_dir()
        .unwrap_or_else(|_| std::env::current_dir().unwrap_or_else(|_| PathBuf::from(".")))
        .join("desktop-profile")
}

fn bundled_backend_jar(app: &AppHandle) -> PathBuf {
    app.path()
        .resource_dir()
        .unwrap_or_else(|_| PathBuf::from("."))
        .join("backend")
        .join("MusicParty.jar")
}

fn normalize_base_url(value: Option<String>) -> Option<String> {
    let value = value?.trim().trim_end_matches('/').to_string();
    if value.is_empty() {
        None
    } else {
        Some(value)
    }
}

fn main() {
    tauri::Builder::default()
        .manage(Mutex::new(ProcessState::default()))
        .invoke_handler(tauri::generate_handler![
            desktop_status,
            desktop_lan_addresses,
            start_desktop_host,
            stop_desktop_host
        ])
        .setup(|app| {
            bootstrap_log("tauri setup: begin");
            let state = app.state::<Mutex<ProcessState>>();
            if let Err(error) = start_desktop_host_inner(app.handle(), &state, None, None, None) {
                bootstrap_log(&format!("tauri setup: startup failed: {error}"));
                eprintln!("MusicParty desktop host startup failed: {error}");
            } else {
                bootstrap_log("tauri setup: startup ok");
            }
            Ok(())
        })
        .on_window_event(|window, event| {
            if matches!(event, tauri::WindowEvent::CloseRequested { .. }) {
                let state = window.state::<Mutex<ProcessState>>();
                if let Ok(mut guard) = state.lock() {
                    stop_child(&mut guard.backend);
                    stop_child(&mut guard.netease);
                    guard.status = None;
                };
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running MusicParty desktop shell");
}
