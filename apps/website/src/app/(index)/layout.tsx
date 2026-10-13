import type { ReactNode } from "react";
import { SiteDocument, siteMetadata } from "@/components/site-document";

export const metadata = siteMetadata;

export default function Layout({ children }: { children: ReactNode }) {
	return <SiteDocument locale="zh">{children}</SiteDocument>;
}
