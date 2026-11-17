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

/** Tracks unsaved editors across route navigation, environment changes and dialogs. */
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
