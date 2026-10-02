import { createNativeSWRConfig } from "./swr";

test("online reads do not subscribe; reconnect listeners are cleaned up", () => {
  let notify: (state: {
    isConnected: boolean | null;
    isInternetReachable: boolean | null;
  }) => void = () => {};
  const unsubscribe = jest.fn();
  const subscribe = jest.fn((listener) => {
    notify = listener;
    return unsubscribe;
  });
  const config = createNativeSWRConfig(
    { currentState: "active", addEventListener: jest.fn() },
    { addEventListener: subscribe },
  );
  const reconnect = jest.fn();
  const cleanup = config.initReconnect!(reconnect);
  notify({ isConnected: false, isInternetReachable: false });
  expect(config.isOnline!()).toBe(false);
  expect(config.isOnline!()).toBe(false);
  expect(subscribe).toHaveBeenCalledTimes(1);
  notify({ isConnected: true, isInternetReachable: true });
  expect(config.isOnline!()).toBe(true);
  expect(reconnect).toHaveBeenCalledTimes(1);
  cleanup!();
  expect(unsubscribe).toHaveBeenCalledTimes(1);
});

test("focus revalidates only when returning to foreground and releases the listener", () => {
  let notify: (state: string) => void = () => {};
  const remove = jest.fn();
  const state = {
    currentState: "background",
    addEventListener: jest.fn((_event, listener) => {
      notify = listener;
      return { remove };
    }),
  };
  const config = createNativeSWRConfig(state, { addEventListener: jest.fn() });
  const focus = jest.fn();
  const cleanup = config.initFocus!(focus);
  notify("active");
  notify("active");
  expect(focus).toHaveBeenCalledTimes(1);
  cleanup!();
  expect(remove).toHaveBeenCalledTimes(1);
});
