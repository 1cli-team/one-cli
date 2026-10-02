import type { ReactNode } from "react";
import type { Metadata } from "next";
import { SiteDocument } from "@/components/site-document";

export const metadata: Metadata = {
	title: "My Website / 我的网站",
	description: "Choose your language / 选择语言",
};
export default function Layout({ children }: { children: ReactNode }) {
	return <SiteDocument locale="en">{children}</SiteDocument>;
}
