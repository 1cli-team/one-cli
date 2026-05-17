import AsyncStorage from "@react-native-async-storage/async-storage";
import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

// 需要持久化的 store
export interface GlobalConfigState {
  // 登录相关
  isLogin: boolean;
  setIsLogin: (isLogin: boolean) => void;

  // 主题
  theme: "light" | "dark";
  setTheme: (theme: "light" | "dark") => void;
}

export const useConfigStore = create<GlobalConfigState>()(
  persist(
    (set) => ({
      isLogin: false,
      setIsLogin: (isLogin) => set({ isLogin }),

      theme: "light",
      setTheme: (theme) => set({ theme }),
    }),
    {
      name: "config-storage",
      storage: createJSONStorage(() => AsyncStorage),
    },
  ),
);

export const configSelector = (state: GlobalConfigState) => ({
  isLogin: state.isLogin,
  setIsLogin: state.setIsLogin,

  theme: state.theme,
  setTheme: state.setTheme,
});
