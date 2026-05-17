// LanguageSwitcher: globe-icon dropdown with three options
// (auto / zh-CN / en-US). Mirrors the theme toggle's footprint in
// the sidebar — same Button size, same icon-only collapsed state.
//
// Uses Radix DropdownMenu so the menu inherits the design system's
// focus/keyboard behaviour for free.

import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { useLocaleStore, type LocaleMode } from "@/lib/stores/locale";
import { Check, Languages } from "lucide-react";
import { useTranslation } from "react-i18next";

interface Option {
	mode: LocaleMode;
	labelKey: string;
}

const OPTIONS: Option[] = [
	{ mode: "auto", labelKey: "sidebar.languageAuto" },
	{ mode: "zh-CN", labelKey: "sidebar.languageZh" },
	{ mode: "en-US", labelKey: "sidebar.languageEn" },
];

export function LanguageSwitcher() {
	const { mode, setMode } = useLocaleStore();
	const { t } = useTranslation();
	return (
		<DropdownMenu>
			<DropdownMenuTrigger asChild>
				<Button
					variant="ghost"
					size="icon"
					className="h-7 w-7"
					title={t("sidebar.language")}
					aria-label={t("sidebar.language")}
				>
					<Languages className="h-4 w-4" />
				</Button>
			</DropdownMenuTrigger>
			<DropdownMenuContent align="end" sideOffset={6}>
				{OPTIONS.map((opt) => (
					<DropdownMenuItem
						key={opt.mode}
						onSelect={() => setMode(opt.mode)}
						className="gap-2"
					>
						<Check
							className={`h-3.5 w-3.5 ${mode === opt.mode ? "opacity-100" : "opacity-0"}`}
						/>
						<span>{t(opt.labelKey)}</span>
					</DropdownMenuItem>
				))}
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
