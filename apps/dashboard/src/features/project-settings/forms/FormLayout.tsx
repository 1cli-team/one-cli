import { Code2, Save } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";
import type { BackendDomain, ProjectSettings, ProjectSettingsResponse } from "@/types/api";

export interface ProjectSettingsFormProps {
	project: ProjectSettings;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
	onUpdated(next: ProjectSettingsResponse): void;
	onDirtyChange(dirty: boolean): void;
}

export const ReadOnlyLayout: React.FC<
	React.PropsWithChildren<{ title: string; description: string }>
> = ({ title, description, children }) => {
	const { t } = useTranslation();
	return (
		<section className="space-y-5">
			<div>
				<div className="flex items-center gap-2">
					<h3 className="text-sm font-semibold">{title}</h3>
					<span className="rounded-full border border-border bg-muted/45 px-2 py-0.5 text-[9px] font-semibold tracking-wider text-muted-foreground uppercase">
						{t("projectInspector.manifestReadOnly")}
					</span>
				</div>
				<p className="mt-1 text-xs leading-relaxed text-muted-foreground">{description}</p>
			</div>
			<div className="space-y-4">{children}</div>
		</section>
	);
};

export const FormLayout: React.FC<
	React.PropsWithChildren<{
		title: string;
		description: string;
		onSave(): void | Promise<void>;
		saving: boolean;
		disabled?: boolean;
	}>
> = ({ title, description, onSave, saving, disabled, children }) => {
	const { t } = useTranslation();
	return (
		<form
			onSubmit={(event) => {
				event.preventDefault();
				void onSave();
			}}
			className="space-y-5"
		>
			<div>
				<div className="flex items-center gap-2">
					<h3 className="text-sm font-semibold">{title}</h3>
					<span className="rounded-full border border-primary/20 bg-primary/8 px-2 py-0.5 text-[9px] font-semibold tracking-wider text-primary uppercase">
						{t("projectInspector.profileOnly")}
					</span>
				</div>
				<p className="mt-1 text-xs leading-relaxed text-muted-foreground">{description}</p>
			</div>
			<div className="space-y-4">{children}</div>
			<div className="sticky bottom-0 -mx-6 mt-6 flex items-center justify-between border-t border-border bg-card/95 px-6 py-4 backdrop-blur">
				<p className="text-[11px] text-muted-foreground">{t("projectInspector.saveHint")}</p>
				<Button type="submit" size="sm" disabled={saving || disabled}>
					{saving ? <Spinner /> : <Save />}
					{saving ? t("projectInspector.saving") : t("projectInspector.save")}
				</Button>
			</div>
		</form>
	);
};

export const ProjectField: React.FC<
	React.PropsWithChildren<{ label: string; htmlFor: string }>
> = ({ label, htmlFor, children }) => (
	<Field className="gap-1.5">
		<FieldLabel htmlFor={htmlFor}>{label}</FieldLabel>
		{children}
	</Field>
);

export const CheckField: React.FC<{
	id: string;
	label: string;
	description: string;
	checked: boolean;
	onChange(value: boolean): void;
	disabled?: boolean;
}> = ({ id, label, description, checked, onChange, disabled }) => (
	<Field
		orientation="horizontal"
		className={cn(
			"flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-muted/20 p-3 transition-colors hover:bg-muted/45",
			disabled && "cursor-not-allowed opacity-50",
		)}
	>
		<Checkbox
			id={id}
			checked={checked}
			onCheckedChange={(value) => onChange(value === true)}
			disabled={disabled}
			className="mt-0.5"
		/>
		<FieldLabel htmlFor={id} className="min-w-0 cursor-inherit font-normal">
			<span className="block text-xs font-medium">{label}</span>
			<span className="mt-0.5 block text-[10px] leading-relaxed text-muted-foreground">
				{description}
			</span>
		</FieldLabel>
	</Field>
);

export const ReadOnlyDatum: React.FC<{
	label: string;
	value?: React.ReactNode;
	mono?: boolean;
	className?: string;
}> = ({ label, value, mono, className }) => (
	<div>
		<p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
			{label}
		</p>
		<div className={cn("mt-1 truncate text-xs font-medium", mono && "font-mono", className)}>
			{value || "—"}
		</div>
	</div>
);

export const BackendBanner: React.FC<{ domain: BackendDomain; backend?: string }> = ({
	domain,
	backend,
}) => {
	const { t } = useTranslation();
	return (
		<div className="flex items-center gap-3 rounded-lg border border-primary/15 bg-primary/6 p-3.5">
			<div className="grid h-8 w-8 place-items-center rounded-md bg-primary/12 text-primary">
				<Code2 className="h-4 w-4" />
			</div>
			<div className="min-w-0">
				<p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
					{t("projectInspector.inheritedBackend", { domain })}
				</p>
				<p className="truncate text-sm font-semibold">
					{backend || t("projects.matrix.notConfigured")}
				</p>
			</div>
		</div>
	);
};
