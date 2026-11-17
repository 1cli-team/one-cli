import { useId } from "react";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import type { Named } from "./api";

export function Choice({
	label,
	value,
	options,
	onChange,
	placeholder,
	disabled,
}: {
	label: string;
	value: string;
	options: Named[];
	onChange(value: string): void;
	placeholder?: string;
	disabled?: boolean;
}) {
	const id = useId();
	const emptyValue = `${id}-empty`;
	return (
		<div className="grid min-w-0 gap-2">
			<Label htmlFor={id}>{label}</Label>
			<Select
				value={value || (placeholder !== undefined ? emptyValue : "")}
				disabled={disabled}
				onValueChange={(next) => onChange(next === emptyValue ? "" : next)}
			>
				<SelectTrigger id={id} className="h-9">
					<SelectValue placeholder={placeholder} />
				</SelectTrigger>
				<SelectContent>
					{placeholder !== undefined && <SelectItem value={emptyValue}>{placeholder}</SelectItem>}
					{options.map((option) => (
						<SelectItem key={option.id} value={option.id}>
							{option.name}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	);
}
