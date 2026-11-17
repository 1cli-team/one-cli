import { Bot, Copy } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import Toast from "@/lib/toast";

export function GatewayAgentConnect() {
	const { t } = useTranslation();
	const prompt = t("gateway.agent.prompt", {
		status: "one gateway status -o json",
		jobs: "one gateway jobs -o json",
	});

	async function copyPrompt() {
		try {
			await navigator.clipboard.writeText(prompt);
			Toast.success(t("gateway.agent.copied"));
		} catch {
			Toast.error(t("gateway.agent.copyError"));
		}
	}

	return (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="outline" size="sm">
					<Bot aria-hidden="true" />
					{t("gateway.agent.connect")}
				</Button>
			</DialogTrigger>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>{t("gateway.agent.connect")}</DialogTitle>
					<DialogDescription>{t("gateway.agent.description")}</DialogDescription>
				</DialogHeader>
				<pre className="select-text rounded-md border border-border bg-muted/50 p-4 font-mono text-sm whitespace-pre-wrap wrap-anywhere">
					{prompt}
				</pre>
				<DialogFooter>
					<Button onClick={() => void copyPrompt()}>
						<Copy aria-hidden="true" />
						{t("gateway.agent.copy")}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
