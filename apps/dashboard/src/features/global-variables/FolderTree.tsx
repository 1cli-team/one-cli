import { useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight, Folder, FolderOpen } from "lucide-react";
import useSWR from "swr";
import {
	getGlobalListing,
	globalListingKey,
	globalQuery,
	message,
	type GlobalLocation,
} from "@/api/session";
import { Button, buttonVariants } from "@/components/ui/button";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";

type Props = {
	location: GlobalLocation;
	environment: string;
	selectedPath: string;
	onSelect(path: string): void;
	disabled: boolean;
};

type BranchProps = Props & {
	path: string;
	depth: number;
	branches: Record<string, boolean>;
	setExpanded(path: string, expanded: boolean): void;
	focusedPath: string;
	onFocus(path: string): void;
};

export function FolderTree(props: Props) {
	const { t } = useTranslation();
	// Remember visited branches and expansion independently of the selected folder.
	const [branches, setBranches] = useState<Record<string, boolean>>({ "/": true });
	const [focusedPath, setFocusedPath] = useState("/");
	return (
		<ul role="tree" aria-label={t("global.folders")}>
			<FolderBranch
				{...props}
				onSelect={(path) => {
					setBranches((current) =>
						Object.hasOwn(current, path) ? current : { ...current, [path]: false },
					);
					props.onSelect(path);
				}}
				path="/"
				depth={0}
				branches={branches}
				setExpanded={(path, expanded) =>
					setBranches((current) => ({ ...current, [path]: expanded }))
				}
				focusedPath={focusedPath}
				onFocus={setFocusedPath}
			/>
		</ul>
	);
}

function FolderBranch(props: BranchProps) {
	const { t } = useTranslation();
	const {
		path,
		depth,
		branches,
		setExpanded,
		selectedPath,
		onSelect,
		disabled,
		focusedPath,
		onFocus,
	} = props;
	const expanded = branches[path] ?? false;
	const query = globalQuery(props.environment, path);
	const listing = useSWR(
		Object.hasOwn(branches, path) || selectedPath === path
			? globalListingKey(props.location, query)
			: null,
		() => getGlobalListing(query),
	);
	const folders = listing.data?.folders;
	const expandable = folders === undefined || folders.length > 0;
	const name = path === "/" ? t("global.rootFolder") : path.split("/").pop()!;

	function handleKeyDown(event: KeyboardEvent<HTMLLIElement>) {
		if (event.target !== event.currentTarget || disabled) return;
		const item = event.currentTarget;
		const items = Array.from(
			item.closest('[role="tree"]')?.querySelectorAll<HTMLElement>('[role="treeitem"]') ?? [],
		);
		const index = items.indexOf(item);
		switch (event.key) {
			case "ArrowDown":
				items[Math.min(index + 1, items.length - 1)]?.focus();
				break;
			case "ArrowUp":
				items[Math.max(index - 1, 0)]?.focus();
				break;
			case "Home":
				items[0]?.focus();
				break;
			case "End":
				items.at(-1)?.focus();
				break;
			case "ArrowRight":
				if (expandable && !expanded) setExpanded(path, true);
				else item.querySelector<HTMLElement>('[role="group"] [role="treeitem"]')?.focus();
				break;
			case "ArrowLeft":
				if (expandable && expanded) setExpanded(path, false);
				else item.parentElement?.closest<HTMLElement>('[role="treeitem"]')?.focus();
				break;
			case "Enter":
			case " ":
				onSelect(path);
				break;
			default:
				return;
		}
		event.preventDefault();
		event.stopPropagation();
	}
	return (
		<li
			role="treeitem"
			aria-label={name}
			aria-level={depth + 1}
			aria-selected={selectedPath === path}
			aria-expanded={expandable ? expanded : undefined}
			aria-disabled={disabled}
			tabIndex={focusedPath === path ? 0 : -1}
			className="outline-none [&:focus-visible>div]:ring-2 [&:focus-visible>div]:ring-ring/30"
			onFocus={(event) => {
				if (event.target === event.currentTarget) onFocus(path);
			}}
			onKeyDown={handleKeyDown}
		>
			<div
				className={cn(
					buttonVariants({ variant: "navigation", size: "navigation-compact" }),
					"cursor-pointer",
					disabled && "cursor-default opacity-50",
				)}
				aria-current={selectedPath === path ? "page" : undefined}
				style={{ paddingLeft: 4 + depth * 14 }}
				onClick={(event) => {
					event.stopPropagation();
					if (disabled) return;
					event.currentTarget.parentElement?.focus();
					onSelect(path);
				}}
			>
				{expandable ? (
					<Button
						variant="ghost"
						size="icon-xs"
						tabIndex={-1}
						aria-label={t(expanded ? "global.collapseFolder" : "global.expandFolder", { name })}
						disabled={disabled}
						onClick={(event) => {
							event.stopPropagation();
							event.currentTarget.closest<HTMLElement>('[role="treeitem"]')?.focus();
							setExpanded(path, !expanded);
						}}
					>
						{expanded ? <ChevronDown /> : <ChevronRight />}
					</Button>
				) : (
					<span className="size-5 shrink-0" aria-hidden="true" />
				)}
				{expanded ? <FolderOpen aria-hidden="true" /> : <Folder aria-hidden="true" />}
				<span className="min-w-0 flex-1 truncate" title={name}>
					{name}
				</span>
			</div>
			{expanded && (
				<ul role="group">
					{listing.isLoading && (
						<li
							role="none"
							className="flex items-center gap-2 px-3 py-2 text-xs text-muted-foreground"
						>
							<Spinner />
							<span role="status">{t("session.loading")}</span>
						</li>
					)}
					{listing.error && (
						<li role="none" className="p-1">
							<ErrorNotice
								action={
									<Button variant="outline" size="xs" onClick={() => void listing.mutate()}>
										{t("secrets.retry")}
									</Button>
								}
							>
								{message(listing.error)}
							</ErrorNotice>
						</li>
					)}
					{!listing.error &&
						folders?.map((child) => (
							<FolderBranch {...props} key={child} path={child} depth={depth + 1} />
						))}
					{!listing.error && folders?.length === 0 && path === "/" && (
						<li role="none" className="px-3 py-2 text-xs text-muted-foreground">
							{t("global.noFolders")}
						</li>
					)}
				</ul>
			)}
		</li>
	);
}
