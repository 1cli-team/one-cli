import { Save } from "lucide-react";
import type React from "react";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { humanizeBackendName } from "@/api/catalog";
import { upsertProfile } from "@/api/configure";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { useToast } from "@/hooks/useToast";
import type {
	AnyProfile,
	BackendFieldSpec,
	BackendSpec,
	ProfileValue,
	UpsertResponse,
} from "@/types/api";

const MASKED_SECRET = "********";

export interface ProfileEditorTarget {
	backend: BackendSpec;
	name: string;
	profile: AnyProfile;
	mode: "add" | "edit";
	hasDefault: boolean;
}

interface ProfileEditorDialogProps {
	target: ProfileEditorTarget | null;
	onOpenChange(open: boolean): void;
	onSaved?(result: UpsertResponse): void;
}

// ProfileEditorDialog owns the complete upsert workflow shared by routed
// profile management and the workspace repair flow. The last target remains
// available while Radix animates a closing dialog, avoiding an empty frame.
export const ProfileEditorDialog: React.FC<ProfileEditorDialogProps> = ({
	target,
	onOpenChange,
	onSaved,
}) => {
	const toast = useToast();
	const { t } = useTranslation();
	const lastTarget = useRef<ProfileEditorTarget | null>(null);
	if (target) lastTarget.current = target;
	const snapshot = target ?? lastTarget.current;

	async function handleSubmit(name: string, profile: AnyProfile, use: boolean) {
		if (!snapshot) return;
		try {
			const result = await upsertProfile(snapshot.backend.domain, snapshot.backend.name, {
				name,
				profile,
				use,
			});
			toast.success(
				result.status === "updated" ? t("toast.updated", { name }) : t("toast.created", { name }),
				{ description: result.default ? t("toast.setDefaultAfterSaveHint") : undefined },
			);
			onOpenChange(false);
			onSaved?.(result);
		} catch (error) {
			const failure = error as { code?: string; message: string };
			toast.error(failure.message, { description: failure.code });
		}
	}

	return (
		<Dialog open={target !== null} onOpenChange={onOpenChange}>
			<DialogContent>
				{snapshot ? (
					<ProfileForm
						key={`${snapshot.backend.id}-${snapshot.mode}-${snapshot.name}`}
						backend={snapshot.backend}
						initialName={snapshot.name}
						initialProfile={snapshot.profile}
						mode={snapshot.mode}
						hasDefault={snapshot.hasDefault}
						onCancel={() => onOpenChange(false)}
						onSubmit={handleSubmit}
					/>
				) : null}
			</DialogContent>
		</Dialog>
	);
};

export function emptyProfile(backend: BackendSpec): AnyProfile {
	let profile: AnyProfile = {};
	for (const field of backend.profile.fields ?? []) {
		const value = field.default ?? (field.type === "boolean" ? false : "");
		profile = setPath(profile, field.path, value);
	}
	return profile;
}

interface ProfileFormProps {
	backend: BackendSpec;
	initialName: string;
	initialProfile: AnyProfile;
	mode: "add" | "edit";
	hasDefault: boolean;
	onCancel(): void;
	onSubmit(name: string, profile: AnyProfile, use: boolean): Promise<void>;
}

const ProfileForm: React.FC<ProfileFormProps> = ({
	backend,
	initialName,
	initialProfile,
	mode,
	hasDefault,
	onCancel,
	onSubmit,
}) => {
	const { t } = useTranslation();
	const [name, setName] = useState(initialName);
	const [profile, setProfile] = useState<AnyProfile>(initialProfile);
	const [use, setUse] = useState(false);
	const [submitting, setSubmitting] = useState(false);

	async function handleSubmit(event: React.FormEvent) {
		event.preventDefault();
		setSubmitting(true);
		try {
			await onSubmit(name.trim(), profile, use);
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<>
			<DialogHeader>
				<DialogTitle>
					{mode === "add" ? t("form.addTitle") : t("form.editTitle", { name: initialName })}
				</DialogTitle>
				<DialogDescription>
					{mode === "add" ? t("form.addDescription") : t("form.editDescription")}
				</DialogDescription>
			</DialogHeader>
			<form onSubmit={handleSubmit} className="space-y-3">
				<FieldRow>
					<FieldLabel htmlFor="profile-name">{t("form.profileName")}</FieldLabel>
					<Input
						id="profile-name"
						value={name}
						onChange={(event) => setName(event.target.value)}
						placeholder={t("form.profileNamePlaceholder")}
						disabled={mode === "edit"}
						required
					/>
				</FieldRow>

				<BackendFields backend={backend} profile={profile} setProfile={setProfile} />

				<FieldRow>
					<div className="flex items-center gap-2">
						<Checkbox
							id="profile-set-default"
							checked={use}
							onCheckedChange={(checked) => setUse(checked === true)}
						/>
						<FieldLabel htmlFor="profile-set-default" className="text-sm font-normal">
							{hasDefault ? t("form.setDefaultAfterSave") : t("form.setDefaultAfterSaveAuto")}
						</FieldLabel>
					</div>
				</FieldRow>

				<DialogFooter className="pt-2">
					<Button type="button" variant="outline" onClick={onCancel}>
						{t("form.cancel")}
					</Button>
					<Button type="submit" disabled={submitting || !name.trim()}>
						{submitting ? <Spinner /> : <Save className="h-4 w-4" />}
						{t("form.save")}
					</Button>
				</DialogFooter>
			</form>
		</>
	);
};

const FieldRow: React.FC<React.PropsWithChildren> = ({ children }) => (
	<Field className="gap-2">{children}</Field>
);

interface BackendFieldsProps {
	backend: BackendSpec;
	profile: AnyProfile;
	setProfile(profile: AnyProfile): void;
}

const BackendFields: React.FC<BackendFieldsProps> = ({ backend, profile, setProfile }) => {
	const { t } = useTranslation();
	return (backend.profile.fields ?? []).map((field) => {
		const value = getPath(profile, field.path);
		const inputID = `profile-${backend.name}-${field.path.replaceAll("/", "-")}`;
		const leaf = field.path.split("/").at(-1) ?? field.path;
		const label = t(field.label_key, { defaultValue: humanizeBackendName(leaf) });
		if (field.type === "boolean") {
			return (
				<FieldRow key={field.path}>
					<div className="flex items-center gap-2">
						<Checkbox
							id={inputID}
							checked={value === true}
							onCheckedChange={(checked) =>
								setProfile(setPath(profile, field.path, checked === true))
							}
						/>
						<FieldLabel htmlFor={inputID} className="text-sm font-normal">
							{label}
						</FieldLabel>
					</div>
				</FieldRow>
			);
		}

		const maskedPlaceholder = t("form.fields.secretUnchangedPlaceholder");
		return (
			<FieldRow key={field.path}>
				<FieldLabel htmlFor={inputID}>{label}</FieldLabel>
				<Input
					id={inputID}
					type={field.type === "secret" ? "password" : "text"}
					value={field.type === "secret" ? secretInputValue(value) : String(value ?? "")}
					placeholder={
						field.type === "secret"
							? secretPlaceholder(value, maskedPlaceholder)
							: field.placeholder
					}
					onChange={(event) => setProfile(setPath(profile, field.path, event.target.value))}
					required={field.required && value !== MASKED_SECRET}
				/>
			</FieldRow>
		);
	});
};

export const ProfileSummary: React.FC<{ backend: BackendSpec; profile: AnyProfile }> = ({
	backend,
	profile,
}) => {
	const { t } = useTranslation();
	const values = (backend.profile.fields ?? [])
		.filter((field) => field.type !== "secret")
		.map((field) => [field, getPath(profile, field.path)] as const)
		.filter(([, value]) => value !== undefined && value !== "" && value !== false)
		.slice(0, 2);
	if (values.length === 0) {
		return <span className="text-xs">{t("form.summary.notSet")}</span>;
	}
	return (
		<span className="text-xs">
			{values.map(([field, value]) => `${summaryLabel(field)}: ${String(value)}`).join(" · ")}
		</span>
	);
};

function getPath(profile: AnyProfile, path: string): ProfileValue | undefined {
	let current: ProfileValue | undefined = profile;
	for (const part of path.split("/")) {
		if (!current || typeof current !== "object" || Array.isArray(current)) return undefined;
		current = current[part];
	}
	return current;
}

function setPath(profile: AnyProfile, path: string, value: ProfileValue): AnyProfile {
	const parts = path.split("/");
	const root: AnyProfile = { ...profile };
	let current = root;
	for (const [index, part] of parts.entries()) {
		if (index === parts.length - 1) {
			current[part] = value;
			break;
		}
		const existing = current[part];
		const child: AnyProfile =
			existing && typeof existing === "object" && !Array.isArray(existing) ? { ...existing } : {};
		current[part] = child;
		current = child;
	}
	return root;
}

function secretInputValue(value: ProfileValue | undefined): string {
	return value === MASKED_SECRET ? "" : typeof value === "string" ? value : "";
}

function secretPlaceholder(
	value: ProfileValue | undefined,
	placeholder: string,
): string | undefined {
	return value === MASKED_SECRET ? placeholder : undefined;
}

function summaryLabel(field: BackendFieldSpec): string {
	return humanizeBackendName(field.path.split("/").at(-1) ?? field.path);
}
