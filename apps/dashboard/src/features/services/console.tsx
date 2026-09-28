/* eslint-disable no-control-regex -- Terminal output contains ANSI control bytes. */
import type { ReactNode } from "react";

// Only text and SGR colors are rendered. OSC links, HTML and terminal commands
// are never interpreted. Joining retained chunks also handles split sequences.
const controls =
	/\x1b\][^\x07]*(?:\x07|\x1b\\)|\x1b\[[0-?]*[ -/]*[a-ln-zA-Z]|[\x00-\x08\x0b\x0c\x0e-\x1a\x1c-\x1f\x7f]/g;
export function consoleText(text: string) {
	return text
		.replace(controls, "")
		.replace(/\x1b\[[0-9;]*m/g, "")
		.replace(/\r\n/g, "\n")
		.replace(/\r/g, "\n");
}
const colors: Record<number, string> = {
	31: "text-error-foreground",
	32: "text-success-foreground",
	33: "text-warning-foreground",
	34: "text-primary-text",
	35: "text-chart-5",
	36: "text-chart-2",
	90: "text-muted-foreground",
	91: "text-error-foreground",
	92: "text-success-foreground",
	93: "text-warning-foreground",
	94: "text-primary-text",
	95: "text-chart-5",
	96: "text-chart-2",
};
export function renderConsole(text: string): ReactNode[] {
	const clean = text.replace(controls, "").replace(/\r\n/g, "\n").replace(/\r/g, "\n");
	const parts = clean.split(/(\x1b\[[0-9;]*m)/g);
	let color = "";
	let bold = false;
	return parts.map((part, index) => {
		if (part.startsWith("\x1b[")) {
			const codes = part.slice(2, -1).split(";").map(Number);
			for (let i = 0; i < codes.length; i++) {
				const code = codes[i];
				if (code === 38 || code === 48) {
					i += codes[i + 1] === 2 ? 4 : 2;
					continue;
				}
				if (code === 0) {
					color = "";
					bold = false;
				} else if (code === 1) bold = true;
				else if (code === 22) bold = false;
				else if (code === 39) color = "";
				else if (colors[code]) color = colors[code];
			}
			return null;
		}
		return (
			<span key={index} className={`${color} ${bold ? "font-bold" : ""}`}>
				{part}
			</span>
		);
	});
}
