import { House, MoonStar, Settings2, SunMedium } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { EnvironmentNavLink } from "@/features/environment-context/EnvironmentLink";
import { WorkspaceRail } from "@/features/workspace-registry/WorkspaceRail";
import { useThemeStore } from "@/lib/stores/theme";
import { cn } from "@/lib/utils";

const navItemClass = ({ isActive }: { isActive: boolean }) =>
	cn(
		"relative flex h-10 items-center gap-2.5 px-3 text-xs transition-colors",
		isActive
			? "bg-sidebar-active font-semibold text-sidebar-foreground before:absolute before:inset-y-0 before:left-0 before:w-0.5 before:bg-primary"
			: "text-sidebar-muted hover:bg-sidebar-active/60 hover:text-sidebar-foreground",
	);

export const AppSidebar: React.FC = () => {
	const { mode, toggle } = useThemeStore();
	const { t } = useTranslation();
	const logoSrc = mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg";

	return (
		<TooltipProvider delayDuration={300}>
			<aside className="flex h-screen w-48 shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
				<div className="flex h-12 items-center gap-2 border-b border-sidebar-border px-3">
					<img src={logoSrc} alt="One CLI" className="h-7 w-7" />
					<div>
						<p className="font-heading text-sm font-semibold tracking-tight">One CLI</p>
						<p className="font-mono text-[8px] font-semibold tracking-[0.2em] text-sidebar-muted uppercase">
							{t("sidebar.brand")}
						</p>
					</div>
				</div>

				<nav className="px-2 py-2">
					<EnvironmentNavLink to="/" end className={navItemClass}>
						<House className="h-4 w-4" />
						<span>{t("sidebar.home")}</span>
					</EnvironmentNavLink>
				</nav>

				<WorkspaceRail />

				<div className="flex h-12 items-center gap-1 border-t border-sidebar-border px-2">
					<EnvironmentNavLink
						to="/settings"
						className={({ isActive }) => `${navItemClass({ isActive })} flex-1`}
					>
						<Settings2 className="h-4 w-4" />
						<span>{t("sidebar.settings")}</span>
					</EnvironmentNavLink>
					<Tooltip>
						<TooltipTrigger asChild>
							<Button
								onClick={toggle}
								variant="ghost"
								size="icon"
								className="h-9 w-9 shrink-0 text-sidebar-muted hover:bg-sidebar-active hover:text-sidebar-foreground"
								aria-label={mode === "light" ? t("sidebar.themeToDark") : t("sidebar.themeToLight")}
							>
								{mode === "light" ? (
									<MoonStar className="h-4 w-4" />
								) : (
									<SunMedium className="h-4 w-4" />
								)}
							</Button>
						</TooltipTrigger>
						<TooltipContent side="top">
							{mode === "light" ? t("sidebar.themeToDark") : t("sidebar.themeToLight")}
						</TooltipContent>
					</Tooltip>
				</div>
			</aside>
		</TooltipProvider>
	);
};
