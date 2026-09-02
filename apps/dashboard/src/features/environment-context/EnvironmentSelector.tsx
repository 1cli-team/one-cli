import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import {
	DASHBOARD_ENVIRONMENTS,
	environmentFromSearch,
	type DashboardEnvironment,
} from "@/features/environment-context/environment";

/**
 * EnvironmentSelector is a tab-local Dashboard view context. It only updates
 * the URL and deliberately does not call a write API or change the Workspace
 * manifest's default environment.
 */
export function EnvironmentSelector() {
	const { t } = useTranslation();
	const [searchParams, setSearchParams] = useSearchParams();
	const [pendingEnvironment, setPendingEnvironment] = useState<DashboardEnvironment | null>(null);
	const dirty = useEnvironmentDirtyStore((state) => state.dirty);
	const discardAll = useEnvironmentDirtyStore((state) => state.discardAll);
	const selected = environmentFromSearch(searchParams.toString());

	function selectEnvironment(environment: DashboardEnvironment) {
		const next = new URLSearchParams(searchParams);
		next.set("env", environment);
		setSearchParams(next, { replace: true });
	}

	function requestEnvironment(environment: DashboardEnvironment) {
		if (environment === selected) return;
		if (dirty) {
			setPendingEnvironment(environment);
			return;
		}
		selectEnvironment(environment);
	}

	return (
		<>
			<div className="flex h-8 items-center rounded-md border border-border bg-card/80 p-0.5 shadow-sm">
				<span className="px-2 font-mono text-[11px] font-semibold tracking-[0.08em] text-muted-foreground uppercase">
					{t("environmentSwitcher.label", { defaultValue: "Environment" })}
				</span>
				<ToggleGroup
					type="single"
					value={selected}
					spacing={1}
					onValueChange={(value) => {
						if (value) requestEnvironment(value as DashboardEnvironment);
					}}
					aria-label={t("environmentSwitcher.label", { defaultValue: "Environment" })}
				>
					{DASHBOARD_ENVIRONMENTS.map((environment) => (
						<ToggleGroupItem
							key={environment}
							value={environment}
							aria-label={t(`environmentSwitcher.${environment}`, {
								defaultValue: environment,
							})}
							className="h-6 min-w-0 rounded px-2.5 text-xs font-medium text-muted-foreground shadow-none data-[state=on]:bg-primary data-[state=on]:text-primary-foreground data-[state=on]:shadow-sm"
						>
							{t(`environmentSwitcher.${environment}`, {
								defaultValue: environment,
							})}
						</ToggleGroupItem>
					))}
				</ToggleGroup>
			</div>

			<AlertDialog
				open={pendingEnvironment !== null}
				onOpenChange={(open) => {
					if (!open) setPendingEnvironment(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t("projectInspector.unsavedChangesTitle")}</AlertDialogTitle>
						<AlertDialogDescription>{t("projectInspector.unsavedChanges")}</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>{t("form.cancel")}</AlertDialogCancel>
						<AlertDialogAction
							onClick={() => {
								const next = pendingEnvironment;
								setPendingEnvironment(null);
								discardAll();
								if (next) selectEnvironment(next);
							}}
						>
							{t("projectInspector.discardAndContinue")}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
}
