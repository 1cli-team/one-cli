import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import type { GatewaySubtask } from "./api";
import { CodexResultMessage } from "./GatewayMarkdown";

export function GatewaySubtasks({ subtasks }: { subtasks?: GatewaySubtask[] }) {
	const { t } = useTranslation();
	if (!subtasks?.length) return null;
	return (
		<section className="space-y-2">
			<h3 className="font-medium">{t("linear.subtasks")}</h3>
			<ul className="divide-y divide-border rounded-md border border-border">
				{subtasks.map((task) => (
					<li key={task.identifier} className="space-y-2 p-3">
						<div className="flex flex-wrap items-center justify-between gap-2">
							<p className="min-w-0 wrap-anywhere">
								<span className="mr-2 font-mono text-xs text-muted-foreground">
									{task.identifier}
								</span>
								{task.title}
							</p>
							<Badge variant="secondary">
								{t(`gateway.taskList.stages.${task.stage}`, { defaultValue: task.stage })}
							</Badge>
						</div>
						{task.commitSha && (
							<code className="block wrap-anywhere text-xs text-muted-foreground">
								{task.commitSha}
							</code>
						)}
						{task.response && <CodexResultMessage value={task.response} />}
					</li>
				))}
			</ul>
		</section>
	);
}
