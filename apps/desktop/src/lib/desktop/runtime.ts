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
	localConnection: boolean;
	mode: "local" | "remote";
	serverURL: string;
	capabilities: Record<string, boolean>;
}
export const useDesktop = create<Runtime>(() => ({
	status: "starting",
	generation: "",
	error: "",
	connected: false,
	localConnection: false,
	mode: "local",
	serverURL: "",
	capabilities: {},
}));
export const native = isTauri();
export async function openExternal(url: string) {
	if (!native) {
		window.open(url, "_blank", "noopener,noreferrer");
		return;
	}
	await invoke("open_external", { url });
}
export const chooseWorkspace = () => invoke<string | null>("choose_workspace");
