import type React from "react";
import { useTranslation } from "react-i18next";
import { useMatch } from "react-router-dom";
import useSWR from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { EnvironmentSelector } from "@/features/environment-context/EnvironmentSelector";
import type { SectionKey } from "@/types/api";

export const TopBar: React.FC = () => {
	const sectionMatch = useMatch("/section/:domain/:backend");
	const profileMatch = useMatch("/profile");
	const settingsSectionMatch = useMatch("/settings/:domain/:backend");
	const settingsMatch = useMatch("/settings");
	const workspaceMatch = useMatch("/workspace/:entryId");
	const detailMatch = settingsSectionMatch ?? sectionMatch;
	const showEnvironmentSelector = Boolean(workspaceMatch);

	return (
		<header className="flex h-[68px] shrink-0 items-center justify-between gap-4 border-b border-border bg-background/90 px-7 backdrop-blur">
			<Breadcrumb>
				<BreadcrumbList>
					{detailMatch ? (
						<SectionCrumb
							match={detailMatch.params}
							settingsRoute={Boolean(settingsSectionMatch)}
						/>
					) : settingsMatch ? (
						<SettingsCrumb />
					) : profileMatch ? (
						<ProfileCrumb />
					) : workspaceMatch ? (
						<WorkspaceCrumb entryId={workspaceMatch.params.entryId ?? ""} />
					) : (
						<HomeCrumb />
					)}
				</BreadcrumbList>
			</Breadcrumb>
			<div className="flex items-center gap-2">
				{showEnvironmentSelector ? <EnvironmentSelector /> : null}
				<LanguageSwitcher />
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
					{workspace?.name ?? t("topbar.home")}
					{workspace?.id ? (
						<span className="ml-2 font-mono text-xs font-normal text-muted-foreground">
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
