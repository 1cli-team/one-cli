import { useEffect, useId, useRef, useState } from "react";
import type { Editor } from "@tiptap/core";
import { Image } from "@tiptap/extension-image";
import { TaskItem, TaskList } from "@tiptap/extension-list";
import { TableKit } from "@tiptap/extension-table";
import { Markdown } from "@tiptap/markdown";
import { EditorContent, useEditor, useEditorState } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import {
	Bold,
	CodeXml,
	Heading2,
	Italic,
	List,
	ListChecks,
	ListOrdered,
	SquareCode,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import i18n from "@/lib/i18n";
import { cn } from "@/lib/utils";

const markedOptions = { gfm: true };
const extensions = [
	StarterKit.configure({ link: { openOnClick: false, defaultProtocol: "https" } }),
	Image.configure({ inline: true }),
	TableKit,
	TaskList,
	TaskItem.configure({
		nested: true,
		a11y: {
			checkboxLabel: (node) => node.textContent || i18n.t("linear.markdownEditor.emptyTask"),
		},
	}),
	Markdown.configure({ markedOptions }),
];

const tools = [
	{
		key: "bold",
		node: "bold",
		icon: Bold,
		run: (e: Editor) => e.chain().focus().toggleBold().run(),
	},
	{
		key: "italic",
		node: "italic",
		icon: Italic,
		run: (e: Editor) => e.chain().focus().toggleItalic().run(),
	},
	{
		key: "heading",
		node: "heading",
		icon: Heading2,
		run: (e: Editor) => e.chain().focus().toggleHeading({ level: 2 }).run(),
	},
	{
		key: "bulletList",
		node: "bulletList",
		icon: List,
		run: (e: Editor) => e.chain().focus().toggleBulletList().run(),
	},
	{
		key: "orderedList",
		node: "orderedList",
		icon: ListOrdered,
		run: (e: Editor) => e.chain().focus().toggleOrderedList().run(),
	},
	{
		key: "taskList",
		node: "taskList",
		icon: ListChecks,
		run: (e: Editor) => e.chain().focus().toggleTaskList().run(),
	},
	{
		key: "codeBlock",
		node: "codeBlock",
		icon: SquareCode,
		run: (e: Editor) => e.chain().focus().toggleCodeBlock().run(),
	},
] as const;

function containsRawHTML(editor: Editor, markdown: string) {
	const marked = editor.markdown?.instance;
	if (!marked) return false;
	let found = false;
	marked.walkTokens(marked.lexer(markdown, markedOptions), (token) => {
		if (token.type === "html") found = true;
	});
	return found;
}

export default function IssueMarkdownEditor({
	label,
	value,
	onChange,
	disabled,
	compact = false,
}: {
	label: string;
	value: string;
	onChange(markdown: string): void;
	disabled: boolean;
	compact?: boolean;
}) {
	const { t } = useTranslation();
	const id = useId();
	const labelId = `${id}-label`;
	const minHeight = compact ? "min-h-20" : "min-h-32";
	const [source, setSource] = useState(false);
	const [sourceRequired, setSourceRequired] = useState(false);
	const syncedValue = useRef<string | null>(null);
	const editor = useEditor({
		extensions,
		immediatelyRender: false,
		editable: !disabled,
		shouldRerenderOnTransaction: false,
		editorProps: {
			attributes: {
				role: "textbox",
				"aria-labelledby": labelId,
				"aria-multiline": "true",
				class: cn(
					minHeight,
					"px-3 py-2.5 text-sm leading-relaxed outline-none [overflow-wrap:anywhere]",
					"[&_:is(h1,h2,h3,h4,h5,h6)]:font-semibold [&_h1]:text-xl [&_h2]:text-lg [&_h3]:text-base [&>*+*]:mt-2",
					"[&_ul]:list-disc [&_ol]:list-decimal [&_:is(ul,ol)]:pl-5 [&_ul[data-type=taskList]]:list-none [&_ul[data-type=taskList]]:pl-0",
					"[&_li[data-checked]]:flex [&_li[data-checked]]:gap-2 [&_li[data-checked]>div]:min-w-0",
					"[&_blockquote]:border-l-2 [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground",
					"[&_a]:text-primary-text [&_a]:underline [&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:font-mono",
					"[&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:bg-muted [&_pre]:p-3 [&_pre]:text-xs [&_pre_code]:p-0",
					"[&_table]:w-full [&_table]:border-collapse [&_:is(th,td)]:border [&_:is(th,td)]:border-border [&_:is(th,td)]:px-2 [&_:is(th,td)]:py-1 [&_th]:bg-muted [&_img]:max-w-full",
				),
			},
		},
		onUpdate: ({ editor }) => {
			const markdown = editor.isEmpty ? "" : editor.getMarkdown();
			syncedValue.current = markdown;
			onChange(markdown);
		},
	});
	const active = useEditorState({
		editor,
		selector: ({ editor }) => tools.map(({ node }) => editor?.isActive(node) ?? false),
	});

	useEffect(() => {
		if (!editor || editor.isDestroyed || syncedValue.current === value) return;
		syncedValue.current = value;
		try {
			const requiresSource = containsRawHTML(editor, value);
			setSourceRequired(requiresSource);
			if (requiresSource) {
				setSource(true);
			} else if (!value.trim()) {
				editor.commands.clearContent(false);
			} else {
				editor.commands.setContent(value, {
					contentType: "markdown",
					emitUpdate: false,
					errorOnInvalidContent: true,
				});
			}
		} catch {
			setSourceRequired(true);
			setSource(true);
		}
	}, [editor, value]);

	useEffect(() => {
		editor?.setEditable(!disabled, false);
	}, [editor, disabled]);

	const showSource = source || sourceRequired;
	return (
		<div className="space-y-2" role="group" aria-labelledby={labelId}>
			<Label
				id={labelId}
				htmlFor={showSource ? id : undefined}
				onClick={() => {
					if (!showSource && !disabled) editor?.commands.focus();
				}}
			>
				{label}
			</Label>
			<div className="rounded-md border border-input bg-background focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50 dark:bg-input/30">
				<div className="flex flex-wrap items-center justify-between gap-1 rounded-t-md border-b border-border bg-muted/40 px-1.5 py-1">
					{!showSource && (
						<div
							role="group"
							aria-label={t("linear.markdownEditor.formatting", { label })}
							className="flex flex-wrap gap-0.5"
						>
							{tools.map(({ key, icon: Icon, run }, index) => (
								<Button
									key={key}
									type="button"
									size="icon-sm"
									variant={active?.[index] ? "secondary" : "ghost"}
									disabled={!editor || disabled}
									aria-pressed={active?.[index] ?? false}
									aria-label={t(`linear.markdownEditor.${key}`)}
									title={t(`linear.markdownEditor.${key}`)}
									onMouseDown={(event) => event.preventDefault()}
									onClick={() => editor && run(editor)}
								>
									<Icon />
								</Button>
							))}
						</div>
					)}
					<Button
						type="button"
						size="sm"
						variant={showSource ? "secondary" : "ghost"}
						className="ml-auto"
						disabled={disabled || sourceRequired}
						aria-pressed={showSource}
						onClick={() => setSource(!source)}
					>
						<CodeXml data-icon="inline-start" />
						{t("linear.markdownEditor.source")}
					</Button>
				</div>
				{showSource && (
					<Textarea
						id={id}
						className={cn(
							minHeight,
							"rounded-t-none border-0 bg-transparent font-mono focus-visible:ring-0 dark:bg-transparent",
						)}
						disabled={disabled}
						value={value}
						onChange={(event) => onChange(event.target.value)}
					/>
				)}
				<EditorContent editor={editor} hidden={showSource} />
			</div>
			{sourceRequired && (
				<p className="text-xs text-muted-foreground">{t("linear.markdownEditor.sourceRequired")}</p>
			)}
		</div>
	);
}
