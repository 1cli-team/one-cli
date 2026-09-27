import { useTranslation } from "react-i18next";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
export function DiscardDialog({
	open,
	onOpenChange,
	onDiscard,
}: {
	open: boolean;
	onOpenChange(open: boolean): void;
	onDiscard(): void;
}) {
	const { t } = useTranslation();
	return (
		<AlertDialog open={open} onOpenChange={onOpenChange}>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>{t("form.discardTitle")}</AlertDialogTitle>
					<AlertDialogDescription>{t("form.discardDescription")}</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>{t("form.continueEditing")}</AlertDialogCancel>
					<AlertDialogAction variant="destructive" onClick={onDiscard}>
						{t("global.discard")}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
