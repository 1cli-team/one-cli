"use client";

import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { messages, type Locale } from "@/lib/i18n";

export function ThemeToggle({ locale }: { locale: Locale }) {
	const { setTheme } = useTheme();
	const text = messages[locale];
	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				render={<Button variant="outline" size="icon" aria-label={text.theme} />}
			>
				<Sun className="size-4 dark:hidden" />
				<Moon className="hidden size-4 dark:block" />
			</DropdownMenuTrigger>
			<DropdownMenuContent align="end">
				<DropdownMenuItem onClick={() => setTheme("light")}>{text.light}</DropdownMenuItem>
				<DropdownMenuItem onClick={() => setTheme("dark")}>{text.dark}</DropdownMenuItem>
				<DropdownMenuItem onClick={() => setTheme("system")}>{text.system}</DropdownMenuItem>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
