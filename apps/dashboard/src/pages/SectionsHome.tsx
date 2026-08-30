// SectionsHome is the machine-level Settings surface. It lists configurable
// profile backends by domain; profile data is fetched only after drilling in.

import { ChevronRight, ServerCog } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { BACKEND_DOMAINS, humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";

export const SectionsHome: React.FC = () => {
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
					<Skeleton className="h-20 w-full rounded-lg" />
					<Skeleton className="h-20 w-full rounded-lg opacity-80" />
					<Skeleton className="h-20 w-full rounded-lg opacity-60" />
				</div>
			</div>
		);
	}

	return (
		<div className="space-y-8">
			<header className="space-y-1">
				<h1 className="text-xl font-semibold tracking-tight">{t("settings.title")}</h1>
				<p className="max-w-2xl text-sm text-muted-foreground">{t("settings.description")}</p>
			</header>

			{BACKEND_DOMAINS.map((domain) => {
				const backends = (catalog.byDomain.get(domain) ?? []).filter(
					(backend) => backend.profile.configurable,
				);
				if (backends.length === 0) return null;
				const groupLabel = t(`sections.groupLabel.${domain}`, { defaultValue: domain });
				return (
					<section key={domain} className="space-y-3">
						<div className="flex items-baseline gap-2 border-b border-border/60 pb-1.5">
							<h2 className="text-sm font-semibold tracking-wide uppercase text-muted-foreground">
								{groupLabel}
							</h2>
							<span className="text-[11px] text-muted-foreground/60 font-mono">{domain}</span>
						</div>
						<div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
							{backends.map((backend) => {
								const title = t(`sections.${backend.domain}.${backend.name}.title`, {
									defaultValue: humanizeBackendName(backend.name),
								});
								return (
									<EnvironmentLink
										key={backend.id}
										to={`/settings/${backend.domain}/${backend.name}`}
										className="group rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
									>
										<Card className="h-full transition-colors hover:border-primary/50">
											<CardHeader className="py-4">
												<div className="flex items-start justify-between gap-2">
													<div className="min-w-0">
														<CardTitle className="flex items-center gap-2">
															<ServerCog className="h-4 w-4 text-primary shrink-0" />
															<span className="truncate">{title}</span>
														</CardTitle>
													</div>
													<ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
												</div>
											</CardHeader>
										</Card>
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
