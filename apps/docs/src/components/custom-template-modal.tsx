"use client";

import {
  Check,
  Copy,
  Globe2,
  Layers3,
  Library,
  Minus,
  Plus,
  Rocket,
  Server,
  X,
} from "lucide-react";
import { useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  templates,
  type TemplateKind,
  type TemplateMeta,
} from "@/data/templates";
import {
  buildCustomPresetCommand,
  type CommandProject,
} from "@/lib/create-command";

type ModalLocale = "zh" | "en";
type KindFilter = "all" | TemplateKind;

type Selection = {
  uid: string;
  templateId: string;
  name: string;
};

const kindOrder: KindFilter[] = ["all", "frontend", "backend", "library"];

const kindIcons: Record<KindFilter, typeof Layers3> = {
  all: Layers3,
  frontend: Globe2,
  backend: Server,
  library: Library,
};

const copy = {
  zh: {
    title: "自定义模板",
    subtitle:
      "选择需要的模板，右侧会实时生成单条 one create 命令。",
    close: "关闭",
    kinds: {
      all: "全部",
      frontend: "前端",
      backend: "后端",
      library: "库",
    } satisfies Record<KindFilter, string>,
    add: "添加",
    remove: "移除",
    selectionTitle: "已选模板",
    empty: "从左侧勾选模板，命令会出现在这里。",
    workspaceLabel: "工作区名",
    envLabel: "Env 提供方",
    cmdLabel: "复制到终端",
    copy: "复制",
    copied: "已复制",
    nameHelp: "决定 workspace 目录；子项目名可在下方分别设置",
  },
  en: {
    title: "Build your own",
    subtitle:
      "Pick templates; a single one create command appears live on the right.",
    close: "Close",
    kinds: {
      all: "All",
      frontend: "Frontend",
      backend: "Backend",
      library: "Library",
    } satisfies Record<KindFilter, string>,
    add: "Add",
    remove: "Remove",
    selectionTitle: "Selected templates",
    empty: "Pick templates on the left to generate the command here.",
    workspaceLabel: "Workspace name",
    envLabel: "Env provider",
    cmdLabel: "Copy to your terminal",
    copy: "Copy",
    copied: "Copied",
    nameHelp: "Used for the workspace directory; subprojects can be named below",
  },
} satisfies Record<ModalLocale, unknown>;

export function CustomTemplateModal({
  lang,
  open,
  onClose,
}: {
  lang: ModalLocale;
  open: boolean;
  onClose: () => void;
}) {
  const text = copy[lang];
  const [activeKind, setActiveKind] = useState<KindFilter>("all");
  const [selection, setSelection] = useState<Selection[]>([]);
  const [workspaceName, setWorkspaceName] = useState("my-workspace");
  const [copied, setCopied] = useState(false);

  const visibleTemplates = useMemo<TemplateMeta[]>(() => {
    if (activeKind === "all") return templates;
    return templates.filter((t) => t.kind === activeKind);
  }, [activeKind]);

  const command = useMemo(() => {
    if (selection.length === 0) return "";
    const projects = selection.flatMap<CommandProject>((s) => {
      const t = templates.find((x) => x.id === s.templateId);
      if (!t) return [];
      return [{
        uid: s.uid,
        kind: t.presetKind,
        tcode: t.code,
        templateId: t.id,
        title: t.title,
        defaultName: t.defaultName,
        name: s.name,
      }];
    });
    return buildCustomPresetCommand({
      workspaceName,
      projects,
    });
  }, [selection, workspaceName]);

  function attemptAdd(template: TemplateMeta) {
    addToSelection(template);
  }

  function addToSelection(
    template: TemplateMeta,
  ) {
    setSelection((prev) => [
      ...prev,
      {
        uid: `${template.id}-${Date.now()}-${prev.length}`,
        templateId: template.id,
        name: nextProjectName(template, prev),
      },
    ]);
  }

  function removeOne(templateId: string) {
    setSelection((prev) => {
      const idx = [...prev].reverse().findIndex((s) => s.templateId === templateId);
      if (idx === -1) return prev;
      const realIdx = prev.length - 1 - idx;
      return prev.filter((_, i) => i !== realIdx);
    });
  }

  function removeByUid(uid: string) {
    setSelection((prev) => prev.filter((s) => s.uid !== uid));
  }

  function updateName(uid: string, name: string) {
    setSelection((prev) =>
      prev.map((s) => (s.uid === uid ? { ...s, name } : s)),
    );
  }

  async function handleCopy() {
    if (!command) return;
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  }

  /**
   * Suppress base-ui Dialog's outside-click close: only close when the
   * user explicitly hits the X button (which calls `onClose` directly).
   * `onOpenChange(false)` fires for outside-click and Esc; we let Esc
   * close via Backdrop, but the consumer asked: clicking the overlay
   * MUST NOT close. We do that by ignoring open=false events that don't
   * come from our own onClose path — implemented here by always re-asserting
   * `open=true` via the prop. The X button calls onClose() which lifts
   * state in the parent.
   */
  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) return; // opening is handled by parent
    onClose();
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={handleOpenChange}
        disablePointerDismissal
      >
        <DialogContent
          showCloseButton={false}
          className="!w-[calc(100%-2rem)] !max-w-[1100px] !p-0 !gap-0 flex h-[min(820px,calc(100vh-2rem))] flex-col overflow-hidden !rounded-2xl border border-stone-200 bg-white !text-stone-900 shadow-[0_24px_80px_rgba(10,10,10,0.18)]"
        >
          <header className="flex items-start justify-between gap-4 border-b border-stone-200 px-6 py-5 sm:px-8">
            <div>
              <DialogTitle className="!font-sans !text-xl !font-bold !leading-tight text-stone-900">
                {text.title}
              </DialogTitle>
              <p className="mt-1 max-w-[640px] text-sm text-stone-600">
                {text.subtitle}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label={text.close}
              className="inline-flex size-9 items-center justify-center rounded-md border border-stone-200 text-stone-600 hover:border-stone-300 hover:text-stone-900"
            >
              <X className="size-5" />
            </button>
          </header>

          <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_380px]">
            <section className="flex min-h-0 flex-col border-stone-200 lg:border-r">
              <div className="flex flex-wrap items-center gap-2 px-6 pb-3 pt-5 sm:px-8">
                {kindOrder.map((kind) => {
                  const Icon = kindIcons[kind];
                  const count =
                    kind === "all"
                      ? templates.length
                      : templates.filter((t) => t.kind === kind).length;
                  const active = activeKind === kind;
                  return (
                    <button
                      key={kind}
                      type="button"
                      onClick={() => setActiveKind(kind)}
                      className={[
                        "inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-xs transition",
                        active
                          ? "border-orange-200 bg-orange-50 text-stone-900"
                          : "border-stone-200 bg-white text-stone-600 hover:border-stone-300 hover:text-stone-900",
                      ].join(" ")}
                    >
                      <Icon className="size-3.5" />
                      {text.kinds[kind]}
                      <span className="font-mono text-[10px] text-stone-500">
                        {count}
                      </span>
                    </button>
                  );
                })}
              </div>
              <div className="grid min-h-0 flex-1 auto-rows-min content-start gap-3 overflow-auto px-6 pb-6 pt-2 sm:grid-cols-2 sm:px-8">
                {visibleTemplates.map((template) => {
                  const count = selection.filter(
                    (s) => s.templateId === template.id,
                  ).length;
                  return (
                    <TemplateChip
                      key={template.id}
                      template={template}
                      lang={lang}
                      count={count}
                      onAdd={() => attemptAdd(template)}
                      onRemove={() => removeOne(template.id)}
                      text={text}
                    />
                  );
                })}
              </div>
            </section>

            <aside className="flex min-h-0 flex-col bg-[#fafaf9]">
              <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-auto px-6 py-5 sm:px-7">
                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wide text-stone-500">
                    {text.workspaceLabel}
                  </label>
                  <input
                    className="mt-1.5 h-10 w-full rounded-md border border-stone-200 bg-white px-3 font-mono text-sm text-stone-900 outline-none focus:border-stone-400"
                    value={workspaceName}
                    onChange={(e) => setWorkspaceName(e.target.value)}
                    spellCheck={false}
                  />
                  <p className="mt-1 text-[11px] text-stone-500">
                    {text.nameHelp}
                  </p>
                </div>

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wide text-stone-500">
                    {text.envLabel}
                  </label>
                  <p className="mt-1.5 text-sm text-stone-900">Infisical</p>
                </div>

                <div>
                  <div className="flex items-center justify-between">
                    <span className="block text-xs font-semibold uppercase tracking-wide text-stone-500">
                      {text.selectionTitle}
                    </span>
                    <span className="font-mono text-xs text-stone-500">
                      {selection.length}
                    </span>
                  </div>
                  {selection.length === 0 ? (
                    <p className="mt-2 rounded-md border border-dashed border-stone-200 bg-white p-3 text-xs text-stone-500">
                      {text.empty}
                    </p>
                  ) : (
                    <ul className="mt-2 flex flex-col gap-2">
                      {selection.map((s) => {
                        const t = templates.find((x) => x.id === s.templateId);
                        if (!t) return null;
                        return (
                          <li
                            key={s.uid}
                            className="flex items-start gap-2 rounded-md border border-stone-200 bg-white px-2.5 py-2"
                          >
                            <div className="min-w-0 flex-1">
                              <div className="truncate text-xs font-semibold text-stone-900">
                                {t.title[lang]}
                              </div>
                              <input
                                className="mt-0.5 h-7 w-full rounded border border-stone-200 px-2 font-mono text-[11px] text-stone-700 outline-none focus:border-stone-400"
                                value={s.name}
                                onChange={(e) =>
                                  updateName(s.uid, e.target.value)
                                }
                                spellCheck={false}
                              />


                            </div>
                            <button
                              type="button"
                              onClick={() => removeByUid(s.uid)}
                              aria-label={text.remove}
                              className="mt-1 inline-flex size-7 items-center justify-center rounded-md text-stone-500 hover:bg-stone-100 hover:text-stone-900"
                            >
                              <X className="size-4" />
                            </button>
                          </li>
                        );
                      })}
                    </ul>
                  )}
                </div>

                <div className="flex flex-col gap-2">
                  <span className="text-xs font-semibold uppercase tracking-wide text-stone-500">
                    {text.cmdLabel}
                  </span>
                  <div className="overflow-hidden rounded-md border border-stone-200 bg-stone-950">
                    <pre className="max-h-[160px] overflow-auto px-3 py-2.5 font-mono text-[12px] leading-5 text-stone-100">
                      {command || `# ${text.empty}`}
                    </pre>
                  </div>
                  <button
                    type="button"
                    onClick={handleCopy}
                    disabled={!command}
                    className={[
                      "inline-flex h-9 items-center justify-center gap-1.5 rounded-md text-sm font-semibold transition",
                      command
                        ? "bg-[#ea580c] text-white hover:bg-[#c2410c]"
                        : "cursor-not-allowed bg-stone-200 text-stone-400",
                    ].join(" ")}
                  >
                    {copied ? (
                      <>
                        <Check className="size-4" />
                        {text.copied}
                      </>
                    ) : (
                      <>
                        <Copy className="size-4" />
                        {text.copy}
                      </>
                    )}
                  </button>
                </div>
              </div>
            </aside>
          </div>
        </DialogContent>
      </Dialog>


    </>
  );
}

function TemplateChip({
  template,
  lang,
  count,
  onAdd,
  onRemove,
  text,
}: {
  template: TemplateMeta;
  lang: ModalLocale;
  count: number;
  onAdd: () => void;
  onRemove: () => void;
  text: (typeof copy)[ModalLocale];
}) {
  const selected = count > 0;
  return (
    <div
      className={[
        "flex flex-col gap-2.5 rounded-lg border bg-white p-3.5 transition",
        selected
          ? "border-orange-300 bg-orange-50/40 shadow-[0_0_0_1px_rgba(234,88,12,0.15)]"
          : "border-stone-200 hover:border-stone-300",
      ].join(" ")}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-stone-900">
            {template.title[lang]}
          </p>
          <p className="mt-0.5 line-clamp-2 text-xs leading-5 text-stone-600">
            {template.tagline[lang]}
          </p>
        </div>
        <span className="shrink-0 rounded-full bg-stone-100 px-2 py-0.5 font-mono text-[10px] uppercase text-stone-500">
          {template.kind}
        </span>
      </div>
      <div className="flex items-center justify-between gap-2">
        <div className="flex flex-wrap gap-1">
          {template.tags.slice(0, 2).map((tag) => (
            <span
              key={tag}
              className="rounded bg-stone-100 px-1.5 py-0.5 font-mono text-[10px] text-stone-500"
            >
              {tag}
            </span>
          ))}
        </div>
        <div className="flex items-center gap-1">
          {selected ? (
            <>
              <button
                type="button"
                onClick={onRemove}
                aria-label={text.remove}
                className="inline-flex size-7 items-center justify-center rounded-md border border-stone-200 text-stone-600 hover:border-stone-300 hover:text-stone-900"
              >
                <Minus className="size-3.5" />
              </button>
              <span className="font-mono text-xs font-semibold text-[#ea580c]">
                ×{count}
              </span>
              <button
                type="button"
                onClick={onAdd}
                aria-label={text.add}
                className="inline-flex size-7 items-center justify-center rounded-md border border-orange-200 bg-orange-50 text-[#ea580c] hover:bg-orange-100"
              >
                <Plus className="size-3.5" />
              </button>
            </>
          ) : (
            <button
              type="button"
              onClick={onAdd}
              className="inline-flex h-7 items-center gap-1 rounded-md border border-stone-200 px-2 text-xs font-medium text-stone-700 hover:border-stone-300 hover:text-stone-900"
            >
              <Plus className="size-3.5" />
              {text.add}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function nextProjectName(template: TemplateMeta, selection: Selection[]) {
  const taken = selection.map((s) => s.name);
  if (!taken.includes(template.defaultName)) return template.defaultName;
  let i = 2;
  while (taken.includes(`${template.defaultName}-${i}`)) i++;
  return `${template.defaultName}-${i}`;
}
