import { create } from "zustand";

// 不需要持久化的 store
export interface GlobalSessionState {
  // 登录弹窗
  loginModalOpen: boolean;
  setLoginModalOpen: (open: boolean) => void;
}

export const useSessionStore = create<GlobalSessionState>((set) => ({
  loginModalOpen: false,
  setLoginModalOpen: (open) => set({ loginModalOpen: open }),
}));

export const sessionSelector = (state: GlobalSessionState) => ({
  loginModalOpen: state.loginModalOpen,
  setLoginModalOpen: state.setLoginModalOpen,
});
