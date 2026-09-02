// LanguageSwitcher: globe-icon dropdown with three options
// (auto / zh-CN / en-US). Mirrors the theme toggle's footprint in
// the sidebar — same Button size, same icon-only collapsed state.
//
// Uses Radix DropdownMenu so the menu inherits the design system's
// focus/keyboard behaviour for free.

import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { useLocaleStore, type LocaleMode } from "@/lib/stores/locale";
import { Languages } from "lucide-react";
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

const MENU_WIDTH = 160;
const TRIGGER_WIDTH = 28;
const CENTERED_END_OFFSET = -(MENU_WIDTH - TRIGGER_WIDTH) / 2;

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
			<DropdownMenuContent
				side="top"
				align="end"
				alignOffset={CENTERED_END_OFFSET}
				sideOffset={8}
				className="w-40"
			>
				<DropdownMenuRadioGroup
					value={mode}
					onValueChange={(value) => setMode(value as LocaleMode)}
				>
					{OPTIONS.map((option) => (
						<DropdownMenuRadioItem key={option.mode} value={option.mode}>
							{t(option.labelKey)}
						</DropdownMenuRadioItem>
					))}
				</DropdownMenuRadioGroup>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
