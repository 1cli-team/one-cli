// Language controls share the existing auto / zh-CN / en-US preference.
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

export function LanguageSwitcher({ showLabel = false }: { showLabel?: boolean }) {
	const { mode } = useLocaleStore();
	const { t } = useTranslation();
	return (
		<DropdownMenu>
			<DropdownMenuTrigger asChild>
				<Button
					variant={showLabel ? "outline" : "ghost"}
					size={showLabel ? "default" : "icon"}
					title={t("sidebar.language")}
					aria-label={t("sidebar.language")}
				>
					<Languages className="size-4" aria-hidden="true" />
					{showLabel && t(OPTIONS.find((option) => option.mode === mode)!.labelKey)}
				</Button>
			</DropdownMenuTrigger>
			<DropdownMenuContent
				side="bottom"
				align="end"
				collisionPadding={12}
				sideOffset={8}
				className="w-40"
			>
				<LanguageMenuItems />
			</DropdownMenuContent>
		</DropdownMenu>
	);
}

function LanguageMenuItems() {
	const { mode, setMode } = useLocaleStore();
	const { t } = useTranslation();
	return (
		<DropdownMenuRadioGroup
			value={mode}
			onValueChange={(value) => setMode(value as LocaleMode)}
			aria-label={t("sidebar.language")}
			className="space-y-1"
		>
			{OPTIONS.map((option) => (
				<DropdownMenuRadioItem key={option.mode} value={option.mode}>
					{t(option.labelKey)}
				</DropdownMenuRadioItem>
			))}
		</DropdownMenuRadioGroup>
	);
}
