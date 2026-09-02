import { ArrowLeft, Settings2 } from "lucide-react";
import type React from "react";
import { useState } from "react";
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
import { SectionDetailContent } from "@/pages/SectionDetail";
import { SectionsHome } from "@/pages/SectionsHome";

interface SelectedBackend {
	domain: string;
	backend: string;
}

export const SettingsDialog: React.FC = () => {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const [selected, setSelected] = useState<SelectedBackend | null>(null);

	return (
		<Dialog
			open={open}
			onOpenChange={(next) => {
				setOpen(next);
				if (!next) setSelected(null);
			}}
		>
			<DialogTrigger asChild>
				<Button variant="outline" size="sm">
					<Settings2 className="size-4" />
					{t("sidebar.settings")}
				</Button>
			</DialogTrigger>
			<DialogContent className="flex h-[min(760px,calc(100dvh-2rem))] max-w-[min(1080px,calc(100vw-2rem))] flex-col gap-0 overflow-hidden p-0 sm:max-w-[min(1080px,calc(100vw-2rem))]">
				<DialogHeader className="shrink-0 border-b border-border px-4 py-3 pr-12">
					<div className="flex items-center gap-2">
						{selected ? (
							<Button
								type="button"
								variant="ghost"
								size="icon-sm"
								aria-label={t("workspaces.unknown.back")}
								onClick={() => setSelected(null)}
							>
								<ArrowLeft />
							</Button>
						) : (
							<span className="grid size-8 place-items-center rounded-[5px] bg-primary/8 text-primary">
								<Settings2 className="size-4" />
							</span>
						)}
						<div>
							<DialogTitle>{t("settings.title")}</DialogTitle>
							<DialogDescription className="mt-1">{t("settings.description")}</DialogDescription>
						</div>
					</div>
				</DialogHeader>
				<div className="min-h-0 flex-1 overflow-y-auto p-4">
					{selected ? (
						<SectionDetailContent
							key={`${selected.domain}/${selected.backend}`}
							domain={selected.domain}
							backendName={selected.backend}
							embedded
						/>
					) : (
						<SectionsHome
							embedded
							onSelect={(domain, backend) => setSelected({ domain, backend })}
						/>
					)}
				</div>
			</DialogContent>
		</Dialog>
	);
};
