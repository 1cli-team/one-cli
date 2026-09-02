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
					size={triggerVariant === "icon" ? "icon-sm" : "sm"}
					title={t("overview.navigation.settings")}
					aria-label={t("overview.navigation.settings")}
				>
					<Settings className="size-4" />
					{triggerVariant === "default" ? t("overview.navigation.settings") : null}
				</Button>
			</DialogTrigger>
			<DialogContent className="flex h-[min(780px,calc(100dvh-2rem))] max-w-[min(1120px,calc(100vw-2rem))] flex-col gap-0 overflow-hidden p-0 sm:max-w-[min(1120px,calc(100vw-2rem))]">
				<DialogHeader className="shrink-0 border-b border-border px-4 py-3 pr-12">
					<DialogTitle>{t("overview.navigation.settings")}</DialogTitle>
					<DialogDescription>{t("overview.workspaceEnv.description")}</DialogDescription>
				</DialogHeader>
				<Tabs
					value={activeTab}
					onValueChange={setActiveTab}
					className="flex min-h-0 flex-1 flex-col gap-0"
				>
					<TabsList className="mx-4 mt-3 h-9 w-fit shrink-0 rounded-[5px] border border-border bg-muted/40 p-0.5">
						<TabsTrigger value="environment" className="h-8 rounded-[4px] px-3 text-xs">
							<Braces className="size-3.5" />
							{t("overview.tabs.environment")}
						</TabsTrigger>
						<TabsTrigger value="secrets" className="h-8 rounded-[4px] px-3 text-xs">
							<KeyRound className="size-3.5" />
							{t("overview.tabs.secrets")}
						</TabsTrigger>
					</TabsList>
					<div className="min-h-0 flex-1 overflow-y-auto p-4">
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
			</DialogContent>
		</Dialog>
	);
};
