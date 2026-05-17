import { ChevronRight } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Link, useMatch } from "react-router-dom";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { SECTION_META, type SectionKey } from "@/types/api";

export const TopBar: React.FC = () => {
	const sectionMatch = useMatch("/section/:domain/:backend");
	const profileMatch = useMatch("/profile");

	return (
		<header className="flex h-12 shrink-0 items-center justify-between gap-4 border-b border-border bg-background px-6">
			<nav className="flex items-center gap-1.5 text-sm">
				{sectionMatch ? (
					<SectionCrumb match={sectionMatch.params} />
				) : profileMatch ? (
					<ProfileCrumb />
				) : (
					<HomeCrumb />
				)}
			</nav>
			<LanguageSwitcher />
		</header>
	);
};

const HomeCrumb: React.FC = () => {
	const { t } = useTranslation();
	return <span className="font-medium text-foreground">{t("topbar.home")}</span>;
};

const ProfileCrumb: React.FC = () => {
	const { t } = useTranslation();
	return <span className="font-medium text-foreground">{t("topbar.profile")}</span>;
};

const SectionCrumb: React.FC<{ match: { domain?: string; backend?: string } }> = ({ match }) => {
	const { t } = useTranslation();
	const key = `${match.domain ?? ""}/${match.backend ?? ""}` as SectionKey;
	const meta = SECTION_META[key];
	const title = meta
		? t(`sections.${meta.domain}.${meta.backend}.title`, { defaultValue: meta.title })
		: key;
	return (
		<>
			<Link to="/" className="text-muted-foreground hover:text-foreground transition-colors">
				{t("topbar.sectionsRoot")}
			</Link>
			<ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
			<span className="font-medium text-foreground">{title}</span>
			{meta ? (
				<span className="ml-1.5 text-xs font-normal text-muted-foreground">{meta.key}</span>
			) : null}
		</>
	);
};
