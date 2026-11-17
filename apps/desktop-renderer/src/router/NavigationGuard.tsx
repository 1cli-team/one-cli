import { useCallback } from "react";
import { useBeforeUnload, useBlocker } from "react-router-dom";
import { DiscardDialog } from "@/components/ui/discard-dialog";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";

export function NavigationGuard() {
	const dirty = useEnvironmentDirtyStore((state) => state.dirty);
	const discardAll = useEnvironmentDirtyStore((state) => state.discardAll);
	const blocker = useBlocker(
		({ currentLocation, nextLocation }) =>
			dirty &&
			(currentLocation.pathname !== nextLocation.pathname ||
				currentLocation.search !== nextLocation.search),
	);
	useBeforeUnload(
		useCallback(
			(event) => {
				if (dirty) event.preventDefault();
			},
			[dirty],
		),
	);
	return (
		<DiscardDialog
			open={blocker.state === "blocked"}
			onOpenChange={(open) => {
				if (!open && blocker.state === "blocked") blocker.reset();
			}}
			onDiscard={() => {
				if (blocker.state !== "blocked") return;
				discardAll();
				blocker.proceed();
			}}
		/>
	);
}
