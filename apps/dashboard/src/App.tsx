import type React from "react";
import { AppSidebar } from "@/components/AppSidebar";
import { TopBar } from "@/components/TopBar";
import { AppRoutes } from "@/router/routes";

export const App: React.FC = () => {
	return (
		// h-screen + overflow-hidden pins the chrome (sidebar + topbar) to
		// the viewport so only <main> scrolls when a page is taller than the
		// viewport (e.g. /profile with many profile cards). Without this,
		// the whole document scrolls and the sidebar disappears off-screen.
		<div className="flex h-screen overflow-hidden bg-background text-foreground">
			<AppSidebar />
			<div className="flex min-w-0 flex-1 flex-col">
				<TopBar />
				<main className="flex-1 overflow-y-auto px-6 py-6">
					<div className="mx-auto w-full max-w-6xl">
						<AppRoutes />
					</div>
				</main>
			</div>
		</div>
	);
};
