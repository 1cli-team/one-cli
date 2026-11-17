import { RestrictToVerticalAxis } from "@dnd-kit/abstract/modifiers";
import { Accessibility } from "@dnd-kit/dom";
import {
	DragDropProvider,
	DragOverlay,
	type DragEndEvent,
	type DragOverEvent,
	type DragStartEvent,
} from "@dnd-kit/react";
import { isSortable, useSortable } from "@dnd-kit/react/sortable";
import { FolderGit2, FolderKanban, GripVertical, Pencil, Plus, Trash2, X } from "lucide-react";
import { useEffect, useRef, useState, type ComponentProps, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";
import {
	addGroupMembers,
	createWorkspaceGroup,
	deleteWorkspaceGroup,
	moveGroupMember,
	removeGroupMember,
	renameWorkspaceGroup,
	type WorkspaceGroup,
} from "@/api/workspace-groups";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice, PageHeader, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { OpenWorkspace } from "@/lib/desktop/OpenWorkspace";
import { cn } from "@/lib/utils";
import { groupPath, groupWorkspacePath, workspacePath } from "@/router/paths";
import type { WorkspaceRegistryEntry } from "@/types/api";
import { useWorkspaceGroups } from "./useWorkspaceGroups";

function errorMessage(error: unknown) {
	return (error as Error)?.message || String(error);
}

export function WorkspaceGroupsPage() {
	const { t } = useTranslation();
	const { groupId } = useParams();
	const navigate = useNavigate();
	const result = useWorkspaceGroups();
	const [creating, setCreating] = useState(false);
	const refresh = () => result.mutate();
	if (result.error)
		return (
			<ErrorNotice
				action={
					<Button variant="outline" onClick={() => void refresh()}>
						{t("workspaces.retry")}
					</Button>
				}
			>
				{errorMessage(result.error)}
			</ErrorNotice>
		);
	if (!result.data) return <Skeleton className="h-64 w-full" />;
	const data = result.data;
	if (groupId) {
		const group = data.groups.find((item) => item.id === groupId);
		if (!group)
			return (
				<StatePanel icon={FolderKanban} title={t("groups.notFound")}>
					<Button asChild>
						<EnvironmentLink to="/">{t("groups.allProjects")}</EnvironmentLink>
					</Button>
				</StatePanel>
			);
		return (
			<GroupDetail key={group.id} group={group} workspaces={data.workspaces} onChange={refresh} />
		);
	}
	return (
		<div className="flex flex-col gap-6">
			<PageHeader
				title={t("groups.title")}
				actions={
					<>
						<Button variant="outline" onClick={() => setCreating(true)}>
							<Plus />
							{t("groups.createGroup")}
						</Button>
						<OpenWorkspace label={t("groups.openProject")} />
					</>
				}
			/>
			{data.projects.length ? (
				<div className="grid gap-4 ud-md:grid-cols-2 xl:grid-cols-3">
					{data.projects.map((project) => {
						const workspace =
							project.kind === "workspace"
								? data.workspaces.find((entry) => entry.entryId === project.id)
								: undefined;
						const Icon = project.kind === "workspace" ? FolderGit2 : FolderKanban;
						return (
							<EnvironmentLink
								key={project.id}
								to={
									project.kind === "workspace" ? workspacePath(project.id) : groupPath(project.id)
								}
								className="flex min-w-0 items-start gap-3 rounded-lg border border-border bg-card p-5 hover:bg-muted/40 focus-visible:outline-ring"
							>
								<Icon className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
								<div className="min-w-0 flex-1">
									<h2 className="break-words font-medium">{project.name}</h2>
									<p className="mt-1 text-sm text-muted-foreground">
										{t("groups.memberCount", { count: project.workspaceIds.length })}
									</p>
									{workspace && (
										<p
											className="mt-2 truncate font-mono text-xs text-muted-foreground"
											title={workspace.root}
										>
											{workspace.root}
										</p>
									)}
								</div>
							</EnvironmentLink>
						);
					})}
				</div>
			) : (
				<StatePanel
					icon={FolderKanban}
					title={t("groups.empty")}
					description={t("groups.emptyDescription")}
				/>
			)}

			{creating && (
				<GroupNameDialog
					title={t("groups.createGroup")}
					onClose={() => setCreating(false)}
					onSave={async (name) => {
						const group = await createWorkspaceGroup(name);
						setCreating(false);
						await refresh();
						navigate(groupPath(group.id));
					}}
				/>
			)}
		</div>
	);
}

function GroupDetail({
	group,
	workspaces,
	onChange,
}: {
	group: WorkspaceGroup;
	workspaces: WorkspaceRegistryEntry[];
	onChange(): Promise<unknown>;
}) {
	const { t } = useTranslation();
	const navigate = useNavigate();
	const [dialog, setDialog] = useState<"rename" | "delete" | "members" | null>(null);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [selected, setSelected] = useState<string[]>([]);
	const [orderedIds, setOrderedIds] = useState(group.workspaceIds);
	const dragging = useRef(false);
	useEffect(() => {
		if (!dragging.current && !busy) setOrderedIds(group.workspaceIds);
	}, [group.workspaceIds, busy]);
	const candidates = workspaces.filter(
		(workspace) => !group.workspaceIds.includes(workspace.entryId),
	);
	async function act(action: () => Promise<unknown>) {
		if (busy) return false;
		setBusy(true);
		setError("");
		try {
			await action();
			await onChange();
			return true;
		} catch (error) {
			setError(errorMessage(error));
			return false;
		} finally {
			setBusy(false);
		}
	}
	function memberName(id: unknown) {
		return (
			workspaces.find((workspace) => workspace.entryId === id)?.name ?? t("groups.missingMember")
		);
	}
	function openDialog(value: typeof dialog) {
		setError("");
		setSelected([]);
		setDialog(value);
	}
	return (
		<div className="flex flex-col gap-6">
			<PageHeader
				title={group.name}
				description={t("groups.memberCount", { count: group.workspaceIds.length })}
				actions={
					<>
						<Button variant="ghost" disabled={busy} onClick={() => openDialog("rename")}>
							<Pencil />
							{t("groups.rename")}
						</Button>
						<Button variant="danger-ghost" disabled={busy} onClick={() => openDialog("delete")}>
							<Trash2 />
							{t("groups.delete")}
						</Button>
						<Button variant="outline" disabled={busy} onClick={() => openDialog("members")}>
							<Plus />
							{t("groups.addMembers")}
						</Button>
						<OpenWorkspace groupId={group.id} />
					</>
				}
			/>
			{error && !dialog && <ErrorNotice>{error}</ErrorNotice>}
			<DragDropProvider
				modifiers={[RestrictToVerticalAxis]}
				plugins={(defaults) => [
					...defaults,
					Accessibility.configure({
						screenReaderInstructions: { draggable: t("groups.reorderHelp") },
						announcements: {
							dragstart: ({ operation }: DragStartEvent) =>
								t("groups.reorderStarted", { name: memberName(operation.source?.id) }),
							dragover: ({ operation }: DragOverEvent) =>
								isSortable(operation.source)
									? t("groups.reorderPosition", {
											name: memberName(operation.source.id),
											position: operation.source.index + 1,
										})
									: undefined,
							dragend: ({ operation, canceled }: DragEndEvent) =>
								t(canceled ? "groups.reorderCanceled" : "groups.reorderFinished", {
									name: memberName(operation.source?.id),
								}),
						},
					}),
				]}
				onBeforeDragStart={(event) => {
					if (busy || orderedIds.length < 2) event.preventDefault();
				}}
				onDragStart={() => {
					dragging.current = true;
				}}
				onDragEnd={async (event) => {
					dragging.current = false;
					const { source, target } = event.operation;
					if (
						event.canceled ||
						!target ||
						!isSortable(source) ||
						source.initialIndex === source.index
					) {
						setOrderedIds(group.workspaceIds);
						return;
					}
					const previous = orderedIds;
					const next = orderedIds.filter((id) => id !== source.id);
					next.splice(source.index, 0, String(source.id));
					setOrderedIds(next);
					const saved = await act(() =>
						moveGroupMember(group.id, String(source.id), next[source.index + 1] ?? ""),
					);
					if (!saved) setOrderedIds(previous);
				}}
			>
				<div className="divide-y divide-border rounded-lg border border-border" aria-busy={busy}>
					{orderedIds.map((id, index) => (
						<SortableWorkspaceMember
							key={id}
							id={id}
							index={index}
							groupId={group.id}
							workspace={workspaces.find((workspace) => workspace.entryId === id)}
							disabled={busy}
							canReorder={orderedIds.length > 1}
							onRemove={() => void act(() => removeGroupMember(group.id, id))}
						/>
					))}
					{!orderedIds.length && (
						<StatePanel
							icon={FolderGit2}
							title={t("groups.noMembers")}
							description={t("groups.noMembersDescription")}
						/>
					)}
				</div>
				<DragOverlay dropAnimation={{ duration: 180, easing: "cubic-bezier(0.2, 0, 0, 1)" }}>
					{(source) => (
						<WorkspaceMember
							id={String(source.id)}
							workspace={workspaces.find((workspace) => workspace.entryId === source.id)}
							rowProps={{
								inert: true,
								"aria-hidden": true,
								className:
									"rounded-lg border border-primary/30 bg-card shadow-xl ring-1 ring-primary/10",
							}}
							dragHandle={
								<span className="flex size-7 shrink-0 items-center justify-center text-primary-text">
									<GripVertical className="size-4" />
								</span>
							}
						/>
					)}
				</DragOverlay>
			</DragDropProvider>
			{dialog === "rename" && (
				<GroupNameDialog
					title={t("groups.rename")}
					initialName={group.name}
					onClose={() => setDialog(null)}
					onSave={async (name) => {
						await renameWorkspaceGroup(group.id, name);
						setDialog(null);
						await onChange();
					}}
				/>
			)}
			<Dialog
				open={dialog === "members" || dialog === "delete"}
				onOpenChange={(open) => {
					if (!open && !busy) setDialog(null);
				}}
			>
				<DialogContent showCloseButton={!busy}>
					<DialogHeader>
						<DialogTitle>
							{t(dialog === "delete" ? "groups.delete" : "groups.addMembers")}
						</DialogTitle>
						<DialogDescription>
							{t(dialog === "delete" ? "groups.deleteDescription" : "groups.memberDescription", {
								name: group.name,
							})}
						</DialogDescription>
					</DialogHeader>
					{dialog === "members" && (
						<div className="max-h-80 space-y-2 overflow-y-auto">
							{candidates.map((workspace) => (
								<label
									key={workspace.entryId}
									className="flex cursor-pointer items-center gap-3 rounded-md border border-border p-3"
								>
									<input
										type="checkbox"
										className="size-4 shrink-0 accent-primary"
										disabled={busy}
										checked={selected.includes(workspace.entryId)}
										onChange={(event) =>
											setSelected((ids) =>
												event.target.checked
													? [...ids, workspace.entryId]
													: ids.filter((id) => id !== workspace.entryId),
											)
										}
									/>
									<span className="min-w-0">
										<span className="block break-words text-sm font-medium">{workspace.name}</span>
										<span
											className="block truncate font-mono text-xs text-muted-foreground"
											title={workspace.root}
										>
											{workspace.root}
										</span>
									</span>
								</label>
							))}
							{!candidates.length && (
								<p className="text-sm text-muted-foreground">{t("groups.noCandidates")}</p>
							)}
						</div>
					)}
					{error && <ErrorNotice>{error}</ErrorNotice>}
					<DialogFooter>
						<Button variant="outline" disabled={busy} onClick={() => setDialog(null)}>
							{t("git.cancel")}
						</Button>
						<Button
							variant={dialog === "delete" ? "destructive" : "default"}
							disabled={busy || (dialog === "members" && !selected.length)}
							onClick={() =>
								void act(async () => {
									if (dialog === "delete") {
										await deleteWorkspaceGroup(group.id);
										setDialog(null);
										navigate("/");
									} else {
										await addGroupMembers(group.id, selected);
										setDialog(null);
									}
								})
							}
						>
							{busy
								? t("git.working")
								: t(dialog === "delete" ? "groups.delete" : "groups.addMembers")}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}

function SortableWorkspaceMember({
	id,
	index,
	groupId,
	workspace,
	disabled,
	canReorder,
	onRemove,
}: {
	id: string;
	index: number;
	groupId: string;
	workspace?: WorkspaceRegistryEntry;
	disabled: boolean;
	canReorder: boolean;
	onRemove(): void;
}) {
	const { t } = useTranslation();
	const { ref, handleRef, isDragSource } = useSortable({
		id,
		index,
		disabled: disabled || !canReorder,
		transition: { duration: 200, easing: "cubic-bezier(0.2, 0, 0, 1)", idle: true },
	});
	return (
		<WorkspaceMember
			id={id}
			groupId={groupId}
			workspace={workspace}
			rowProps={{
				ref,
				className: cn(
					"relative bg-card transition-colors first:rounded-t-lg last:rounded-b-lg",
					isDragSource ? "bg-primary/5 opacity-35" : "hover:bg-muted/40",
				),
			}}
			dragHandle={
				<Button
					ref={handleRef}
					variant="ghost"
					size="icon"
					className="touch-none cursor-grab text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary-text active:cursor-grabbing aria-disabled:cursor-default aria-disabled:opacity-40"
					aria-disabled={disabled || !canReorder}
					aria-roledescription={t("groups.reorderRole")}
					aria-label={t("groups.reorderMember", { name: workspace?.name ?? id })}
					title={t("groups.reorderHelp")}
				>
					<GripVertical />
				</Button>
			}
			actions={
				<Button
					variant="danger-ghost"
					size="icon"
					disabled={disabled}
					aria-label={t("groups.removeMember")}
					title={t("groups.removeMember")}
					onClick={onRemove}
				>
					<X />
				</Button>
			}
		/>
	);
}

function WorkspaceMember({
	id,
	workspace,
	groupId,
	actions,
	dragHandle,
	rowProps,
}: {
	id: string;
	workspace?: WorkspaceRegistryEntry;
	groupId?: string;
	actions?: ReactNode;
	dragHandle?: ReactNode;
	rowProps?: ComponentProps<"article">;
}) {
	const { t } = useTranslation();
	const content = (
		<>
			<FolderGit2 className="size-5 shrink-0 text-muted-foreground" />
			<div className="min-w-0 flex-1">
				<h3 className="break-words text-sm font-medium">
					{workspace?.name ?? t("groups.missingMember")}
				</h3>
				<p
					className="truncate font-mono text-xs text-muted-foreground"
					title={workspace?.root ?? id}
				>
					{workspace?.root ?? id}
				</p>
			</div>
			<span className="shrink-0 text-xs text-muted-foreground">
				{workspace && t(`workspaces.status.${workspace.status}`)}
			</span>
		</>
	);
	return (
		<article {...rowProps} className={cn("flex items-center gap-2 p-3", rowProps?.className)}>
			{dragHandle}
			{workspace ? (
				<EnvironmentLink
					draggable={false}
					className="flex min-w-0 flex-1 items-center gap-3 rounded-md p-2"
					to={
						groupId
							? groupWorkspacePath(groupId, workspace.entryId)
							: workspacePath(workspace.entryId)
					}
				>
					{content}
				</EnvironmentLink>
			) : (
				<div className="flex min-w-0 flex-1 items-center gap-3 p-2">{content}</div>
			)}
			{actions && <div className="ml-auto flex shrink-0 items-center">{actions}</div>}
		</article>
	);
}

function GroupNameDialog({
	title,
	initialName = "",
	onClose,
	onSave,
}: {
	title: string;
	initialName?: string;
	onClose(): void;
	onSave(name: string): Promise<void>;
}) {
	const { t } = useTranslation();
	const [name, setName] = useState(initialName);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	async function submit() {
		setBusy(true);
		setError("");
		try {
			await onSave(name.trim());
		} catch (error) {
			setError(errorMessage(error));
		} finally {
			setBusy(false);
		}
	}
	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open && !busy) onClose();
			}}
		>
			<DialogContent showCloseButton={!busy}>
				<DialogHeader>
					<DialogTitle>{title}</DialogTitle>
					<DialogDescription>{t("groups.nameDescription")}</DialogDescription>
				</DialogHeader>
				<form
					className="space-y-4"
					onSubmit={(event) => {
						event.preventDefault();
						void submit();
					}}
				>
					<div className="space-y-2">
						<Label htmlFor="group-name">{t("groups.name")}</Label>
						<Input
							id="group-name"
							value={name}
							onChange={(event) => setName(event.target.value)}
							disabled={busy}
							required
							autoFocus
						/>
					</div>
					{error && <ErrorNotice>{error}</ErrorNotice>}
					<DialogFooter>
						<Button type="button" variant="outline" disabled={busy} onClick={onClose}>
							{t("git.cancel")}
						</Button>
						<Button type="submit" disabled={busy || !name.trim()}>
							{busy ? t("git.working") : t("groups.save")}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
