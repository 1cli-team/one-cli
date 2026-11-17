import { Combobox as ComboboxPrimitive } from "@base-ui/react/combobox";
import { CheckIcon, ChevronDownIcon } from "lucide-react";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupButton,
	InputGroupInput,
} from "@/components/ui/input-group";
import { cn } from "@/lib/utils";

const Combobox = ComboboxPrimitive.Root;

function ComboboxInput({
	className,
	disabled,
	...props
}: Omit<ComboboxPrimitive.Input.Props, "className"> & { className?: string }) {
	return (
		<InputGroup className={className}>
			<ComboboxPrimitive.Input render={<InputGroupInput />} disabled={disabled} {...props} />
			<InputGroupAddon align="inline-end">
				<ComboboxPrimitive.Trigger
					render={<InputGroupButton size="icon-xs" />}
					disabled={disabled}
					aria-label={props["aria-label"]}
				>
					<ChevronDownIcon className="size-4 text-muted-foreground" />
				</ComboboxPrimitive.Trigger>
			</InputGroupAddon>
		</InputGroup>
	);
}

function ComboboxContent({
	className,
	container,
	...props
}: ComboboxPrimitive.Popup.Props & { container?: HTMLElement | null }) {
	return (
		<ComboboxPrimitive.Portal container={container}>
			<ComboboxPrimitive.Positioner sideOffset={4} className="z-[70]">
				<ComboboxPrimitive.Popup
					data-slot="combobox-content"
					className={cn(
						"max-h-[min(24rem,var(--available-height))] w-[var(--anchor-width)] max-w-[var(--available-width)] overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-lg",
						className,
					)}
					{...props}
				/>
			</ComboboxPrimitive.Positioner>
		</ComboboxPrimitive.Portal>
	);
}

function ComboboxList({ className, ...props }: ComboboxPrimitive.List.Props) {
	return (
		<ComboboxPrimitive.List
			data-slot="combobox-list"
			className={cn("max-h-[min(24rem,var(--available-height))] overflow-y-auto p-1", className)}
			{...props}
		/>
	);
}

function ComboboxItem({ className, children, ...props }: ComboboxPrimitive.Item.Props) {
	return (
		<ComboboxPrimitive.Item
			data-slot="combobox-item"
			className={cn(
				"relative flex cursor-default select-none items-center gap-2 rounded-md py-1.5 pl-2 pr-8 text-sm outline-none data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
				className,
			)}
			{...props}
		>
			{children}
			<ComboboxPrimitive.ItemIndicator className="absolute right-2 flex size-4 items-center justify-center">
				<CheckIcon className="size-4" />
			</ComboboxPrimitive.ItemIndicator>
		</ComboboxPrimitive.Item>
	);
}

export { Combobox, ComboboxInput, ComboboxContent, ComboboxList, ComboboxItem };
