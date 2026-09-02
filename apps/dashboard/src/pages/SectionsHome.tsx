// SectionsHome is the machine-level Settings surface. It lists configurable
// profile backends by domain; profile data is fetched only after drilling in.

import { ChevronRight, ServerCog } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { BACKEND_DOMAINS, humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { cn } from "@/lib/utils";

export const SectionsHome: React.FC<{
	embedded?: boolean;
	onSelect?: (domain: string, backend: string) => void;
}> = ({ embedded = false, onSelect }) => {
	const { t } = useTranslation();
	const catalog = useBackendCatalog();

	if (catalog.error) {
		return (
			<Alert variant="destructive">
				<AlertTitle>{t("settings.loadFailedTitle")}</AlertTitle>
				<AlertDescription>{catalog.error.message}</AlertDescription>
			</Alert>
		);
	}
	if (catalog.isLoading) {
		return (
			<div role="status" className="space-y-5">
				<span className="sr-only">{t("detail.loading")}</span>
				<Skeleton className="h-14 w-80" />
				<div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
					<Skeleton className="h-20 w-full" />
					<Skeleton className="h-20 w-full opacity-80" />
					<Skeleton className="h-20 w-full opacity-60" />
				</div>
			</div>
		);
	}

	return (
		<div className={cn(embedded ? "space-y-4" : "space-y-7")}>
			{embedded ? null : (
				<header className="pb-1">
					<h1 className="text-3xl font-semibold tracking-tight">{t("settings.title")}</h1>
					<p className="mt-2 max-w-2xl text-sm text-muted-foreground">
						{t("settings.description")}
					</p>
				</header>
			)}

			{BACKEND_DOMAINS.map((domain) => {
				const backends = (catalog.byDomain.get(domain) ?? []).filter(
					(backend) => backend.profile.configurable,
				);
				if (backends.length === 0) return null;
				const groupLabel = t(`sections.groupLabel.${domain}`, { defaultValue: domain });
				return (
					<section
						key={domain}
						className="overflow-hidden rounded-xl border border-border bg-card shadow-sm"
					>
						<div className="flex items-center justify-between border-b border-border bg-muted/45 px-4 py-3">
							<h2 className="font-mono text-[10px] font-semibold tracking-[0.12em] uppercase text-muted-foreground">
								{groupLabel}
							</h2>
							<span className="font-mono text-[10px] text-muted-foreground/70">{domain}</span>
						</div>
						<div className="divide-y divide-border">
							{backends.map((backend) => {
								const title = t(`sections.${backend.domain}.${backend.name}.title`, {
									defaultValue: humanizeBackendName(backend.name),
								});
								const content = (
									<>
										<span className="grid size-9 shrink-0 place-items-center rounded-lg bg-muted text-primary">
											<ServerCog className="size-4" />
										</span>
										<span className="min-w-0 flex-1">
											<span className="flex items-center gap-2">
												<span className="truncate text-base font-medium">{title}</span>
												<span className="font-mono text-[10px] text-muted-foreground">
													{backend.name}
												</span>
											</span>
											<span className="mt-1 block text-xs text-muted-foreground">
												{t(`sections.${backend.domain}.${backend.name}.description`, {
													defaultValue: backend.id,
												})}
											</span>
										</span>
										<span className="inline-flex items-center gap-1 text-xs font-medium text-primary">
											{t("settings.manageBackend")}
											<ChevronRight className="size-3.5 transition-transform group-hover:translate-x-0.5" />
										</span>
									</>
								);
								const itemClass =
									"group flex min-h-[72px] w-full items-center justify-start gap-4 rounded-none px-4 py-3 text-left font-normal whitespace-normal transition-colors hover:bg-accent/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring";
								return onSelect ? (
									<Button
										type="button"
										key={backend.id}
										variant="ghost"
										className={itemClass}
										onClick={() => onSelect(backend.domain, backend.name)}
									>
										{content}
									</Button>
								) : (
									<EnvironmentLink
										key={backend.id}
										to={`/settings/${backend.domain}/${backend.name}`}
										className={itemClass}
									>
										{content}
									</EnvironmentLink>
								);
							})}
						</div>
					</section>
				);
			})}
		</div>
	);
};
