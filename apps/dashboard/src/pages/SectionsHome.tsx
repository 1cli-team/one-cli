// SectionsHome lists the (domain, backend) sections grouped by domain
// (env / deploy / container). Each section card surfaces its default
// profile and total count; drilling in goes to /section/{domain}/{backend}.

import { ChevronRight, ServerCog } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import useSWR from "swr";
import { configKey, getConfig } from "@/api/configure";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
	type Config,
	SECTION_DOMAINS,
	SECTION_KEYS_BY_DOMAIN,
	SECTION_META,
	type SectionKey,
} from "@/types/api";

function defaultFor(cfg: Config | undefined, key: SectionKey): { defaultName: string; count: number } {
	if (!cfg) return { defaultName: "", count: 0 };
	const sec = cfg[key];
	return {
		defaultName: sec?.default ?? "",
		count: sec?.profiles ? Object.keys(sec.profiles).length : 0,
	};
}

export const SectionsHome: React.FC = () => {
	const { t } = useTranslation();
	const { data, error, isLoading } = useSWR(configKey, () => getConfig(false));

	// Auth comes from EITHER ?token= on the initial URL OR a one_serve_token
	// cookie the Go landing handler drops on first GET /. The cookie is
	// HttpOnly, so JS can't tell whether it's present — gating the UI on
	// `hasToken()` (which only sees the URL token) blocks legitimate
	// cookie-auth navigations (e.g. refresh on /profile or /section/...).
	// We let the API request go through and surface any real 401 via the
	// error branch below.

	if (error) {
		return (
			<Alert variant="destructive">
				<AlertTitle>{t("home.loadFailedTitle")}</AlertTitle>
				<AlertDescription>
					{error.message}
					{error.code ? <span className="ml-2 text-xs opacity-75">[{error.code}]</span> : null}
				</AlertDescription>
			</Alert>
		);
	}

	return (
		<div className="space-y-8">
			<h1 className="text-xl font-semibold tracking-tight">{t("home.title")}</h1>

			{SECTION_DOMAINS.map((domain) => {
				const keys = SECTION_KEYS_BY_DOMAIN[domain];
				if (keys.length === 0) return null;
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
							{keys.map((key) => {
								const meta = SECTION_META[key];
								const { defaultName, count } = defaultFor(data?.config, key);
								const title = t(`sections.${meta.domain}.${meta.backend}.title`, {
									defaultValue: meta.title,
								});
								const description = t(`sections.${meta.domain}.${meta.backend}.description`, {
									defaultValue: meta.description,
								});
								return (
									<Link
										key={key}
										to={`/section/${meta.domain}/${meta.backend}`}
										className="group rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
									>
										<Card className="h-full transition-colors hover:border-primary/50">
											<CardHeader>
												<div className="flex items-start justify-between gap-2">
													<div className="space-y-1 min-w-0">
														<CardTitle className="flex items-center gap-2">
															<ServerCog className="h-4 w-4 text-primary shrink-0" />
															<span className="truncate">{title}</span>
														</CardTitle>
														<p className="text-[11px] font-mono text-muted-foreground">
															{meta.key}
														</p>
														<CardDescription>{description}</CardDescription>
													</div>
													<ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
												</div>
											</CardHeader>
											<CardContent className="flex items-center gap-2 text-xs">
												<Badge variant="secondary">{t("home.profileCount", { count })}</Badge>
												{defaultName ? (
													<span className="inline-flex items-center gap-1.5 text-muted-foreground">
														<span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
														<span className="text-foreground">
															{t("home.default", { name: defaultName })}
														</span>
													</span>
												) : (
													<span className="text-muted-foreground">{t("home.noDefault")}</span>
												)}
												{isLoading ? (
													<span className="text-muted-foreground">{t("home.loading")}</span>
												) : null}
											</CardContent>
										</Card>
									</Link>
								);
							})}
						</div>
					</section>
				);
			})}
		</div>
	);
};
