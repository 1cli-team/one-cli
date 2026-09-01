import { XIcon } from "lucide-react";
import { Dialog as SheetPrimitive } from "radix-ui";
import * as React from "react";
import { cn } from "@/lib/utils";

function Sheet({ ...props }: React.ComponentProps<typeof SheetPrimitive.Root>) {
	return <SheetPrimitive.Root data-slot="sheet" {...props} />;
}

function SheetTrigger({ ...props }: React.ComponentProps<typeof SheetPrimitive.Trigger>) {
	return <SheetPrimitive.Trigger data-slot="sheet-trigger" {...props} />;
}

function SheetClose({ ...props }: React.ComponentProps<typeof SheetPrimitive.Close>) {
	return <SheetPrimitive.Close data-slot="sheet-close" {...props} />;
}

function SheetPortal({ ...props }: React.ComponentProps<typeof SheetPrimitive.Portal>) {
	return <SheetPrimitive.Portal data-slot="sheet-portal" {...props} />;
}

function SheetOverlay({
	className,
	...props
}: React.ComponentProps<typeof SheetPrimitive.Overlay>) {
	return (
		<SheetPrimitive.Overlay
			data-slot="sheet-overlay"
			className={cn(
				"fixed inset-0 z-50 bg-slate-950/45 backdrop-blur-[1px] data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0",
				className,
			)}
			{...props}
		/>
	);
}

function SheetContent({
	className,
	children,
	side = "right",
	showCloseButton = true,
	closeLabel = "Close",
	showOverlay = true,
	overlayClassName,
	onOpenAutoFocus,
	onCloseAutoFocus,
	...props
}: React.ComponentProps<typeof SheetPrimitive.Content> & {
	side?: "top" | "right" | "bottom" | "left";
	showCloseButton?: boolean;
	closeLabel?: string;
	showOverlay?: boolean;
	overlayClassName?: string;
}) {
	const returnFocusRef = React.useRef<HTMLElement | null>(null);

	return (
		<SheetPortal>
			{showOverlay ? <SheetOverlay className={overlayClassName} /> : null}
			<SheetPrimitive.Content
				data-slot="sheet-content"
				onOpenAutoFocus={(event) => {
					returnFocusRef.current =
						typeof document !== "undefined" && document.activeElement instanceof HTMLElement
							? document.activeElement
							: null;
					onOpenAutoFocus?.(event);
				}}
				onCloseAutoFocus={(event) => {
					onCloseAutoFocus?.(event);
					const returnFocus = returnFocusRef.current;
					returnFocusRef.current = null;
					if (!event.defaultPrevented && returnFocus?.isConnected) {
						event.preventDefault();
						returnFocus.focus();
					}
				}}
				className={cn(
					"fixed z-50 flex flex-col gap-4 bg-popover text-popover-foreground shadow-lg outline-none transition ease-in-out data-[state=closed]:animate-out data-[state=closed]:duration-300 data-[state=open]:animate-in data-[state=open]:duration-500",
					side === "right" &&
						"inset-y-0 right-0 h-full w-[560px] max-w-[calc(100vw-2rem)] border-l border-border shadow-[-20px_0_60px_-28px_rgb(15_23_42_/_0.45)] data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right",
					side === "left" &&
						"inset-y-0 left-0 h-full w-[560px] max-w-[calc(100vw-2rem)] border-r border-border data-[state=closed]:slide-out-to-left data-[state=open]:slide-in-from-left",
					side === "top" &&
						"inset-x-0 top-0 h-auto border-b border-border data-[state=closed]:slide-out-to-top data-[state=open]:slide-in-from-top",
					side === "bottom" &&
						"inset-x-0 bottom-0 h-auto border-t border-border data-[state=closed]:slide-out-to-bottom data-[state=open]:slide-in-from-bottom",
					className,
				)}
				{...props}
			>
				{children}
				{showCloseButton && (
					<SheetPrimitive.Close className="absolute top-5 right-5 grid size-8 place-items-center rounded-md text-muted-foreground opacity-70 ring-offset-background transition-colors hover:bg-accent hover:text-foreground hover:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none data-[state=open]:bg-secondary">
						<XIcon className="size-4" />
						<span className="sr-only">{closeLabel}</span>
					</SheetPrimitive.Close>
				)}
			</SheetPrimitive.Content>
		</SheetPortal>
	);
}

function SheetHeader({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="sheet-header"
			className={cn("flex flex-col gap-1.5 p-4", className)}
			{...props}
		/>
	);
}

function SheetFooter({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="sheet-footer"
			className={cn("mt-auto flex flex-col gap-2 p-4", className)}
			{...props}
		/>
	);
}

function SheetTitle({ className, ...props }: React.ComponentProps<typeof SheetPrimitive.Title>) {
	return (
		<SheetPrimitive.Title
			data-slot="sheet-title"
			className={cn("text-lg font-semibold tracking-tight text-foreground", className)}
			{...props}
		/>
	);
}

function SheetDescription({
	className,
	...props
}: React.ComponentProps<typeof SheetPrimitive.Description>) {
	return (
		<SheetPrimitive.Description
			data-slot="sheet-description"
			className={cn("text-xs text-muted-foreground", className)}
			{...props}
		/>
	);
}

export {
	Sheet,
	SheetTrigger,
	SheetClose,
	SheetContent,
	SheetHeader,
	SheetFooter,
	SheetTitle,
	SheetDescription,
};
