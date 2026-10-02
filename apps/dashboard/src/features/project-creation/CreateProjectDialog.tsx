import { useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR, { useSWRConfig } from "swr";
import { createProject, getProjectTemplates, projectTemplatesKey } from "@/api/project-creation";
import { workspaceBasePath, workspacesKey } from "@/api/workspaces";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectGroup,
	SelectItem,
	SelectLabel,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/hooks/useToast";
import type { HttpError } from "@/types/api";

interface Props {
	entryId: string;
	projectNames: string[];
	onClose(): void;
	onCreated(name: string): void;
}

export function CreateProjectDialog({ entryId, projectNames, onClose, onCreated }: Props) {
	const { t } = useTranslation();
	const { mutate } = useSWRConfig();
	const toast = useToast();
	const id = useId();
	const nameInput = useRef<HTMLInputElement>(null);
	const submitting = useRef(false);
	const [name, setName] = useState("");
	const [templateId, setTemplateId] = useState("react-spa");
	const [installSkills, setInstallSkills] = useState(true);
	const [pending, setPending] = useState(false);
	const [nameError, setNameError] = useState("");
	const [failure, setFailure] = useState<HttpError>();
	const catalog = useSWR(projectTemplatesKey, getProjectTemplates);
	const templates = catalog.data?.templates ?? [];
	const selected = templates.find((entry) => entry.id === templateId) ?? templates[0];
	const cleanName = name.trim();
	const validName = /^[a-zA-Z0-9][a-zA-Z0-9_-]*$/.test(cleanName);
	const generatedNames = selected?.projects?.map((project) => `${cleanName}${project.suffix}`) ?? [
		cleanName,
	];
	const categoryKeys = ["frontend", "backend", "library"] as const;

	async function submit(event: React.FormEvent) {
		event.preventDefault();
		if (submitting.current || !selected) return;
		const validation = !validName
			? "projectCreate.nameInvalid"
			: [cleanName, ...generatedNames].some((project) => projectNames.includes(project))
				? "projectCreate.nameExists"
				: "";
		setNameError(validation);
		if (validation) {
			nameInput.current?.focus();
			return;
		}
		submitting.current = true;
		setPending(true);
		setFailure(undefined);
		let result;
		try {
			result = await createProject(entryId, {
				name: cleanName,
				templateId: selected.id,
				...(!installSkills && { skipSkills: true }),
			});
		} catch (cause) {
			setFailure(cause as HttpError);
			setPending(false);
			submitting.current = false;
			return;
		}
		// A refresh failure must never turn a successful creation into a retry.
		const refreshed = await Promise.allSettled([
			mutate((key) => typeof key === "string" && key.startsWith(`${workspaceBasePath(entryId)}/`)),
			mutate(workspacesKey),
		]);
		onCreated(result.projects?.[0]?.name ?? result.name);
		toast.success(
			t(result.projects?.length ? "projectCreate.groupSuccess" : "projectCreate.success", {
				name: result.name,
				count: result.projects?.length,
			}),
		);
		for (const warning of result.warnings ?? []) toast.warning(warning);
		if (refreshed.some((item) => item.status === "rejected"))
			toast.warning(t("projectCreate.refreshFailed"));
		onClose();
	}

	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open && !submitting.current) onClose();
			}}
		>
			<DialogContent
				showCloseButton={!pending}
				onInteractOutside={(event) => event.preventDefault()}
			>
				<DialogHeader>
					<DialogTitle>{t("projectCreate.title")}</DialogTitle>
					<DialogDescription>{t("projectCreate.description")}</DialogDescription>
				</DialogHeader>
				<form className="space-y-5" onSubmit={submit} noValidate aria-busy={pending}>
					<Field>
						<FieldLabel htmlFor={`${id}-name`}>{t("projectCreate.name")}</FieldLabel>
						<Input
							id={`${id}-name`}
							ref={nameInput}
							value={name}
							placeholder="web"
							autoComplete="off"
							disabled={pending}
							aria-invalid={!!nameError}
							aria-describedby={`${id}-name-help`}
							onChange={(event) => {
								setName(event.target.value);
								setNameError("");
							}}
						/>
						<FieldDescription
							id={`${id}-name-help`}
							role={nameError ? "alert" : undefined}
							className={nameError ? "text-destructive" : undefined}
						>
							{t(nameError || "projectCreate.nameHint")}
						</FieldDescription>
					</Field>
					<Field>
						<FieldLabel htmlFor={`${id}-template`}>{t("projectCreate.template")}</FieldLabel>
						{catalog.error ? (
							<Alert variant="destructive">
								<AlertTitle>{t("projectCreate.templatesFailed")}</AlertTitle>
								<AlertDescription>
									<Button variant="outline" type="button" onClick={() => void catalog.mutate()}>
										{t("workspaces.retry")}
									</Button>
								</AlertDescription>
							</Alert>
						) : !selected ? (
							<p role="status" className="text-sm text-muted-foreground">
								{t(
									catalog.isLoading
										? "projectCreate.loadingTemplates"
										: "projectCreate.noTemplates",
								)}
							</p>
						) : (
							<>
								<Select
									value={selected?.id ?? ""}
									onValueChange={setTemplateId}
									disabled={pending || !selected}
								>
									<SelectTrigger
										id={`${id}-template`}
										className="w-full"
										aria-describedby={`${id}-template-help`}
									>
										<SelectValue
											placeholder={t(
												catalog.isLoading
													? "projectCreate.loadingTemplates"
													: "projectCreate.noTemplates",
											)}
										/>
									</SelectTrigger>
									<SelectContent>
										{categoryKeys.map((category) => (
											<SelectGroup key={category}>
												<SelectLabel>{t(`projectCreate.categories.${category}`)}</SelectLabel>
												{templates
													.filter((entry) => entry.category === category)
													.map((entry) => (
														<SelectItem key={entry.id} value={entry.id}>
															{t(`projectCreate.templates.${entry.id}.name`, {
																defaultValue: entry.name,
															})}
														</SelectItem>
													))}
											</SelectGroup>
										))}
									</SelectContent>
								</Select>
								{selected && (
									<FieldDescription id={`${id}-template-help`}>
										{t(`projectCreate.templates.${selected.id}.description`, {
											defaultValue: selected.description,
										})}
									</FieldDescription>
								)}
							</>
						)}
					</Field>
					{selected && (
						<div className="rounded-md border border-border bg-muted/40 p-3">
							<p className="text-xs text-muted-foreground">{t("projectCreate.location")}</p>
							{(selected.projects ?? [{ directory: selected.directory, suffix: "" }]).map(
								(project) => (
									<p
										key={`${project.directory}${project.suffix}`}
										className="mt-1 break-all font-mono text-sm"
									>
										{project.directory}/{validName ? cleanName : "…"}
										{project.suffix}
									</p>
								),
							)}
							<p className="mt-2 text-xs leading-relaxed text-muted-foreground">
								{t(
									selected.projects?.length
										? "projectCreate.groupLocationHint"
										: "projectCreate.locationHint",
								)}
							</p>
						</div>
					)}
					<Field>
						<div className="flex items-center justify-between gap-3">
							<FieldLabel htmlFor={`${id}-skills`}>{t("projectCreate.installSkills")}</FieldLabel>
							<Switch
								id={`${id}-skills`}
								checked={installSkills}
								onCheckedChange={setInstallSkills}
								disabled={pending}
								aria-describedby={`${id}-skills-help`}
							/>
						</div>
						<FieldDescription id={`${id}-skills-help`}>
							{t("projectCreate.skillsHint")}
						</FieldDescription>
					</Field>
					{failure && (
						<Alert variant="destructive" role="alert">
							<AlertTitle>{t("projectCreate.failed")}</AlertTitle>
							<AlertDescription>
								{failure.status === 0 ? t("projectCreate.connectionLost") : failure.message}
							</AlertDescription>
						</Alert>
					)}
					{pending && (
						<p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
							<Spinner aria-hidden="true" />
							{t("projectCreate.creating")}
						</p>
					)}
					<DialogFooter>
						<Button type="button" variant="outline" disabled={pending} onClick={onClose}>
							{t("form.cancel")}
						</Button>
						<Button type="submit" disabled={pending || !selected || !!catalog.error}>
							{t("projectCreate.submit")}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
