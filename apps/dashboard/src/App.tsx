import type React from "react";
import { useMatch } from "react-router-dom";
import { TopBar } from "@/components/TopBar";
import { AppRoutes } from "@/router/routes";
import { cn } from "@/lib/utils";

export const App: React.FC = () => {
	const workspaceMode = Boolean(useMatch("/workspace/:entryId"));

	return (
		<div className="flex h-dvh min-w-[960px] flex-col overflow-hidden bg-background text-foreground">
			{workspaceMode ? null : <TopBar />}
			<main
				className={cn(
					"min-h-0 flex-1",
					workspaceMode ? "overflow-hidden" : "overflow-y-auto px-6 py-6",
				)}
			>
				<div className={cn("w-full", workspaceMode ? "h-full min-h-0" : "mx-auto max-w-[1480px]")}>
					<AppRoutes />
				</div>
			</main>
		</div>
	);
};
