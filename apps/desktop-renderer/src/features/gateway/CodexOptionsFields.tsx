import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Choice } from "@/features/linear/Choice";
import { useCodexModels, type CodexModelCatalog, type CodexOptions } from "./api";
import type { ExecutionWorker } from "./executions";

export function CodexOptionsFields({
	worker,
	options,
	onChange,
	disabled,
}: {
	worker?: ExecutionWorker;
	options: CodexOptions;
	onChange(options: CodexOptions): void;
	disabled?: boolean;
}) {
	const { t } = useTranslation();
	const useLocalCatalog = !worker || worker.id === "local";
	const local = useCodexModels(useLocalCatalog);
	const catalog = useLocalCatalog ? local.data : worker?.models;
	return (
		<div className="grid gap-3">
			<CodexModelFields
				catalog={catalog}
				options={options}
				onChange={onChange}
				disabled={disabled}
			/>
			{local.isLoading && (
				<p role="status" className="text-xs text-muted-foreground">
					{t("gateway.codex.modelsLoading")}
				</p>
			)}
			{!local.isLoading && !catalog?.models?.length && (
				<div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
					<span role="status">{t("gateway.codex.modelsUnavailable")}</span>
					{local.error && (
						<Button
							type="button"
							size="sm"
							variant="outline"
							disabled={disabled || local.isValidating}
							onClick={() => void local.mutate()}
						>
							{t("workspaces.retry")}
						</Button>
					)}
				</div>
			)}
		</div>
	);
}

function CodexModelFields({
	catalog,
	options,
	onChange,
	disabled,
}: {
	catalog?: CodexModelCatalog;
	options: CodexOptions;
	onChange(options: CodexOptions): void;
	disabled?: boolean;
}) {
	const { t } = useTranslation();
	const models = catalog?.models ?? [];
	const defaultModel = catalog?.defaultModel;
	const configuredEffort = !options.model || options.model === defaultModel;
	const efforts =
		models.find((model) => model.id === (options.model || defaultModel))?.efforts ?? [];
	const defaultLabel = (value?: string) =>
		value ? t("gateway.codex.defaultValue", { value }) : t("gateway.codex.default");

	return (
		<div className="grid gap-3 sm:grid-cols-2">
			<Choice
				label={t("gateway.codex.model")}
				value={options.model ?? ""}
				options={models.map(({ id, name }) => ({ id, name: name || id }))}
				placeholder={defaultLabel(defaultModel)}
				disabled={disabled || models.length === 0}
				onChange={(model) => {
					const supported = models.find((entry) => entry.id === (model || defaultModel))?.efforts;
					onChange({
						model: model || undefined,
						effort:
							options.effort && supported?.includes(options.effort) ? options.effort : undefined,
					});
				}}
			/>
			<Choice
				label={t("gateway.codex.effort")}
				value={options.effort ?? ""}
				options={efforts.map((effort) => ({ id: effort, name: effort }))}
				placeholder={defaultLabel(configuredEffort ? catalog?.defaultEffort : undefined)}
				disabled={disabled || efforts.length === 0}
				onChange={(effort) => onChange({ ...options, effort: effort || undefined })}
			/>
		</div>
	);
}
