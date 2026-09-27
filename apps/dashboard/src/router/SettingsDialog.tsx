import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { AccountSettings } from "@/features/infisical-session/AccountSettings";
export function SettingsDialog() {
	const { t } = useTranslation();
	return (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="outline" size="sm">
					{t("sidebar.settings")}
				</Button>
			</DialogTrigger>
			<DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
				<DialogHeader>
					<DialogTitle>{t("sidebar.settings")}</DialogTitle>
					<DialogDescription>{t("session.description")}</DialogDescription>
				</DialogHeader>
				<AccountSettings />
			</DialogContent>
		</Dialog>
	);
}
