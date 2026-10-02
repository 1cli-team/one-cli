import type { SWRConfiguration } from "swr";

type AppStateSource = {
  currentState: string | null;
  addEventListener: (event: "change", listener: (state: string) => void) => { remove: () => void };
};
type NetworkState = { isConnected: boolean | null; isInternetReachable: boolean | null };
type NetworkSource = { addEventListener: (listener: (state: NetworkState) => void) => () => void };

export function createNativeSWRConfig(
  appState: AppStateSource,
  network: NetworkSource,
): SWRConfiguration {
  let online = true;
  return {
    // SWR owns this cache and initializes/cleans up its native listeners.
    provider: () => new Map(),
    isOnline: () => online,
    isVisible: () => appState.currentState === "active",
    initFocus(callback) {
      let previous = appState.currentState;
      const subscription = appState.addEventListener("change", (next) => {
        if (previous !== "active" && next === "active") callback();
        previous = next;
      });
      return () => subscription.remove();
    },
    initReconnect(callback) {
      return network.addEventListener((state) => {
        const next = state.isConnected !== false && state.isInternetReachable !== false;
        if (!online && next) callback();
        online = next;
      });
    },
  };
}
