import { createStore } from "@/lib/utils";

type DiscardCallback = () => void;

interface EnvironmentDirtyState {
	dirtyOwners: Readonly<Record<string, true>>;
	dirty: boolean;
	setDirty(owner: string, dirty: boolean, onDiscard?: DiscardCallback): void;
	clearOwner(owner: string): void;
	discardAll(): void;
	reset(): void;
}

const discardCallbacks = new Map<string, DiscardCallback>();

/**
 * Bridges the project Profile editor and the environment control in the app
 * chrome. The selector lives outside the workspace route, so local component
 * state cannot protect an in-flight edit when the route's SWR key changes.
 */
export const useEnvironmentDirtyStore = createStore<EnvironmentDirtyState>(
	(set, get) => ({
		dirtyOwners: {},
		dirty: false,
		setDirty: (owner, dirty, onDiscard) => {
			if (dirty && onDiscard) discardCallbacks.set(owner, onDiscard);
			else discardCallbacks.delete(owner);
			set((state) => {
				const dirtyOwners = { ...state.dirtyOwners };
				if (dirty) dirtyOwners[owner] = true;
				else delete dirtyOwners[owner];
				return { dirtyOwners, dirty: Object.keys(dirtyOwners).length > 0 };
			});
		},
		clearOwner: (owner) => get().setDirty(owner, false),
		discardAll: () => {
			const callbacks = [...discardCallbacks.values()];
			discardCallbacks.clear();
			set({ dirtyOwners: {}, dirty: false });
			for (const discard of callbacks) discard();
		},
		reset: () => {
			discardCallbacks.clear();
			set({ dirtyOwners: {}, dirty: false });
		},
	}),
	"environmentDirtyStore",
);
