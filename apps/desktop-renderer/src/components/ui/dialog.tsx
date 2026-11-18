import { useTranslation } from "react-i18next";
import { XIcon } from "lucide-react";
import { Dialog as DialogPrimitive } from "radix-ui";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

function Dialog({ ...props }: React.ComponentProps<typeof DialogPrimitive.Root>) {
	return <DialogPrimitive.Root data-slot="dialog" {...props} />;
}

function DialogTrigger({ ...props }: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
	return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />;
}

function DialogPortal({ ...props }: React.ComponentProps<typeof DialogPrimitive.Portal>) {
	return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />;
}

function DialogClose({ ...props }: React.ComponentProps<typeof DialogPrimitive.Close>) {
	return <DialogPrimitive.Close data-slot="dialog-close" {...props} />;
}

function DialogOverlay({
	className,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
	return (
		<DialogPrimitive.Overlay
			data-slot="dialog-overlay"
			className={cn(
				"fixed inset-0 z-50 bg-black/35 duration-200 ease-ui dark:bg-black/55 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0",
				className,
			)}
			{...props}
		/>
	);
}

function DialogContent({
	className,
	children,
	placement = "center",
	showCloseButton = true,
	onOpenAutoFocus,
	onCloseAutoFocus,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
	placement?: "center" | "right";
	showCloseButton?: boolean;
}) {
	const { t } = useTranslation();
	const returnFocusRef = React.useRef<HTMLElement | null>(null);

	return (
		<DialogPortal>
			<DialogOverlay
				className={
					placement === "right"
						? "duration-200 motion-reduce:data-[state=closed]:animate-none motion-reduce:data-[state=open]:animate-none"
						: undefined
				}
			/>
			<DialogPrimitive.Content
				data-slot="dialog-content"
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
					"fixed z-50 grid w-full gap-4 overflow-y-auto border border-border bg-card p-6 shadow-lg duration-200 ease-ui outline-none",
					placement === "right"
						? "inset-y-0 right-0 content-start rounded-none data-[state=closed]:slide-out-to-right data-[state=closed]:ease-in data-[state=open]:slide-in-from-right data-[state=open]:ease-out motion-safe:data-[state=closed]:animate-out motion-safe:data-[state=open]:animate-in"
						: "top-1/2 left-1/2 max-h-[calc(100dvh-4rem)] max-w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-xl data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0 ud-sm:max-w-[37.5rem]",
					className,
				)}
				{...props}
			>
				{children}
				{showCloseButton && (
					<DialogPrimitive.Close
						data-slot="dialog-close"
						className="absolute top-4 right-4 grid size-7 place-items-center rounded-md text-muted-foreground opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none data-[state=open]:bg-accent data-[state=open]:text-muted-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
					>
						<XIcon aria-hidden="true" />
						<span className="sr-only">{t("form.close", { defaultValue: "Close" })}</span>
					</DialogPrimitive.Close>
				)}
			</DialogPrimitive.Content>
		</DialogPortal>
	);
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="dialog-header"
			className={cn("flex flex-col gap-1 pr-7 text-left", className)}
			{...props}
		/>
	);
}

function DialogFooter({
	className,
	showCloseButton = false,
	children,
	...props
}: React.ComponentProps<"div"> & {
	showCloseButton?: boolean;
}) {
	return (
		<div
			data-slot="dialog-footer"
			className={cn("flex flex-col-reverse gap-2 sm:flex-row sm:justify-end", className)}
			{...props}
		>
			{children}
			{showCloseButton && (
				<DialogPrimitive.Close asChild>
					<Button variant="outline">Close</Button>
				</DialogPrimitive.Close>
			)}
		</div>
	);
}

function DialogTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) {
	return (
		<DialogPrimitive.Title
			data-slot="dialog-title"
			className={cn("text-lg font-semibold", className)}
			{...props}
		/>
	);
}

function DialogDescription({
	className,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
	return (
		<DialogPrimitive.Description
			data-slot="dialog-description"
			className={cn("text-sm text-muted-foreground", className)}
			{...props}
		/>
	);
}

export {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogOverlay,
	DialogPortal,
	DialogTitle,
	DialogTrigger,
};
