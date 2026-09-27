import { cva, type VariantProps } from "class-variance-authority";
import { Slot } from "radix-ui";
import * as React from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
	"inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap transition-[color,background-color,border-color,box-shadow] duration-150 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/30 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
	{
		variants: {
			variant: {
				default:
					"bg-primary-action text-primary-foreground hover:bg-primary-hover active:bg-primary-hover",
				destructive:
					"bg-destructive text-destructive-foreground hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40",
				outline:
					"border border-input bg-card hover:bg-accent hover:text-accent-foreground dark:bg-input/30 dark:hover:bg-input/50",
				secondary: "bg-secondary text-secondary-foreground hover:bg-secondary/85",
				navigation:
					"justify-start text-left font-normal hover:bg-muted aria-[current=page]:bg-accent aria-[current=page]:text-accent-foreground",
				"danger-ghost": "text-muted-foreground hover:bg-error-surface hover:text-error-foreground",
				ghost: "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
				link: "text-primary-text underline-offset-4 hover:underline",
			},
			size: {
				navigation: "h-auto min-h-14 w-full gap-3 px-3 py-2",
				default: "h-8 px-3 py-1 has-[>svg]:px-3",
				xs: "h-6 gap-1 px-2 text-xs has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
				sm: "h-7 gap-1.5 px-3 text-xs has-[>svg]:px-2.5",
				lg: "h-10 px-5 has-[>svg]:px-4",
				icon: "size-7",
				"icon-xs": "size-5 [&_svg:not([class*='size-'])]:size-3",
				"icon-sm": "size-6",
				"icon-lg": "size-8",
			},
		},
		defaultVariants: {
			variant: "default",
			size: "default",
		},
	},
);

export interface ButtonProps
	extends React.ComponentProps<"button">, VariantProps<typeof buttonVariants> {
	asChild?: boolean;
}

function Button({
	className,
	variant = "default",
	size = "default",
	asChild = false,
	...props
}: ButtonProps) {
	const Comp = asChild ? Slot.Root : "button";

	return (
		<Comp
			type={asChild ? undefined : "button"}
			data-slot="button"
			data-variant={variant}
			data-size={size}
			className={cn(buttonVariants({ variant, size, className }))}
			{...props}
		/>
	);
}

export { Button, buttonVariants };
