import {
	Cloud,
	Container,
	Home,
	KeyRound,
	MoonStar,
	SlidersHorizontal,
	SunMedium,
} from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";
import { BACKEND_DOMAINS, humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { Button } from "@/components/ui/button";
import { useThemeStore } from "@/lib/stores/theme";
import { cn } from "@/lib/utils";
import type { BackendDomain } from "@/types/api";

const DOMAIN_ICONS: Record<BackendDomain, React.ComponentType<{ className?: string }>> = {
	env: KeyRound,
	deploy: Cloud,
	container: Container,
};

const navItemClass = ({ isActive }: { isActive: boolean }) =>
	cn(
		"flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors",
		isActive
			? "bg-accent text-accent-foreground font-medium"
			: "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
	);

export const AppSidebar: React.FC = () => {
	const { mode, toggle } = useThemeStore();
	const { t } = useTranslation();
	const catalog = useBackendCatalog();
	const logoSrc = mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg";

	return (
		<aside className="flex h-screen w-60 shrink-0 flex-col border-r border-border bg-card/40">
			<div className="flex items-center gap-2.5 px-4 py-4">
				<img src={logoSrc} alt="One CLI" className="h-7 w-7" />
				<p className="truncate text-[11px] font-semibold tracking-[0.22em] text-muted-foreground uppercase">
					{t("sidebar.brand")}
				</p>
			</div>

			<nav className="flex-1 overflow-y-auto px-2 pb-3">
				<NavLink to="/" end className={navItemClass}>
					<Home className="h-4 w-4" />
					<span>{t("sidebar.home")}</span>
				</NavLink>
				<NavLink to="/profile" className={navItemClass}>
					<SlidersHorizontal className="h-4 w-4" />
					<span>{t("sidebar.profile")}</span>
				</NavLink>

				{BACKEND_DOMAINS.map((domain) => {
					const backends = (catalog.byDomain.get(domain) ?? []).filter(
						(backend) => backend.profile.configurable,
					);
					if (backends.length === 0) return null;
					const groupLabel = t(`sections.groupLabel.${domain}`, {
						defaultValue: domain,
					});
					return (
						<div key={domain} className="mt-4">
							<p className="mb-1 px-2.5 text-[10px] font-semibold tracking-[0.18em] text-muted-foreground uppercase">
								{groupLabel}
							</p>
							{backends.map((backend) => {
								const Icon = DOMAIN_ICONS[backend.domain];
								const title = t(`sections.${backend.domain}.${backend.name}.title`, {
									defaultValue: humanizeBackendName(backend.name),
								});
								return (
									<NavLink
										key={backend.id}
										to={`/section/${backend.domain}/${backend.name}`}
										className={navItemClass}
									>
										<Icon className="h-4 w-4" />
										<span className="truncate">{title}</span>
										<span className="ml-auto truncate text-[10px] text-muted-foreground/70">
											{backend.name}
										</span>
									</NavLink>
								);
							})}
						</div>
					);
				})}
			</nav>

			<div className="flex items-center justify-between border-t border-border px-3 py-3">
				<div className="flex items-center gap-1.5 text-xs text-muted-foreground">
					<span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
					<span>{t("sidebar.loopbackOnly")}</span>
				</div>
				<div className="flex items-center gap-1">
					<Button
						onClick={toggle}
						variant="ghost"
						size="icon"
						className="h-7 w-7"
						title={mode === "light" ? t("sidebar.themeToDark") : t("sidebar.themeToLight")}
					>
						{mode === "light" ? (
							<MoonStar className="h-4 w-4" />
						) : (
							<SunMedium className="h-4 w-4" />
						)}
					</Button>
				</div>
			</div>
		</aside>
	);
};
