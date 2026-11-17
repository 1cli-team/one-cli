import type { RefObject } from "react";
import {
	ChevronDown,
	ChevronRight,
	Copy,
	Eye,
	EyeOff,
	Folder,
	FolderOpen,
	FolderPlus,
	MoreHorizontal,
	Pencil,
	Plus,
	RefreshCw,
	Trash2,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import {
	getGlobalListing,
	globalListingKey,
	globalQuery,
	message,
	type GlobalListing,
	type GlobalLocation,
} from "@/api/session";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { IconButton } from "@/components/ui/icon-button";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Spinner } from "@/components/ui/spinner";
import { TableCell, TableRow } from "@/components/ui/table";

export interface CredentialActions {
	read(path: string, key: string, copy: boolean): void;
	edit(path: string, key: string): void;
	remove(path: string, key: string): void;
	addVariable(path: string): void;
	addFolder(path: string): void;
}

interface GroupProps {
	location: GlobalLocation;
	environment: string;
	path: string;
	depth: number;
	isExpanded(path: string): boolean;
	toggle(path: string): void;
	actions: CredentialActions;
	revealed: Record<string, string>;
	disabled: boolean;
	actionTrigger: RefObject<HTMLButtonElement | null>;
	listing?: GlobalListing;
}

export function CredentialGroup(props: GroupProps) {
	const { t } = useTranslation();
	const {
		location,
		environment,
		path,
		depth,
		isExpanded,
		toggle,
		actions,
		revealed,
		disabled,
		actionTrigger,
		listing,
	} = props;
	const expanded = isExpanded(path);
	const query = globalQuery(environment, path);
	const result = useSWR(
		!listing && expanded ? globalListingKey(location, query) : null,
		() => getGlobalListing(query),
		{ keepPreviousData: true },
	);
	const data = listing ?? result.data;
	const name = path === "/" ? t("global.rootFolder") : listing ? path : path.split("/").pop();
	return (
		<>
			<TableRow className="bg-muted/40">
				<TableCell colSpan={3} className="p-0">
					<div className="flex items-center gap-2 pr-3">
						<Button
							variant="ghost"
							className="h-auto min-h-11 min-w-0 flex-1 justify-start rounded-none py-2 text-left"
							style={{ paddingLeft: 16 + depth * 20 }}
							aria-expanded={expanded}
							aria-label={t(expanded ? "global.collapseFolder" : "global.expandFolder", { name })}
							disabled={disabled}
							onClick={() => toggle(path)}
						>
							{expanded ? <ChevronDown /> : <ChevronRight />}
							{expanded ? (
								<FolderOpen className="text-muted-foreground" aria-hidden="true" />
							) : (
								<Folder className="text-muted-foreground" aria-hidden="true" />
							)}
							<span className="truncate" title={path}>
								{name}
							</span>
							{data && (
								<span className="shrink-0 text-xs font-normal text-muted-foreground">
									{t("global.count", { count: data.variables.length })}
								</span>
							)}
						</Button>
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button
									variant="ghost"
									size="icon"
									disabled={disabled}
									aria-label={t("global.folderActions", { path })}
									onPointerDown={(event) => {
										actionTrigger.current = event.currentTarget;
									}}
									onFocus={(event) => {
										actionTrigger.current = event.currentTarget;
									}}
								>
									<MoreHorizontal />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuGroup>
									<DropdownMenuItem onSelect={() => actions.addVariable(path)}>
										<Plus />
										{t("global.add")}
									</DropdownMenuItem>
									<DropdownMenuItem onSelect={() => actions.addFolder(path)}>
										<FolderPlus />
										{t("global.addFolder")}
									</DropdownMenuItem>
								</DropdownMenuGroup>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</TableCell>
			</TableRow>
			{expanded && (
				<>
					{result.isLoading && !data && (
						<TableRow>
							<TableCell colSpan={3}>
								<span
									role="status"
									className="flex items-center gap-2 text-sm text-muted-foreground"
								>
									<Spinner />
									{t("session.loading")}
								</span>
							</TableCell>
						</TableRow>
					)}
					{result.error && (
						<TableRow>
							<TableCell colSpan={3}>
								<ErrorNotice
									action={
										<Button variant="outline" size="sm" onClick={() => void result.mutate()}>
											<RefreshCw />
											{t("secrets.retry")}
										</Button>
									}
								>
									{message(result.error)}
								</ErrorNotice>
							</TableCell>
						</TableRow>
					)}
					{data && !result.error && (
						<>
							<CredentialVariables
								variables={data.variables}
								path={path}
								depth={depth + 1}
								environment={environment}
								revealed={revealed}
								disabled={disabled}
								actions={actions}
								actionTrigger={actionTrigger}
							/>
							{data.folders.map((child) => (
								<CredentialGroup
									{...props}
									key={child}
									path={child}
									depth={depth + 1}
									listing={undefined}
								/>
							))}
							{data.variables.length === 0 && data.folders.length === 0 && (
								<TableRow>
									<TableCell
										colSpan={3}
										className="py-5 text-sm text-muted-foreground"
										style={{ paddingLeft: 64 + depth * 20 }}
									>
										{t("global.empty")}
									</TableCell>
								</TableRow>
							)}
						</>
					)}
				</>
			)}
		</>
	);
}

function CredentialVariables({
	variables,
	path,
	depth,
	environment,
	revealed,
	disabled,
	actions,
	actionTrigger,
}: {
	variables: GlobalListing["variables"];
	path: string;
	depth: number;
	environment: string;
	revealed: Record<string, string>;
	disabled: boolean;
	actions: CredentialActions;
	actionTrigger: RefObject<HTMLButtonElement | null>;
}) {
	const { t } = useTranslation();
	const query = globalQuery(environment, path);
	return variables.map((variable) => {
		const value = revealed[`${query}:${variable.key}`];
		return (
			<TableRow key={variable.key}>
				<TableCell className="py-3 pr-4" style={{ paddingLeft: 60 + depth * 20 }}>
					<code className="break-all">{variable.key}</code>
					{variable.description && (
						<p className="mt-1 whitespace-pre-wrap text-xs text-muted-foreground">
							{variable.description}
						</p>
					)}
				</TableCell>
				<TableCell className="break-all px-4 py-3 font-mono text-xs text-muted-foreground">
					{value ?? "••••••••"}
				</TableCell>
				<TableCell className="px-4 py-3">
					<div className="flex justify-end gap-1">
						<IconButton
							label={t(value === undefined ? "global.reveal" : "global.hide")}
							disabled={disabled}
							onClick={() => actions.read(path, variable.key, false)}
						>
							{value === undefined ? <Eye /> : <EyeOff />}
						</IconButton>
						<IconButton
							label={t("global.copy")}
							disabled={disabled}
							onClick={() => actions.read(path, variable.key, true)}
						>
							<Copy />
						</IconButton>
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button
									variant="ghost"
									size="icon"
									disabled={disabled}
									aria-label={t("global.more", { key: variable.key })}
									onPointerDown={(event) => {
										actionTrigger.current = event.currentTarget;
									}}
									onFocus={(event) => {
										actionTrigger.current = event.currentTarget;
									}}
								>
									<MoreHorizontal />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuGroup>
									<DropdownMenuItem onSelect={() => actions.edit(path, variable.key)}>
										<Pencil />
										{t("global.edit")}
									</DropdownMenuItem>
								</DropdownMenuGroup>
								<DropdownMenuSeparator />
								<DropdownMenuGroup>
									<DropdownMenuItem
										variant="destructive"
										onSelect={() => actions.remove(path, variable.key)}
									>
										<Trash2 />
										{t("global.delete")}
									</DropdownMenuItem>
								</DropdownMenuGroup>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</TableCell>
			</TableRow>
		);
	});
}
