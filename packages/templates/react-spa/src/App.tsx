import { Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useThemeStore } from "@/lib/stores/theme";
import { AppRoutes } from "@/router/routes";

export function App() {
	const mode = useThemeStore((state) => state.mode);
	const toggle = useThemeStore((state) => state.toggle);
	return (
		<div className="min-h-screen bg-background text-foreground">
			<header className="mx-auto flex max-w-2xl items-center justify-between px-6 py-4">
				<span>One CLI · React</span>
				<Button onClick={toggle} variant="outline" size="icon" aria-label="切换主题 / Toggle theme">
					{mode === "light" ? <Moon /> : <Sun />}
				</Button>
			</header>
			<main className="mx-auto max-w-2xl px-6 py-16">
				<AppRoutes />
			</main>
		</div>
	);
}
