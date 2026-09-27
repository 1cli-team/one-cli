import type React from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useMatch } from "react-router-dom";
import useSWR from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { MobileNavigation } from "@/components/AppSidebar";
import type { SectionKey } from "@/types/api";
export { ManifestSaveControl } from "@/features/manifest-draft/ManifestSaveControl";
interface TopBarProps {
	devDataMode?: string;
}

export const TopBar: React.FC<TopBarProps> = () => {
	const { t } = useTranslation();
	const sectionMatch = useMatch("/section/:domain/:backend");
	const profileMatch = useMatch("/profile");
	const settingsSectionMatch = useMatch("/settings/:domain/:backend");
	const settingsMatch = useMatch("/settings");
	const globalMatch = useMatch("/global");
	const workspaceMatch = useMatch("/workspace/:entryId");

	const { pathname } = useLocation();
	const detailMatch = settingsSectionMatch ?? sectionMatch;

	return (
		<header className="flex min-h-14 shrink-0 flex-wrap items-center justify-between gap-2 border-b border-border bg-card px-4 py-2 ud-md:px-6">
			<div className="flex min-w-0 flex-1 items-center gap-3">
				<MobileNavigation />
				<div className="min-w-0">
					<Breadcrumb>
						<BreadcrumbList className="flex-nowrap text-sm [&>li]:min-w-0 [&_[data-slot=breadcrumb-page]]:truncate">
							{detailMatch ? (
								<SectionCrumb
									match={detailMatch.params}
									settingsRoute={Boolean(settingsSectionMatch)}
								/>
							) : globalMatch ? (
								<BreadcrumbItem>
									<BreadcrumbPage>{t("global.title")}</BreadcrumbPage>
								</BreadcrumbItem>
							) : settingsMatch ? (
								<SettingsCrumb />
							) : profileMatch ? (
								<ProfileCrumb />
							) : workspaceMatch ? (
								<WorkspaceCrumb entryId={workspaceMatch.params.entryId ?? ""} />
							) : pathname === "/" ? (
								<HomeCrumb />
							) : (
								<BreadcrumbItem>
									<BreadcrumbPage>{t("notFound.title")}</BreadcrumbPage>
								</BreadcrumbItem>
							)}
						</BreadcrumbList>
					</Breadcrumb>
				</div>
			</div>
		</header>
	);
};

const HomeCrumb: React.FC = () => {
	const { t } = useTranslation();
	return (
		<BreadcrumbItem>
			<BreadcrumbPage>{t("topbar.workspaces")}</BreadcrumbPage>
		</BreadcrumbItem>
	);
};

const ProfileCrumb: React.FC = () => {
	const { t } = useTranslation();
	return (
		<BreadcrumbItem>
			<BreadcrumbPage>{t("topbar.profile")}</BreadcrumbPage>
		</BreadcrumbItem>
	);
};

const SettingsCrumb: React.FC = () => {
	const { t } = useTranslation();
	return (
		<BreadcrumbItem>
			<BreadcrumbPage>{t("topbar.settings", { defaultValue: "Settings" })}</BreadcrumbPage>
		</BreadcrumbItem>
	);
};

const WorkspaceCrumb: React.FC<{ entryId: string }> = ({ entryId }) => {
	const { t } = useTranslation();
	const registry = useSWR(workspacesKey, getWorkspaces);
	const workspace = registry.data?.workspaces.find((entry) => entry.entryId === entryId);
	return (
		<>
			<BreadcrumbItem>
				<BreadcrumbLink asChild>
					<EnvironmentLink to="/">{t("topbar.workspaces")}</EnvironmentLink>
				</BreadcrumbLink>
			</BreadcrumbItem>
			<BreadcrumbSeparator />
			<BreadcrumbItem>
				<BreadcrumbPage>
					{workspace?.name ?? t("workspaces.unknown.title")}
					{workspace?.id ? (
						<span className="ml-2 hidden font-mono text-xs font-normal text-muted-foreground ud-lg:inline">
							{workspace.id}
						</span>
					) : null}
				</BreadcrumbPage>
			</BreadcrumbItem>
		</>
	);
};

const SectionCrumb: React.FC<{
	match: { domain?: string; backend?: string };
	settingsRoute?: boolean;
}> = ({ match, settingsRoute = false }) => {
	const { t } = useTranslation();
	const catalog = useBackendCatalog();
	const key = `${match.domain ?? ""}/${match.backend ?? ""}` as SectionKey;
	const backend = catalog.byID.get(key);
	const title = backend
		? t(`sections.${backend.domain}.${backend.name}.title`, {
				defaultValue: humanizeBackendName(backend.name),
			})
		: key;
	return (
		<>
			<BreadcrumbItem>
				<BreadcrumbLink asChild>
					<EnvironmentLink to={settingsRoute ? "/settings" : "/profile"}>
						{settingsRoute
							? t("topbar.settings", { defaultValue: "Settings" })
							: t("topbar.sectionsRoot")}
					</EnvironmentLink>
				</BreadcrumbLink>
			</BreadcrumbItem>
			<BreadcrumbSeparator />
			<BreadcrumbItem>
				<BreadcrumbPage>
					{title}
					{backend ? (
						<span className="ml-2 text-xs font-normal text-muted-foreground">{backend.id}</span>
					) : null}
				</BreadcrumbPage>
			</BreadcrumbItem>
		</>
	);
};
