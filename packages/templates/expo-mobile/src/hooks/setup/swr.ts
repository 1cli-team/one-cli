import NetInfo from "@react-native-community/netinfo";
import { AppState, AppStateStatus } from "react-native";
import { SWRConfiguration } from "swr";

// import { storage } from "@/lib";
import { isAppInactive, safeParse } from "@/lib/helper";

const storage = new Map();

export function localStorageProvider() {
  const appCache = storage.get("app-cache");
  // 性能问题
  const cache = safeParse(appCache) || [];
  const map = new Map(cache);

  const subscription = AppState.addEventListener("change", (state) => {
    if (isAppInactive(state)) {
      // TODO: 性能问题
      const appCache = JSON.stringify(Array.from(map.entries()));
      storage.set("app-cache", appCache);
    }
    subscription.remove();
  });

  return map as Map<string, any>;
}

export const swrConfig: SWRConfiguration = {
  provider: localStorageProvider,
  isOnline() {
    let isConnected: boolean | null = null;
    NetInfo.addEventListener((state) => {
      isConnected = state.isConnected;
    });
    return !!isConnected;
  },
  isVisible() {
    return AppState.currentState === "active";
  },
  initFocus(callback) {
    let appState = AppState.currentState;
    const onAppStateChange = (nextAppState: AppStateStatus) => {
      if (isAppInactive(appState) && nextAppState === "active") {
        callback();
      }
      appState = nextAppState;
    };
    const subscription = AppState.addEventListener("change", onAppStateChange);

    return () => {
      subscription.remove();
    };
  },
  initReconnect(callback) {
    const unsubscribe = NetInfo.addEventListener((state) => {
      if (state.isConnected) {
        callback();
      }
    });
    return () => {
      unsubscribe();
    };
  },
};
