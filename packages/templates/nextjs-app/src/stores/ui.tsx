"use client";

import { createContext, useContext, useState, type PropsWithChildren } from "react";
import { createStore, useStore } from "zustand";

type UIState = { locale: "en" | "zh"; toggleLocale: () => void };
function createUIStore() {
	return createStore<UIState>()((set) => ({
		locale: "en",
		toggleLocale: () => set((state) => ({ locale: state.locale === "en" ? "zh" : "en" })),
	}));
}
const UIContext = createContext<ReturnType<typeof createUIStore> | null>(null);
export function UIProvider({ children }: PropsWithChildren) {
	const [store] = useState(createUIStore);
	return <UIContext.Provider value={store}>{children}</UIContext.Provider>;
}
export function useUI<T>(selector: (state: UIState) => T) {
	const store = useContext(UIContext);
	if (!store) throw new Error("UIProvider is required");
	return useStore(store, selector);
}
