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
		"relative flex h-9 items-center gap-2.5 rounded-md px-3 text-sm transition-colors",
		isActive
			? "bg-primary/9 font-medium text-primary before:absolute before:-left-2 before:h-5 before:w-0.5 before:rounded-full before:bg-primary"
			: "text-muted-foreground hover:bg-accent/70 hover:text-foreground",
	);

export const AppSidebar: React.FC = () => {
	const { mode, toggle } = useThemeStore();
	const { t } = useTranslation();
	const logoSrc = mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg";

	return (
		<TooltipProvider delayDuration={300}>
			<aside className="flex h-screen w-56 shrink-0 flex-col border-r border-border bg-card/75">
				<div className="flex h-[68px] items-center gap-2.5 border-b border-border/70 px-5">
					<img src={logoSrc} alt="One CLI" className="h-8 w-8" />
					<div>
						<p className="text-sm font-semibold tracking-tight">One CLI</p>
						<p className="text-[10px] font-medium tracking-[0.18em] text-muted-foreground uppercase">
							{t("sidebar.brand")}
						</p>
					</div>
				</div>

				<nav className="px-3 py-4">
					<EnvironmentNavLink to="/" end className={navItemClass}>
						<House className="h-4 w-4" />
						<span>{t("sidebar.home")}</span>
					</EnvironmentNavLink>
				</nav>

				<WorkspaceRail />

				<div className="flex items-center gap-1 border-t border-border px-3 py-3">
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
								className="h-9 w-9 shrink-0"
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
