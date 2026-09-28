import type React from "react";
import { useMatch } from "react-router-dom";
import { TopBar } from "@/components/TopBar";
import { AppRoutes } from "@/router/routes";
import { cn } from "@/lib/utils";

export const App: React.FC = () => {
	const workspaceMode = Boolean(useMatch("/workspace/:entryId"));

	return (
		<div className="flex h-dvh min-w-0 overflow-hidden bg-background text-foreground">
			<div className="flex min-w-0 flex-1 flex-col">
				<TopBar />
				<main
					className={cn(
						"min-h-0 min-w-0 flex-1",
						workspaceMode ? "overflow-hidden" : "overflow-y-auto p-4 ud-md:p-6",
					)}
				>
					<div
						className={cn("w-full", workspaceMode ? "h-full min-h-0" : "mx-auto max-w-[1600px]")}
					>
						<AppRoutes />
					</div>
				</main>
			</div>
		</div>
	);
};
