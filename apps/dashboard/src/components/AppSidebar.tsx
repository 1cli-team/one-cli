import { KeyRound, House, Menu, MoonStar, Settings2, SunMedium } from "lucide-react";
import type React from "react";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { Button } from "@/components/ui/button";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetTitle,
	SheetTrigger,
} from "@/components/ui/sheet";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import {
	EnvironmentLink,
	EnvironmentNavLink,
} from "@/features/environment-context/EnvironmentLink";
import { SessionStatus } from "@/features/infisical-session/AccountSettings";
import { WorkspaceRail } from "@/features/workspace-registry/WorkspaceRail";
import { useThemeStore } from "@/lib/stores/theme";
import { cn } from "@/lib/utils";

const navItemClass = ({ isActive }: { isActive: boolean }) =>
	cn(
		"relative flex min-h-10 items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors duration-150",
		isActive
			? "bg-sidebar-active font-medium text-primary-text"
			: "text-sidebar-muted hover:bg-muted hover:text-sidebar-foreground",
	);

function SidebarContent({ onNavigate }: { onNavigate?: () => void }) {
	const { mode, toggle } = useThemeStore();
	const { t } = useTranslation();
	return (
		<TooltipProvider delayDuration={300}>
			<div
				className="flex h-full min-h-0 flex-col"
				onClick={(event) => {
					if ((event.target as HTMLElement).closest("a")) onNavigate?.();
				}}
			>
				<EnvironmentLink
					to="/"
					className="flex h-14 shrink-0 items-center gap-3 border-b border-sidebar-border px-5"
				>
					<img
						src={mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg"}
						alt=""
						className="size-7"
					/>
					<span className="text-base font-semibold tracking-tight">One CLI</span>
					<span className="text-xs text-sidebar-muted">Dashboard</span>
				</EnvironmentLink>
				<nav aria-label={t("sidebar.navigation")} className="space-y-1 px-3 py-4">
					<EnvironmentNavLink to="/" end className={navItemClass}>
						<House className="size-4" />
						<span>{t("sidebar.home")}</span>
					</EnvironmentNavLink>
					<EnvironmentNavLink to="/global" className={navItemClass}>
						<KeyRound className="size-4" />
						<span>{t("global.title")}</span>
					</EnvironmentNavLink>
				</nav>
				<WorkspaceRail />
				<EnvironmentLink
					to="/settings"
					className="mx-3 mb-3 rounded-md border border-sidebar-border bg-card p-3"
				>
					<SessionStatus />
				</EnvironmentLink>
				<div className="flex min-h-14 shrink-0 items-center gap-1 border-t border-sidebar-border px-3">
					<EnvironmentNavLink
						to="/settings"
						className={({ isActive }) => cn(navItemClass({ isActive }), "min-w-0 flex-1")}
					>
						<Settings2 className="size-4" />
						<span>{t("sidebar.settings")}</span>
					</EnvironmentNavLink>
					<LanguageSwitcher />
					<Tooltip>
						<TooltipTrigger asChild>
							<Button
								onClick={toggle}
								variant="ghost"
								size="icon"
								aria-label={mode === "light" ? t("sidebar.themeToDark") : t("sidebar.themeToLight")}
							>
								{mode === "light" ? <MoonStar /> : <SunMedium />}
							</Button>
						</TooltipTrigger>
						<TooltipContent side="top">
							{mode === "light" ? t("sidebar.themeToDark") : t("sidebar.themeToLight")}
						</TooltipContent>
					</Tooltip>
				</div>
			</div>
		</TooltipProvider>
	);
}

export const AppSidebar: React.FC = () => (
	<aside className="hidden h-dvh w-60 shrink-0 border-r border-sidebar-border bg-sidebar text-sidebar-foreground ud-md:block">
		<SidebarContent />
	</aside>
);

export function MobileNavigation() {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const navigationRef = useRef<HTMLDivElement>(null);
	return (
		<Sheet open={open} onOpenChange={setOpen}>
			<SheetTrigger asChild>
				<Button
					variant="ghost"
					size="icon-lg"
					className="ud-md:hidden"
					aria-label={t("sidebar.openNavigation")}
				>
					<Menu />
				</Button>
			</SheetTrigger>
			<SheetContent
				ref={navigationRef}
				onOpenAutoFocus={(event) => {
					event.preventDefault();
					navigationRef.current?.querySelector<HTMLAnchorElement>("a")?.focus();
				}}
				side="left"
				closeLabel={t("sidebar.closeNavigation")}
				className="w-80 gap-0 rounded-none bg-sidebar [&>button]:top-3 [&>button]:right-2"
			>
				<SheetTitle className="sr-only">{t("sidebar.navigation")}</SheetTitle>
				<SheetDescription className="sr-only">{t("workspaces.home.description")}</SheetDescription>
				<SidebarContent onNavigate={() => setOpen(false)} />
			</SheetContent>
		</Sheet>
	);
}
