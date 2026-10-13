export type UnifiedDiffLineKind = "context" | "removed" | "added";

export interface UnifiedDiffLine {
	kind: UnifiedDiffLineKind;
	text: string;
	beforeLine?: number;
	afterLine?: number;
}

export interface SideBySideDiffRow {
	before?: UnifiedDiffLine;
	after?: UnifiedDiffLine;
}

function fileLines(value: string): string[] {
	const withoutTrailingNewline = value.endsWith("\n") ? value.slice(0, -1) : value;
	return withoutTrailingNewline === "" ? [] : withoutTrailingNewline.split("\n");
}

// The manifest is intentionally rendered in full. Unchanged lines are kept as
// context while the LCS marks only removed and added lines.
export function unifiedFileDiff(before: string, after: string): UnifiedDiffLine[] {
	const beforeLines = fileLines(before);
	const afterLines = fileLines(after);
	const rows = Array.from(
		{ length: beforeLines.length + 1 },
		() => new Uint32Array(afterLines.length + 1),
	);

	for (let beforeIndex = beforeLines.length - 1; beforeIndex >= 0; beforeIndex -= 1) {
		for (let afterIndex = afterLines.length - 1; afterIndex >= 0; afterIndex -= 1) {
			rows[beforeIndex][afterIndex] =
				beforeLines[beforeIndex] === afterLines[afterIndex]
					? rows[beforeIndex + 1][afterIndex + 1] + 1
					: Math.max(rows[beforeIndex + 1][afterIndex], rows[beforeIndex][afterIndex + 1]);
		}
	}

	const result: UnifiedDiffLine[] = [];
	let beforeIndex = 0;
	let afterIndex = 0;
	while (beforeIndex < beforeLines.length || afterIndex < afterLines.length) {
		if (
			beforeIndex < beforeLines.length &&
			afterIndex < afterLines.length &&
			beforeLines[beforeIndex] === afterLines[afterIndex]
		) {
			result.push({
				kind: "context",
				text: beforeLines[beforeIndex],
				beforeLine: beforeIndex + 1,
				afterLine: afterIndex + 1,
			});
			beforeIndex += 1;
			afterIndex += 1;
			continue;
		}

		if (
			beforeIndex < beforeLines.length &&
			(afterIndex >= afterLines.length ||
				rows[beforeIndex + 1][afterIndex] >= rows[beforeIndex][afterIndex + 1])
		) {
			result.push({
				kind: "removed",
				text: beforeLines[beforeIndex],
				beforeLine: beforeIndex + 1,
			});
			beforeIndex += 1;
			continue;
		}

		result.push({
			kind: "added",
			text: afterLines[afterIndex],
			afterLine: afterIndex + 1,
		});
		afterIndex += 1;
	}

	return result;
}

export function sideBySideDiffRows(lines: UnifiedDiffLine[]): SideBySideDiffRow[] {
	const result: SideBySideDiffRow[] = [];
	let index = 0;

	while (index < lines.length) {
		const line = lines[index];
		if (line.kind === "context") {
			result.push({ before: line, after: line });
			index += 1;
			continue;
		}

		const removed: UnifiedDiffLine[] = [];
		const added: UnifiedDiffLine[] = [];
		while (index < lines.length && lines[index].kind !== "context") {
			const changedLine = lines[index];
			if (changedLine.kind === "removed") removed.push(changedLine);
			if (changedLine.kind === "added") added.push(changedLine);
			index += 1;
		}

		const rowCount = Math.max(removed.length, added.length);
		for (let rowIndex = 0; rowIndex < rowCount; rowIndex += 1) {
			result.push({ before: removed[rowIndex], after: added[rowIndex] });
		}
	}

	return result;
}
