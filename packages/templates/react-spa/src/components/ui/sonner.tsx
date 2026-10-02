"use client";

import { useThemeStore } from "@/lib/stores/theme";
import { Toaster as Sonner, type ToasterProps } from "sonner";

import {
	CircleCheckIcon,
	InfoIcon,
	Loader2Icon,
	OctagonXIcon,
	TriangleAlertIcon,
} from "lucide-react";

const Toaster = ({ ...props }: ToasterProps) => {
	const { mode: theme } = useThemeStore();

	return (
		<Sonner
			theme={theme as ToasterProps["theme"]}
			className="toaster group"
			closeButton
			richColors
			visibleToasts={5}
			icons={{
				success: <CircleCheckIcon aria-hidden="true" className="size-4" />,
				info: <InfoIcon aria-hidden="true" className="size-4" />,
				warning: <TriangleAlertIcon aria-hidden="true" className="size-4" />,
				error: <OctagonXIcon aria-hidden="true" className="size-4" />,
				loading: <Loader2Icon aria-hidden="true" className="size-4 animate-spin" />,
			}}
			style={
				{
					"--normal-bg": "var(--popover)",
					"--normal-text": "var(--popover-foreground)",
					"--normal-border": "var(--border)",
					"--border-radius": "var(--radius)",
				} as React.CSSProperties
			}
			toastOptions={{
				classNames: {
					toast: "cn-toast",
				},
			}}
			{...props}
		/>
	);
};

export { Toaster };
