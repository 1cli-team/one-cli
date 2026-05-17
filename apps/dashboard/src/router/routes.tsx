import type React from "react";
import { type RouteObject, useRoutes } from "react-router-dom";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { getOverview, overviewKey } from "@/api/workspace";
import { Overview } from "@/pages/Overview";
import { SectionsHome } from "@/pages/SectionsHome";
import { SectionDetail } from "@/pages/SectionDetail";

const NotFoundRoute: React.FC = () => {
	const { t } = useTranslation();

	return <div className="text-sm text-muted-foreground">{t("notFound.message")}</div>;
};

// HomeRoute renders the workspace Overview when `one serve` was launched
// inside a One workspace; otherwise it falls back to SectionsHome (the
// profile-editor grid) so non-workspace use is unchanged. The overview
// endpoint returns 200 {present: false} for the non-workspace case, so the
// fallback path is reached without an error.
const HomeRoute: React.FC = () => {
	const { data, error } = useSWR(overviewKey, getOverview);
	if (error || !data) return <SectionsHome />;
	if (data.present) return <Overview data={data} />;
	return <SectionsHome />;
};

const routes: RouteObject[] = [
	{ path: "/", element: <HomeRoute /> },
	{ path: "/profile", element: <SectionsHome /> },
	{ path: "/section/:domain/:backend", element: <SectionDetail /> },
	{ path: "*", element: <NotFoundRoute /> },
];

export const AppRoutes: React.FC = () => useRoutes(routes);
