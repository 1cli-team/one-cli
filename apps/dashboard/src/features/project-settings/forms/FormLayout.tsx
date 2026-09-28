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
			"flex items-start gap-4 border-b border-border py-4 last:border-b-0",
			disabled && "cursor-not-allowed opacity-50",
		)}
	>
		<FieldLabel htmlFor={id} className="block min-w-0 flex-1 cursor-pointer font-normal">
			<span className="block text-sm font-medium">{label}</span>
			<span className="mt-1 block text-sm text-muted-foreground">{description}</span>
		</FieldLabel>
		<Switch
			id={id}
			checked={checked}
			onCheckedChange={onChange}
			disabled={disabled}
			className="mt-0.5"
		/>
	</Field>
);

export const ReadOnlyDatum: React.FC<{
	label: string;
	value?: React.ReactNode;
	mono?: boolean;
	className?: string;
}> = ({ label, value, mono, className }) => (
	<div>
		<p className="text-xs font-medium text-muted-foreground">{label}</p>
		<div className={cn("mt-1.5 break-words text-sm font-medium", mono && "font-mono", className)}>
			{value || "-"}
		</div>
	</div>
);
