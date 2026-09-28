import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import {
	ArrowRight,
	BookOpen,
	Boxes,
	Braces,
	Check,
	Folder,
	Globe,
	Monitor,
	Package,
	Search,
	Server,
	Smartphone,
	type LucideIcon,
} from "lucide-react";
import { getProjectTemplates, projectTemplatesKey } from "@/api/project-creation";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorNotice, PageHeader, SearchInput, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";

const CATEGORIES = ["all", "frontend", "backend", "library"] as const;
const TEMPLATE_ICONS: Record<string, LucideIcon> = {
	"nestjs-api": Server,
	"go-api": Server,
	"astro-site": Globe,
	"starlight-docs": BookOpen,
	"nextjs-app": Globe,
	"react-spa": Braces,
	"expo-mobile": Smartphone,
	"electron-app": Monitor,
	"ts-library": Package,
	"go-lib": Package,
};

export function ProjectTemplates() {
	const { t } = useTranslation();
	const catalog = useSWR(projectTemplatesKey, getProjectTemplates);
	const [query, setQuery] = useState("");
	const [category, setCategory] = useState<(typeof CATEGORIES)[number]>("all");
	const templates = (catalog.data?.templates ?? []).map((template) => ({
		...template,
		name: t(`projectCreate.templates.${template.id}.name`, { defaultValue: template.name }),
		description: t(`templateCatalog.templates.${template.id}.description`, {
			defaultValue: template.description,
		}),
		stack: t(`templateCatalog.templates.${template.id}.stack`, {
			defaultValue: template.toolchain === "go" ? "Go" : "Node.js",
		}),
		features: ["feature1", "feature2", "feature3"]
			.map((key) => t(`templateCatalog.templates.${template.id}.${key}`, { defaultValue: "" }))
			.filter(Boolean),
	}));
	const search = query.trim().toLocaleLowerCase();
	const filtered = templates.filter(
		(template) =>
			(category === "all" || template.category === category) &&
			[
				template.name,
				template.id,
				template.description,
				template.stack,
				template.directory,
				template.toolchain === "go" ? "Go" : "Node.js",
				...template.features,
			]
				.join(" ")
				.toLocaleLowerCase()
				.includes(search),
	);
	function resetFilters() {
		setCategory("all");
		setQuery("");
	}

	return (
		<div className="mx-auto max-w-7xl space-y-6">
			<PageHeader
				title={t("templateCatalog.title")}
				description={t("templateCatalog.description")}
			/>
			<div className="flex flex-wrap items-center justify-between gap-4 rounded-lg border border-border bg-card p-4">
				<div className="flex min-w-0 items-start gap-3">
					<Boxes className="mt-0.5 size-5 shrink-0 text-primary-text" aria-hidden="true" />
					<div>
						<h2 className="text-sm font-medium">{t("templateCatalog.getStarted")}</h2>
						<p className="mt-1 text-sm text-muted-foreground">
							{t("templateCatalog.getStartedHint")}
						</p>
					</div>
				</div>
				<Button asChild variant="outline">
					<EnvironmentLink to="/">
						{t("templateCatalog.chooseWorkspace")}
						<ArrowRight aria-hidden="true" />
					</EnvironmentLink>
				</Button>
			</div>
			{catalog.isLoading ? (
				<div
					role="status"
					aria-label={t("projectCreate.loadingTemplates")}
					className="grid gap-4 ud-sm:grid-cols-2 ud-lg:grid-cols-3"
				>
					{Array.from({ length: 6 }, (_, index) => (
						<Skeleton key={index} className="h-80 rounded-lg" />
					))}
				</div>
			) : catalog.error ? (
				<ErrorNotice
					action={
						<Button variant="outline" onClick={() => void catalog.mutate().catch(() => undefined)}>
							{t("workspaces.retry")}
						</Button>
					}
				>
					{t("projectCreate.templatesFailed")}
				</ErrorNotice>
			) : templates.length === 0 ? (
				<StatePanel icon={Boxes} title={t("projectCreate.noTemplates")} />
			) : (
				<section aria-label={t("templateCatalog.title")} className="space-y-4">
					<div className="flex flex-wrap items-center justify-between gap-3">
						<div
							role="group"
							aria-label={t("templateCatalog.filter")}
							className="flex flex-wrap gap-1"
						>
							{CATEGORIES.map((value) => (
								<Button
									key={value}
									variant={category === value ? "secondary" : "ghost"}
									aria-pressed={category === value}
									onClick={() => setCategory(value)}
								>
									{t(value === "all" ? "templateCatalog.all" : `projectCreate.categories.${value}`)}
									<span className="text-xs tabular-nums text-muted-foreground">
										{value === "all"
											? templates.length
											: templates.filter((template) => template.category === value).length}
									</span>
								</Button>
							))}
						</div>
						<div className="w-full ud-sm:w-80">
							<SearchInput
								value={query}
								onChange={(event) => setQuery(event.target.value)}
								placeholder={t("templateCatalog.search")}
								aria-label={t("templateCatalog.search")}
							/>
						</div>
					</div>
					<p role="status" className="text-xs text-muted-foreground">
						{t("templateCatalog.resultCount", { count: filtered.length, total: templates.length })}
					</p>
					{filtered.length === 0 ? (
						<StatePanel
							icon={Search}
							title={t("templateCatalog.noResults")}
							description={t("templateCatalog.noResultsHint")}
						>
							<Button variant="outline" onClick={resetFilters}>
								{t("templateCatalog.reset")}
							</Button>
						</StatePanel>
					) : (
						<div className="grid gap-4 ud-sm:grid-cols-2 ud-lg:grid-cols-3">
							{filtered.map((template) => {
								const Icon = TEMPLATE_ICONS[template.id] ?? Boxes;
								return (
									<article
										key={template.id}
										aria-labelledby={`template-${template.id}`}
										className="flex min-w-0 flex-col rounded-lg border border-border bg-card p-5"
									>
										<div className="flex items-start gap-3">
											<span className="grid size-10 shrink-0 place-items-center rounded-lg bg-muted text-foreground">
												<Icon className="size-5" aria-hidden="true" />
											</span>
											<div className="min-w-0 flex-1">
												<h2 id={`template-${template.id}`} className="text-base font-semibold">
													{template.name}
												</h2>
												<p className="mt-0.5 break-all font-mono text-xs text-muted-foreground">
													{template.id}
												</p>
											</div>
										</div>
										<div className="mt-4">
											<Badge variant="muted">
												{t(`projectCreate.categories.${template.category}`)}
											</Badge>
										</div>
										<p className="mt-3 text-sm leading-relaxed text-muted-foreground">
											{template.description}
										</p>
										<p className="mt-4 text-xs font-medium leading-relaxed">{template.stack}</p>
										{template.features.length > 0 && (
											<ul
												className="mt-3 mb-5 space-y-2"
												aria-label={t("templateCatalog.includes")}
											>
												{template.features.map((feature) => (
													<li
														key={feature}
														className="flex gap-2 text-xs leading-relaxed text-muted-foreground"
													>
														<Check className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
														<span>{feature}</span>
													</li>
												))}
											</ul>
										)}
										<div className="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3 text-xs text-muted-foreground">
											<span
												className="inline-flex items-center gap-1.5"
												title={t("projectCreate.location")}
											>
												<Folder className="size-3.5" aria-hidden="true" />
												<span className="font-mono">{template.directory}/</span>
											</span>
											<span>{template.toolchain === "go" ? "Go" : "Node.js"}</span>
										</div>
									</article>
								);
							})}
						</div>
					)}
				</section>
			)}
		</div>
	);
}
