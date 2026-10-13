import { invoke, isTauri } from "@tauri-apps/api/core";
import { create } from "zustand";
export interface Connection {
	sequence: number;
	status: string;
	generation: string;
	port: number;
	token?: string;
	error: string;
}
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
export const native = isTauri();
export async function openExternal(url: string) {
	if (!native) throw new Error("One Desktop is required");
	await invoke("open_external", { url });
}
export const chooseWorkspace = () => invoke<string | null>("choose_workspace");
