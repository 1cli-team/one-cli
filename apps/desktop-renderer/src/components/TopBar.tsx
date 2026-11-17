import { Menu, MoonStar, SunMedium } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocation, useMatch } from "react-router-dom";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { EnvironmentSelector } from "@/features/environment-context/EnvironmentSelector";
import { useWorkspace } from "@/features/workspace-navigation/useWorkspace";
import { useThemeStore } from "@/lib/stores/theme";
import { workspacePath } from "@/router/paths";
export { ManifestSaveControl } from "@/features/manifest-draft/ManifestSaveControl";

export function TopBar({ homeTo, onOpenNavigation }: { homeTo: string; onOpenNavigation(): void }) {
	const { t } = useTranslation();
	const { mode, toggle } = useThemeStore();
	const { pathname } = useLocation();
	const match = useMatch("/workspaces/:entryId/*");
	const settingsMatch = useMatch("/settings/*");
	const projectMatch = useMatch("/workspaces/:entryId/projects/:projectName");
	const { workspace, overview, available } = useWorkspace(match?.params.entryId);
	const themeLabel = t(mode === "light" ? "sidebar.themeToDark" : "sidebar.themeToLight");
	const titles: Record<string, string> = {
		"/": t("sidebar.home"),
		"/settings/credentials": t("global.title"),
		"/settings": t("sidebar.settings"),
		"/settings/general": t("navigation.generalSettings"),
		"/settings/desktop": t("desktop.settings"),
	};
	return (
		<header className="flex min-h-14 shrink-0 items-center justify-between gap-2 border-b border-border bg-card px-4 py-2 ud-md:px-6">
			<div className="flex min-w-0 flex-1 items-center gap-3">
				<Button
					variant="ghost"
					size="icon"
					className="ud-md:hidden"
					aria-label={t("navigation.open")}
					onClick={onOpenNavigation}
				>
					<Menu />
				</Button>
				<Breadcrumb className="min-w-0">
					<BreadcrumbList className="flex-nowrap text-sm [&>li]:min-w-0 [&_[data-slot=breadcrumb-page]]:truncate [&_[data-slot=breadcrumb-link]]:truncate">
						{match ? (
							<>
								<BreadcrumbItem>
									<BreadcrumbLink asChild>
										<EnvironmentLink to={homeTo}>{t("sidebar.home")}</EnvironmentLink>
									</BreadcrumbLink>
								</BreadcrumbItem>
								<BreadcrumbSeparator />
								<BreadcrumbItem>
									{projectMatch ? (
										<BreadcrumbLink asChild>
											<EnvironmentLink to={workspacePath(match.params.entryId!)}>
												{workspace?.name ?? t("workspaces.workspaceLabel")}
											</EnvironmentLink>
										</BreadcrumbLink>
									) : (
										<BreadcrumbPage>
											{workspace?.name ?? t("workspaces.workspaceLabel")}
										</BreadcrumbPage>
									)}
								</BreadcrumbItem>
								{projectMatch && (
									<>
										<BreadcrumbSeparator />
										<BreadcrumbItem>
											<BreadcrumbPage>{projectMatch.params.projectName}</BreadcrumbPage>
										</BreadcrumbItem>
									</>
								)}
							</>
						) : settingsMatch && pathname !== "/settings" ? (
							<>
								<BreadcrumbItem>
									<BreadcrumbLink asChild>
										<EnvironmentLink to="/settings">{t("sidebar.settings")}</EnvironmentLink>
									</BreadcrumbLink>
								</BreadcrumbItem>
								<BreadcrumbSeparator />
								<BreadcrumbItem>
									<BreadcrumbPage>{titles[pathname] ?? t("notFound.title")}</BreadcrumbPage>
								</BreadcrumbItem>
							</>
						) : (
							<BreadcrumbItem>
								<BreadcrumbPage>{titles[pathname] ?? t("notFound.title")}</BreadcrumbPage>
							</BreadcrumbItem>
						)}
					</BreadcrumbList>
				</Breadcrumb>
			</div>
			<div className="flex shrink-0 items-center gap-1">
				{match && available && (
					<EnvironmentSelector environments={overview.data?.workspace?.environments} />
				)}
				<LanguageSwitcher />
				<Button
					variant="ghost"
					size="icon"
					onClick={toggle}
					title={themeLabel}
					aria-label={themeLabel}
				>
					{mode === "light" ? <MoonStar /> : <SunMedium />}
				</Button>
			</div>
		</header>
	);
}
