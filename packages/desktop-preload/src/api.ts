export interface Connection {
  sequence: number;
  status: string;
  generation: string;
  port: number;
  token?: string;
  error: string;
}

export interface UpdateInfo {
  enabled: boolean;
  version?: string;
  notes?: string;
  reason?: string;
}

export interface DesktopAPI {
  getLegacyPreferences(): Promise<
    Partial<Record<"app_locale_mode" | "app_theme_mode" | "one_desktop_setup_seen", string>>
  >;
  getBackendConnection(): Promise<Connection>;
  onBackendState(callback: () => void): () => void;
  retryBackend(): Promise<void>;
  confirmHealth(generation: string): Promise<void>;
  chooseWorkspace(): Promise<string | null>;
  openExternal(url: string): Promise<void>;
  setNativeLocale(locale: string): Promise<void>;
  checkUpdate(): Promise<UpdateInfo>;
  installUpdate(): Promise<void>;
}

declare global {
  interface Window {
    desktop?: DesktopAPI;
  }
}
