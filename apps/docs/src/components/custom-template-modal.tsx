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
  type DeployOption,
  type TemplateKind,
  type TemplateMeta,
} from "@/data/templates";
import {
  buildCustomPresetCommand,
  type CommandProject,
} from "@/lib/create-command";

type ModalLocale = "zh" | "en";
type EnvProvider = "dotenv" | "infisical";
type KindFilter = "all" | TemplateKind;
type ContainerKind = "docker" | "dockerhub" | "ghcr" | "acr";

type Selection = {
  uid: string;
  templateId: string;
  name: string;
  deployCode: string | null;
  containerCode: string | null;
};

const kindOrder: KindFilter[] = ["all", "frontend", "backend", "library"];

const kindIcons: Record<KindFilter, typeof Layers3> = {
  all: Layers3,
  frontend: Globe2,
  backend: Server,
  library: Library,
};

const containerOptions: {
  id: ContainerKind;
  code: string;
  name: Record<ModalLocale, string>;
}[] = [
  { id: "dockerhub", code: "h", name: { zh: "Docker Hub", en: "Docker Hub" } },
  { id: "ghcr", code: "g", name: { zh: "GHCR", en: "GHCR" } },
  { id: "acr", code: "a", name: { zh: "阿里云 ACR", en: "Aliyun ACR" } },
  {
    id: "docker",
    code: "d",
    name: { zh: "通用 Docker Registry", en: "Generic Docker Registry" },
  },
];

const copy = {
  zh: {
    title: "自定义模板",
    subtitle:
      "选择需要的模板与部署目标，右侧会实时生成单条 one create 命令。",
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
    deployModal: {
      title: "选择部署目标",
      containerTitle: "Container 类型",
      cancel: "取消",
      confirm: "添加",
    },
  },
  en: {
    title: "Build your own",
    subtitle:
      "Pick templates and deploy targets; a single one create command appears live on the right.",
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
    deployModal: {
      title: "Choose a deploy target",
      containerTitle: "Container type",
      cancel: "Cancel",
      confirm: "Add",
    },
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
  const [env, setEnv] = useState<EnvProvider>("dotenv");
  const [copied, setCopied] = useState(false);
  const [pendingTemplate, setPendingTemplate] = useState<TemplateMeta | null>(
    null,
  );

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
        dcode: s.deployCode ?? undefined,
        ccode: s.containerCode ?? undefined,
        templateId: t.id,
        title: t.title,
        defaultName: t.defaultName,
        name: s.name,
      }];
    });
    return buildCustomPresetCommand({
      workspaceName,
      env,
      projects,
    });
  }, [selection, env, workspaceName]);

  function attemptAdd(template: TemplateMeta) {
    if (template.deployOptions.length === 0) {
      addToSelection(template, null, null);
      return;
    }
    setPendingTemplate(template);
  }

  function addToSelection(
    template: TemplateMeta,
    deployCode: string | null,
    containerCode: string | null,
  ) {
    setSelection((prev) => [
      ...prev,
      {
        uid: `${template.id}-${Date.now()}-${prev.length}`,
        templateId: template.id,
        name: nextProjectName(template, prev),
        deployCode,
        containerCode,
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
                  <div className="mt-1.5 inline-flex rounded-md border border-stone-200 bg-white p-0.5 text-xs">
                    {(["dotenv", "infisical"] as const).map((opt) => (
                      <button
                        key={opt}
                        type="button"
                        onClick={() => setEnv(opt)}
                        className={[
                          "h-8 rounded px-3 font-medium transition",
                          env === opt
                            ? "bg-stone-900 text-white"
                            : "text-stone-600 hover:text-stone-900",
                        ].join(" ")}
                      >
                        {opt}
                      </button>
                    ))}
                  </div>
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
                        const deployName = s.deployCode
                          ? t.deployOptions.find(
                              (o) => o.code === s.deployCode,
                            )?.name[lang] ?? s.deployCode
                          : null;
                        const containerName = s.containerCode
                          ? containerOptions.find(
                              (o) => o.code === s.containerCode,
                            )?.name[lang] ?? s.containerCode
                          : null;
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
                              {deployName && (
                                <div className="mt-1 inline-flex items-center gap-1 rounded bg-orange-50 px-1.5 py-0.5 text-[10px] text-[#ea580c]">
                                  <Rocket className="size-3" />
                                  {deployName}
                                </div>
                              )}
                              {containerName && (
                                <div className="mt-1 ml-1 inline-flex items-center gap-1 rounded bg-stone-100 px-1.5 py-0.5 text-[10px] text-stone-600">
                                  {containerName}
                                </div>
                              )}
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

      <DeployPickerDialog
        lang={lang}
        template={pendingTemplate}
        text={text}
        onCancel={() => setPendingTemplate(null)}
        onConfirm={({ deployCode, containerCode }) => {
          if (pendingTemplate) {
            addToSelection(pendingTemplate, deployCode, containerCode);
          }
          setPendingTemplate(null);
        }}
      />
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

function DeployPickerDialog({
  lang,
  template,
  text,
  onCancel,
  onConfirm,
}: {
  lang: ModalLocale;
  template: TemplateMeta | null;
  text: (typeof copy)[ModalLocale];
  onCancel: () => void;
  onConfirm: (value: {
    deployCode: string | null;
    containerCode: string | null;
  }) => void;
}) {
  const [chosen, setChosen] = useState<string | null>(null);
  const [container, setContainer] = useState<string>("h");

  // Initialize chosen when template changes.
  useMemoChosenInit(template, setChosen, setContainer);

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) return;
    onCancel();
  }

  return (
    <Dialog
      open={template !== null}
      onOpenChange={handleOpenChange}
      disablePointerDismissal
    >
      <DialogContent
        showCloseButton={false}
        className="!w-[calc(100%-2rem)] !max-w-md !p-0 !gap-0 max-h-[min(640px,calc(100vh-2rem))] flex flex-col overflow-hidden !rounded-xl border border-stone-200 bg-white !text-stone-900"
      >
        {template && (
          <>
            <header className="flex items-start justify-between gap-3 px-5 pb-3 pt-5">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <Rocket className="size-4 shrink-0 text-[#ea580c]" />
                  <DialogTitle className="!font-sans !text-base !font-semibold !leading-tight text-stone-900 truncate">
                    {text.deployModal.title}
                  </DialogTitle>
                </div>
                <p className="mt-1 text-xs leading-5 text-stone-500">
                  {template.title[lang]}
                </p>
              </div>
              <button
                type="button"
                onClick={onCancel}
                aria-label={text.close}
                className="-mr-1 inline-flex size-8 shrink-0 items-center justify-center rounded-md text-stone-400 hover:bg-stone-100 hover:text-stone-700"
              >
                <X className="size-4" />
              </button>
            </header>

            <ul className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-3 pb-3">
              {template.deployOptions.map((opt) => (
                <DeployOptionRow
                  key={opt.code}
                  option={opt}
                  lang={lang}
                  isDefault={opt.code === template.defaultDeployCode}
                  checked={chosen === opt.code}
                  onChoose={() => setChosen(opt.code)}
                />
              ))}
            </ul>

            {chosen === "k" && (
              <div className="border-t border-stone-100 px-5 py-3">
                <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-stone-500">
                  {text.deployModal.containerTitle}
                </p>
                <div className="grid grid-cols-2 gap-1">
                  {containerOptions.map((opt) => (
                    <button
                      key={opt.id}
                      type="button"
                      onClick={() => setContainer(opt.code)}
                      className={[
                        "rounded-md border px-2.5 py-2 text-left text-xs transition",
                        container === opt.code
                          ? "border-orange-200 bg-orange-50 text-stone-900"
                          : "border-stone-200 bg-white text-stone-600 hover:border-stone-300 hover:text-stone-900",
                      ].join(" ")}
                    >
                      {opt.name[lang]}
                    </button>
                  ))}
                </div>
              </div>
            )}

            <footer className="flex items-center justify-end gap-2 border-t border-stone-100 px-5 py-3">
              <button
                type="button"
                onClick={onCancel}
                className="inline-flex h-8 items-center rounded-md px-3 text-sm font-medium text-stone-600 hover:bg-stone-100 hover:text-stone-900"
              >
                {text.deployModal.cancel}
              </button>
              <button
                type="button"
                onClick={() =>
                  onConfirm({
                    deployCode: chosen,
                    containerCode: chosen === "k" ? container : null,
                  })
                }
                className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#ea580c] px-3.5 text-sm font-semibold text-white hover:bg-[#c2410c]"
              >
                {text.deployModal.confirm}
              </button>
            </footer>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function useMemoChosenInit(
  template: TemplateMeta | null,
  setChosen: (v: string | null) => void,
  setContainer: (v: string) => void,
) {
  // Reset chosen when template opens; runs only when template identity changes.
  useMemo(() => {
    if (template) {
      setChosen(template.defaultDeployCode);
      setContainer("h");
    }
  }, [template]);
}

function DeployOptionRow({
  option,
  lang,
  isDefault,
  checked,
  onChoose,
}: {
  option: DeployOption;
  lang: ModalLocale;
  isDefault: boolean;
  checked: boolean;
  onChoose: () => void;
}) {
  return (
    <li>
      <button
        type="button"
        onClick={onChoose}
        className={[
          "flex w-full items-center gap-3 rounded-md px-3 py-2 text-left transition",
          checked
            ? "bg-orange-50 text-stone-900"
            : "text-stone-700 hover:bg-stone-50",
        ].join(" ")}
      >
        <span
          className={[
            "inline-flex size-4 shrink-0 items-center justify-center rounded-full border-2 transition",
            checked
              ? "border-[#ea580c] bg-[#ea580c]"
              : "border-stone-300 bg-white",
          ].join(" ")}
          aria-hidden
        >
          {checked && <Check className="size-2.5 text-white" strokeWidth={3} />}
        </span>
        <span className="min-w-0 flex-1 truncate text-sm font-medium">
          {option.name[lang]}
        </span>
        <code
          className={[
            "shrink-0 rounded px-1.5 py-0.5 font-mono text-[10px] transition",
            checked
              ? "bg-orange-100 text-[#c2410c]"
              : "bg-stone-100 text-stone-500",
          ].join(" ")}
        >
          {option.code}
        </code>
        {isDefault && (
          <span className="shrink-0 text-[10px] text-stone-400">
            {lang === "zh" ? "默认" : "default"}
          </span>
        )}
      </button>
    </li>
  );
}

function nextProjectName(template: TemplateMeta, selection: Selection[]) {
  const taken = selection.map((s) => s.name);
  if (!taken.includes(template.defaultName)) return template.defaultName;
  let i = 2;
  while (taken.includes(`${template.defaultName}-${i}`)) i++;
  return `${template.defaultName}-${i}`;
}
