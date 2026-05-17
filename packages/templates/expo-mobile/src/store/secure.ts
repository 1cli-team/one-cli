import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import * as SecureStore from "expo-secure-store";

export interface TokenState {
  token: string | null;
  setToken: (token: string | null) => void;
}

export const useTokenStore = create<TokenState>()(
  persist(
    (set) => ({
      token: null,
      setToken: (token) =>
        set(() => {
          return { token };
        }),
    }),
    {
      name: "token-storage",
      storage: createJSONStorage(() => ({
        getItem: (key) => {
          return SecureStore.getItemAsync(key);
        },
        setItem: async (key, value) => {
          await SecureStore.setItemAsync(key, value);
        },
        removeItem: async (key) => {
          await SecureStore.deleteItemAsync(key);
        },
      })),
    },
  ),
);

export const tokenSelector = (state: TokenState) => ({
  token: state.token,
  setToken: state.setToken,
});
