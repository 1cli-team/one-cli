import type { DesktopAPI } from "@one-desktop/preload";
export type { Connection } from "@one-desktop/preload";
import { create } from "zustand";
interface Runtime {
	status: string;
	generation: string;
	error: string;
	connected: boolean;
}
export const useDesktop = create<Runtime>(() => ({
	status: "starting",
	generation: "",
	error: "",
	connected: false,
}));
export const native = typeof window !== "undefined" && Boolean(window.desktop);
export function desktop(): DesktopAPI {
	if (!window.desktop) throw new Error("One Desktop is required");
	return window.desktop;
}
export async function openExternal(url: string) {
	if (!native) throw new Error("One Desktop is required");
	await desktop().openExternal(url);
}
export const chooseWorkspace = () => desktop().chooseWorkspace();
