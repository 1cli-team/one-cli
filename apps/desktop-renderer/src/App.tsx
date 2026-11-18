import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Outlet, useLocation, useMatch, useNavigate } from "react-router-dom";
import { AppSidebar } from "@/components/AppSidebar";
import { TopBar } from "@/components/TopBar";
import { DesktopWelcome } from "@/lib/desktop/DesktopSettings";
import { desktop, native } from "@/lib/desktop/runtime";
import { NavigationGuard } from "@/router/NavigationGuard";
import { groupPath } from "@/router/paths";
import { cn } from "@/lib/utils";

export function App() {
	const { pathname, search } = useLocation();
	const navigate = useNavigate();
	const entryId = useMatch("/workspaces/:entryId/*")?.params.entryId;
	const workspaceMode = Boolean(entryId);
	const detailMode = workspaceMode;
	const credentialsMode = Boolean(useMatch("/settings/credentials"));
	const [mobileOpen, setMobileOpen] = useState(false);
	const homeTo = useRef("/");
	const main = useRef<HTMLElement>(null);
	const scrollPositions = useRef(new Map<string, number>());
	if (pathname === "/" || pathname === "/workspaces") homeTo.current = `${pathname}${search}`;
	const groupId = new URLSearchParams(search).get("group");
	const workspaceHome = entryId && groupId ? groupPath(groupId) : homeTo.current;
	useEffect(() => {
		if (!native) return;
		return desktop().onGatewayExecution?.((id) => {
			void navigate(`/gateway?execution=${encodeURIComponent(id)}`);
		});
	}, [navigate]);
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
				homeTo={workspaceHome}
				mobileOpen={mobileOpen}
				onMobileOpenChange={setMobileOpen}
			/>
			<div className="flex min-w-0 flex-1 flex-col">
				<TopBar homeTo={workspaceHome} onOpenNavigation={() => setMobileOpen(true)} />
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
