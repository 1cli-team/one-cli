import { useEffect } from "react";
import { Outlet } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { useAppStore } from "@/store/app-store";

export default function App() {
  const theme = useAppStore((state) => state.theme);
  const toggleTheme = useAppStore((state) => state.toggleTheme);
  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark");
    document.documentElement.style.colorScheme = theme;
  }, [theme]);
  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="mx-auto flex max-w-2xl items-center justify-between px-6 py-4">
        <span>One CLI · Electron</span>
        <Button variant="outline" onClick={toggleTheme}>
          切换主题 / Toggle theme
        </Button>
      </header>
      <main className="mx-auto max-w-2xl px-6 py-16">
        <Outlet />
      </main>
    </div>
  );
}
