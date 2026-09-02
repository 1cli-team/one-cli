import { Layers3 } from "lucide-react";
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
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
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
export function EnvironmentSelector({
	variant = "default",
}: {
	variant?: "default" | "icon";
}) {
	const { t } = useTranslation();
	const [searchParams, setSearchParams] = useSearchParams();
	const [pendingEnvironment, setPendingEnvironment] = useState<DashboardEnvironment | null>(null);
	const dirty = useEnvironmentDirtyStore((state) => state.dirty);
	const discardAll = useEnvironmentDirtyStore((state) => state.discardAll);
	const selected = environmentFromSearch(searchParams.toString());
	const label = t("environmentSwitcher.label", { defaultValue: "Environment" });
	const selectedLabel = t(`environmentSwitcher.${selected}`, { defaultValue: selected });

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
			<Select
				value={selected}
				onValueChange={(value) => requestEnvironment(value as DashboardEnvironment)}
			>
				<SelectTrigger
					size="sm"
					showChevron={variant !== "icon"}
					className={
						variant === "icon"
							? "h-9 w-9 shrink-0 justify-center border-0 bg-transparent p-0 shadow-none hover:bg-accent"
							: "w-32 shrink-0 font-mono text-xs font-semibold"
					}
					title={variant === "icon" ? `${label}: ${selectedLabel}` : undefined}
					aria-label={`${label}: ${selectedLabel}`}
				>
					{variant === "icon" ? <Layers3 className="size-4" /> : <SelectValue />}
				</SelectTrigger>
				<SelectContent>
					{DASHBOARD_ENVIRONMENTS.map((environment) => (
						<SelectItem key={environment} value={environment} className="font-mono text-xs">
							{t(`environmentSwitcher.${environment}`, {
								defaultValue: environment,
							})}
						</SelectItem>
					))}
				</SelectContent>
			</Select>

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
