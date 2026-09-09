import { create } from "zustand";
import { immer } from "zustand/middleware/immer";

// 全局状态 store：zustand + immer
// immer 让嵌套字段可以直接赋值（不用展开）
interface AppState {
  count: number;
  theme: "light" | "dark";
}

interface AppActions {
  increase: (step?: number) => void;
  reset: () => void;
  toggleTheme: () => void;
}

export const useAppStore = create<AppState & AppActions>()(
  immer((set) => ({
    count: 0,
    theme: "light",
    increase: (step = 1) =>
      set((state) => {
        state.count += step;
      }),
    reset: () =>
      set((state) => {
        state.count = 0;
      }),
    toggleTheme: () =>
      set((state) => {
        state.theme = state.theme === "light" ? "dark" : "light";
      }),
  })),
);
