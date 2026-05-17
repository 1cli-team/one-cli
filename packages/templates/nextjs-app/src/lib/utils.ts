import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";
import type { StateCreator } from "zustand";
import { create } from "zustand";
import { devtools } from "zustand/middleware";

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export const createStore = <T>(storeCreator: StateCreator<T>, name: string) => {
	if (process.env.NODE_ENV === "development") {
		return create(devtools(storeCreator, { name }));
	}
	return create(storeCreator);
};
