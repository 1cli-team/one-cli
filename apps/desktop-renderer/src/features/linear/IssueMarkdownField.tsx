import { lazy, Suspense, type ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";

const IssueMarkdownEditor = lazy(() => import("./IssueMarkdownEditor"));

export function IssueMarkdownField(props: ComponentProps<typeof IssueMarkdownEditor>) {
	const { t } = useTranslation();
	return (
		<Suspense
			fallback={
				<div role="status" className="space-y-2" aria-label={t("linear.loading")}>
					<Label>{props.label}</Label>
					<Skeleton className={props.compact ? "h-28 w-full" : "h-40 w-full"} />
				</div>
			}
		>
			<IssueMarkdownEditor {...props} />
		</Suspense>
	);
}
