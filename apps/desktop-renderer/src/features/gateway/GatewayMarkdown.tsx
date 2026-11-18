import { lazy, Suspense } from "react";
import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";
import { openExternal } from "@/lib/desktop/runtime";
import i18n from "@/lib/i18n";
import Toast from "@/lib/toast";

const MarkdownPreview = lazy(() =>
	import("@/features/linear/IssueMarkdownEditor").then(({ IssueMarkdownPreview }) => ({
		default: IssueMarkdownPreview,
	})),
);

export function openGatewayLink(url: string) {
	void openExternal(url).catch(() => Toast.error(i18n.t("gateway.taskList.linkError")));
}

export function GatewayMarkdown({ value }: { value: string }) {
	return (
		<Suspense fallback={<p className="whitespace-pre-wrap wrap-anywhere">{value}</p>}>
			<MarkdownPreview value={value} onOpenLink={openGatewayLink} />
		</Suspense>
	);
}

export function CodexResultMessage({ value, isLoading }: { value?: string; isLoading?: boolean }) {
	const { t } = useTranslation();
	return (
		<section className="flex flex-col gap-2 rounded-md border border-border p-3 text-sm">
			<h3 className="font-medium">{t("gateway.taskList.codexResult")}</h3>
			{isLoading ? (
				<Skeleton aria-busy="true" className="h-12 w-full" />
			) : value ? (
				<GatewayMarkdown value={value} />
			) : (
				<p className="text-muted-foreground">{t("gateway.taskList.codexResultUnavailable")}</p>
			)}
		</section>
	);
}
