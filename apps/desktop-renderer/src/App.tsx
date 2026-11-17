import { useLayoutEffect, useRef, useState } from "react";
import { Outlet, useLocation, useMatch } from "react-router-dom";
import { AppSidebar } from "@/components/AppSidebar";
import { TopBar } from "@/components/TopBar";
import { DesktopWelcome } from "@/lib/desktop/DesktopSettings";
import { NavigationGuard } from "@/router/NavigationGuard";
import { cn } from "@/lib/utils";

export function App() {
	const { pathname, search } = useLocation();
	const entryId = useMatch("/workspaces/:entryId/*")?.params.entryId;
	const workspaceMode = Boolean(entryId);
	const detailMode = workspaceMode;
	const credentialsMode = Boolean(useMatch("/settings/credentials"));
	const [mobileOpen, setMobileOpen] = useState(false);
	const homeTo = useRef("/");
	const main = useRef<HTMLElement>(null);
	const scrollPositions = useRef(new Map<string, number>());
	if (pathname === "/") homeTo.current = `/${search}`;
	useLayoutEffect(() => {
		setMobileOpen(false);
		const element = main.current;
		if (!element) return;
		element.scrollTop = scrollPositions.current.get(pathname) ?? 0;
	}, [pathname]);
	return (
		<div className="flex h-dvh min-w-0 overflow-hidden bg-background text-foreground">
			<DesktopWelcome />
			<NavigationGuard />
			<AppSidebar
				entryId={entryId}
				homeTo={homeTo.current}
				mobileOpen={mobileOpen}
				onMobileOpenChange={setMobileOpen}
			/>
			<div className="flex min-w-0 flex-1 flex-col">
				<TopBar homeTo={homeTo.current} onOpenNavigation={() => setMobileOpen(true)} />
				<main
					ref={main}
					onScroll={(event) => scrollPositions.current.set(pathname, event.currentTarget.scrollTop)}
					className={cn(
						"min-h-0 min-w-0 flex-1",
						detailMode ? "overflow-hidden" : "overflow-y-auto p-4 ud-md:p-6",
					)}
				>
					<div
						className={cn(
							"w-full",
							(detailMode || credentialsMode) && "h-full min-h-0",
							!detailMode && "mx-auto max-w-[1600px]",
						)}
					>
						<Outlet />
					</div>
				</main>
			</div>
		</div>
	);
}
