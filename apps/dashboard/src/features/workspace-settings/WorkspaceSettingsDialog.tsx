import { ManifestSaveControl } from "@/features/manifest-draft/ManifestSaveControl";
import { Braces, KeyRound, Settings } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { SecretsManager } from "@/features/secrets/SecretsManager";
import { WorkspaceEnvironmentSettings } from "@/features/workspace-settings/WorkspaceEnvironmentSettings";
import type { OverviewProject } from "@/types/api";

export const WorkspaceSettingsDialog: React.FC<{
	currentBackend?: string;
	environment: string;
	projects: OverviewProject[];
	workspaceEntryId?: string;
	readOnly?: boolean;
	triggerVariant?: "default" | "icon";
}> = ({
	currentBackend,
	environment,
	projects,
	workspaceEntryId,
	readOnly,
	triggerVariant = "default",
}) => {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const [activeTab, setActiveTab] = useState("environment");

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>
				<Button
					variant={triggerVariant === "icon" ? "ghost" : "outline"}
					size={triggerVariant === "icon" ? "icon" : "default"}
					title={t("overview.navigation.settings")}
					aria-label={t("overview.navigation.settings")}
				>
					<Settings className="size-4" />
					{triggerVariant === "default" ? t("overview.navigation.settings") : null}
				</Button>
			</DialogTrigger>
			<DialogContent className="flex max-h-[calc(100dvh-4rem)] flex-col gap-0 overflow-hidden p-0 ud-sm:max-w-[52.5rem]">
				<DialogHeader className="shrink-0 border-b border-border px-6 py-5 pr-12">
					<DialogTitle>{t("overview.navigation.settings")}</DialogTitle>
					<DialogDescription>{t("overview.workspaceEnv.description")}</DialogDescription>
				</DialogHeader>
				<Tabs
					value={activeTab}
					onValueChange={setActiveTab}
					className="flex min-h-0 flex-1 flex-col gap-0"
				>
					<div
						className="mx-6 mt-3 shrink-0 overflow-x-auto overflow-y-hidden"
						onFocusCapture={(event) => {
							event.target.scrollIntoView({ block: "nearest", inline: "nearest" });
						}}
					>
						<TabsList variant="line" className="min-w-max justify-start">
							<TabsTrigger value="environment" className="px-3">
								<Braces className="size-4" />
								{t("overview.tabs.environment")}
							</TabsTrigger>
							<TabsTrigger value="secrets" className="px-3">
								<KeyRound className="size-4" />
								{t("overview.tabs.secrets")}
							</TabsTrigger>
						</TabsList>
					</div>
					<div className="min-h-0 flex-1 overflow-y-auto p-6">
						<TabsContent value="environment" className="mt-0">
							<WorkspaceEnvironmentSettings
								key={`${workspaceEntryId ?? "current"}:${environment}:${currentBackend ?? ""}`}
								currentBackend={currentBackend}
								environment={environment}
								workspaceEntryId={workspaceEntryId}
								readOnly={readOnly}
							/>
						</TabsContent>
						<TabsContent value="secrets" className="mt-0">
							{currentBackend === "infisical" ? (
								<SecretsManager
									workspaceEntryId={workspaceEntryId}
									environment={environment}
									projects={projects}
									readOnly={readOnly}
								/>
							) : (
								<Card className="rounded-[6px] border-dashed shadow-none">
									<CardContent className="flex items-center gap-3 p-4">
										<span className="grid size-8 place-items-center rounded-[5px] bg-muted text-muted-foreground">
											<KeyRound className="size-4" />
										</span>
										<div>
											<h2 className="text-sm font-semibold">{t("secrets.unavailableTitle")}</h2>
											<p className="mt-0.5 text-xs text-muted-foreground">
												{t("secrets.unavailableDescription")}
											</p>
										</div>
									</CardContent>
								</Card>
							)}
						</TabsContent>
					</div>
				</Tabs>
				<DialogFooter className="shrink-0 border-t border-border bg-muted/20 px-6 py-4">
					<Button variant="outline" onClick={() => setOpen(false)}>
						{t("form.close")}
					</Button>
					{workspaceEntryId && !readOnly && <ManifestSaveControl entryId={workspaceEntryId} />}
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
};
