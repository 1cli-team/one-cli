import type React from "react";
import { Field, FieldLabel } from "@/components/ui/field";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import type { ProjectSettings, ProjectSettingsResponse } from "@/types/api";

export interface ProjectSettingsFormProps {
	project: ProjectSettings;
	revision: string;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
	onUpdated(next: ProjectSettingsResponse): void;
	onDirtyChange(dirty: boolean): void;
}

export const ManifestDraftLayout: React.FC<React.PropsWithChildren> = ({ children }) => (
	<section className="space-y-4">{children}</section>
);

export const FormLayout: React.FC<
	React.PropsWithChildren<{
		title: string;
	}>
> = ({ title, children }) => (
	<section aria-label={title} className="space-y-4">
		{children}
	</section>
);

export const ProjectField: React.FC<
	React.PropsWithChildren<{ label: string; htmlFor: string }>
> = ({ label, htmlFor, children }) => (
	<Field className="gap-1.5">
		<FieldLabel htmlFor={htmlFor}>{label}</FieldLabel>
		{children}
	</Field>
);

export const SwitchField: React.FC<{
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
			"flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-background p-3 transition-colors hover:border-primary/25 hover:bg-primary/[0.025]",
			disabled && "cursor-not-allowed opacity-50",
		)}
	>
		<Switch
			id={id}
			checked={checked}
			onCheckedChange={onChange}
			disabled={disabled}
			className="mt-0.5"
		/>
		<FieldLabel htmlFor={id} className="min-w-0 cursor-inherit font-normal">
			<span className="block text-sm font-medium">{label}</span>
			<span className="mt-1 block text-xs leading-relaxed text-muted-foreground">
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
		<p className="text-[11px] font-medium text-muted-foreground">{label}</p>
		<div className={cn("mt-1.5 truncate text-sm font-medium", mono && "font-mono", className)}>
			{value || "-"}
		</div>
	</div>
);
