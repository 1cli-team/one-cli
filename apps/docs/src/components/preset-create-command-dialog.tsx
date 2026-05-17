"use client";

import {
  Check,
  Copy,
  Globe2,
  Layers3,
  Library,
  Plus,
  Rocket,
  Server,
  Trash2,
  X,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import type { Example } from "@/data/examples";
import {
  templates,
  type DeployOption,
  type TemplateKind,
  type TemplateMeta,
} from "@/data/templates";
import {
  buildCustomPresetCommand,
  envFromPresetId,
  projectsFromPresetId,
  sortCommandProjects,
  type CommandProject,
} from "@/lib/create-command";

type DialogLocale = "zh" | "en";
type KindFilter = "all" | TemplateKind;
type ContainerKind = "docker" | "dockerhub" | "ghcr" | "acr";

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
  name: Record<DialogLocale, string>;
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
    title: "复制创建命令",
    subtitle:
      "确认工作区名和子项目名，也可以继续追加模板，命令会实时更新。",
    close: "关闭",
    workspaceLabel: "工作区名",
    projectsLabel: "子项目名",
    addProject: "添加子项目",
    removeProject: "移除子项目",
    pickerTitle: "添加子项目",
    commandLabel: "复制到终端",
    copy: "复制命令",
    copied: "已复制",
    empty: "至少添加一个子项目后才能复制命令。",
    kinds: {
      all: "全部",
      frontend: "前端",
      backend: "后端",
      library: "库",
    } satisfies Record<KindFilter, string>,
    deployModal: {
      title: "选择部署目标",
      containerTitle: "Container 类型",
      cancel: "取消",
      confirm: "添加",
    },
  },
  en: {
    title: "Copy create command",
    subtitle:
      "Confirm names or add more templates; the command updates live.",
    close: "Close",
    workspaceLabel: "Workspace name",
    projectsLabel: "Subproject names",
    addProject: "Add subproject",
    removeProject: "Remove subproject",
    pickerTitle: "Add subproject",
    commandLabel: "Copy to your terminal",
    copy: "Copy command",
    copied: "Copied",
    empty: "Add at least one subproject before copying the command.",
    kinds: {
      all: "All",
      frontend: "Frontend",
      backend: "Backend",
      library: "Library",
    } satisfies Record<KindFilter, string>,
    deployModal: {
      title: "Choose a deploy target",
      containerTitle: "Container type",
      cancel: "Cancel",
      confirm: "Add",
    },
  },
} satisfies Record<DialogLocale, unknown>;

export function PresetCreateCommandDialog({
  lang,
  example,
  open,
  onClose,
}: {
  lang: DialogLocale;
  example: Example | null;
  open: boolean;
  onClose: () => void;
}) {
  const text = copy[lang];
  const [workspaceName, setWorkspaceName] = useState("my-workspace");
  const [projects, setProjects] = useState<CommandProject[]>([]);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [pendingTemplate, setPendingTemplate] = useState<TemplateMeta | null>(
    null,
  );
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!example || !open) return;
    setWorkspaceName(example.workspaceName);
    setProjects(
      projectsFromPresetId(example.presetId).map((project, index) => ({
        ...project,
        uid: `${project.templateId}-${index}-${Date.now()}`,
      })),
    );
    setPickerOpen(false);
    setPendingTemplate(null);
    setCopied(false);
  }, [example, open]);

  const orderedProjects = useMemo(
    () => sortCommandProjects(projects),
    [projects],
  );

  const env = example ? envFromPresetId(example.presetId) : "dotenv";
  const command = useMemo(() => {
    if (projects.length === 0) return "";
    return buildCustomPresetCommand({
      workspaceName,
      env,
      projects,
    });
  }, [env, projects, workspaceName]);

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

  function updateProjectName(uid: string, name: string) {
    setProjects((prev) =>
      prev.map((project) =>
        project.uid === uid ? { ...project, name } : project,
      ),
    );
  }

  function removeProject(uid: string) {
    setProjects((prev) => prev.filter((project) => project.uid !== uid));
  }

  function attemptAdd(template: TemplateMeta) {
    if (template.deployOptions.length === 0) {
      addProject(template, null, null);
      return;
    }
    setPendingTemplate(template);
  }

  function addProject(
    template: TemplateMeta,
    deployCode: string | null,
    containerCode: string | null,
  ) {
    setProjects((prev) => [
      ...prev,
      {
        uid: `${template.id}-${Date.now()}-${prev.length}`,
        kind: template.presetKind,
        tcode: template.code,
        dcode: deployCode ?? undefined,
        ccode: containerCode ?? undefined,
        templateId: template.id,
        title: template.title,
        defaultName: template.defaultName,
        name: nextProjectName(template, prev),
      },
    ]);
    setCopied(false);
  }

  return (
    <>
      <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
        <DialogContent
          showCloseButton={false}
          className="!w-[calc(100%-2rem)] !max-w-2xl !gap-0 !p-0 overflow-hidden !rounded-2xl border border-stone-200 bg-white !text-stone-900 shadow-[0_24px_80px_rgba(10,10,10,0.18)]"
        >
          <header className="flex items-start justify-between gap-4 border-b border-stone-200 px-6 py-5">
            <div>
              <DialogTitle className="!font-sans !text-xl !font-bold !leading-tight text-stone-900">
                {text.title}
              </DialogTitle>
              <p className="mt-1 text-sm leading-6 text-stone-600">
                {text.subtitle}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label={text.close}
              className="inline-flex size-9 shrink-0 items-center justify-center rounded-md border border-stone-200 text-stone-600 hover:border-stone-300 hover:text-stone-900"
            >
              <X className="size-5" />
            </button>
          </header>

          <div className="flex max-h-[min(760px,calc(100vh-10rem))] flex-col gap-5 overflow-auto px-6 py-5">
            <div>
              <label className="block text-xs font-semibold uppercase tracking-wide text-stone-500">
                {text.workspaceLabel}
              </label>
              <input
                className="mt-1.5 h-10 w-full rounded-md border border-stone-200 bg-white px-3 font-mono text-sm text-stone-900 outline-none focus:border-stone-400"
                value={workspaceName}
                onChange={(event) => setWorkspaceName(event.target.value)}
                spellCheck={false}
              />
            </div>

            <div>
              <div className="flex items-center justify-between">
                <span className="block text-xs font-semibold uppercase tracking-wide text-stone-500">
                  {text.projectsLabel}
                </span>
                <button
                  type="button"
                  onClick={() => setPickerOpen(true)}
                  aria-label={text.addProject}
                  title={text.addProject}
                  className="inline-flex size-7 items-center justify-center rounded-md border border-orange-200 bg-orange-50 text-[#ea580c] hover:bg-orange-100"
                >
                  <Plus className="size-3.5" />
                </button>
              </div>

              <ul className="mt-2 flex flex-col gap-2">
                {orderedProjects.map((project) => (
                  <li
                    key={project.uid}
                    className="rounded-md border border-stone-200 bg-[#fafaf9] px-3 py-2.5"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-xs font-semibold text-stone-900">
                          {project.title[lang]}
                        </p>
                        <p className="mt-0.5 font-mono text-[10px] uppercase text-stone-500">
                          {project.kind}
                        </p>
                      </div>
                      <div className="flex shrink-0 items-center gap-1.5">
                        <input
                          className="h-8 w-32 rounded border border-stone-200 bg-white px-2 font-mono text-xs text-stone-800 outline-none focus:border-stone-400 sm:w-44"
                          value={project.name}
                          onChange={(event) =>
                            updateProjectName(project.uid ?? "", event.target.value)
                          }
                          spellCheck={false}
                        />
                        <button
                          type="button"
                          onClick={() => removeProject(project.uid ?? "")}
                          aria-label={text.removeProject}
                          className="inline-flex size-8 items-center justify-center rounded-md text-stone-500 hover:bg-stone-100 hover:text-stone-900"
                        >
                          <Trash2 className="size-4" />
                        </button>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            </div>

            <div className="flex flex-col gap-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-stone-500">
                {text.commandLabel}
              </span>
              <div className="overflow-hidden rounded-md border border-stone-200 bg-stone-950">
                <pre className="max-h-[150px] overflow-auto px-3 py-2.5 font-mono text-[12px] leading-5 text-stone-100">
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
        </DialogContent>
      </Dialog>

      <AddProjectDialog
        lang={lang}
        open={pickerOpen}
        text={text}
        onCancel={() => setPickerOpen(false)}
        onChoose={(template) => {
          setPickerOpen(false);
          attemptAdd(template);
        }}
      />

      <DeployPickerDialog
        lang={lang}
        template={pendingTemplate}
        text={text}
        onCancel={() => setPendingTemplate(null)}
        onConfirm={({ deployCode, containerCode }) => {
          if (pendingTemplate) {
            addProject(pendingTemplate, deployCode, containerCode);
          }
          setPendingTemplate(null);
        }}
      />
    </>
  );
}

function AddProjectDialog({
  lang,
  open,
  text,
  onCancel,
  onChoose,
}: {
  lang: DialogLocale;
  open: boolean;
  text: (typeof copy)[DialogLocale];
  onCancel: () => void;
  onChoose: (template: TemplateMeta) => void;
}) {
  const [activeKind, setActiveKind] = useState<KindFilter>("all");

  useEffect(() => {
    if (open) setActiveKind("all");
  }, [open]);

  const visibleTemplates = useMemo<TemplateMeta[]>(() => {
    if (activeKind === "all") return templates;
    return templates.filter((template) => template.kind === activeKind);
  }, [activeKind]);

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onCancel()}>
      <DialogContent
        showCloseButton={false}
        className="!w-[calc(100%-2rem)] !max-w-2xl !p-0 !gap-0 max-h-[min(720px,calc(100vh-2rem))] flex flex-col overflow-hidden !rounded-xl border border-stone-200 bg-white !text-stone-900"
      >
        <header className="flex items-start justify-between gap-3 border-b border-stone-100 px-5 py-4">
          <div className="min-w-0">
            <DialogTitle className="!font-sans !text-base !font-semibold !leading-tight text-stone-900">
              {text.pickerTitle}
            </DialogTitle>
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

        <div className="flex min-h-0 flex-1 flex-col">
          <div className="flex flex-wrap items-center gap-1.5 px-5 py-3">
            {kindOrder.map((kind) => {
              const Icon = kindIcons[kind];
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
                </button>
              );
            })}
          </div>

          <div className="grid min-h-0 flex-1 auto-rows-min content-start gap-2 overflow-auto px-5 pb-5 sm:grid-cols-2">
            {visibleTemplates.map((template) => (
              <button
                key={template.id}
                type="button"
                onClick={() => onChoose(template)}
                className="rounded-lg border border-stone-200 bg-white p-3 text-left transition hover:border-orange-200 hover:bg-orange-50/40"
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
                  <Plus className="mt-0.5 size-4 shrink-0 text-[#ea580c]" />
                </div>
              </button>
            ))}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function DeployPickerDialog({
  lang,
  template,
  text,
  onCancel,
  onConfirm,
}: {
  lang: DialogLocale;
  template: TemplateMeta | null;
  text: (typeof copy)[DialogLocale];
  onCancel: () => void;
  onConfirm: (value: {
    deployCode: string | null;
    containerCode: string | null;
  }) => void;
}) {
  const [chosen, setChosen] = useState<string | null>(null);
  const [container, setContainer] = useState<string>("h");

  useEffect(() => {
    if (!template) return;
    setChosen(template.defaultDeployCode);
    setContainer("h");
  }, [template]);

  return (
    <Dialog open={template !== null} onOpenChange={(nextOpen) => !nextOpen && onCancel()}>
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

function DeployOptionRow({
  option,
  lang,
  isDefault,
  checked,
  onChoose,
}: {
  option: DeployOption;
  lang: DialogLocale;
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

function nextProjectName(template: TemplateMeta, projects: CommandProject[]) {
  const taken = projects.map((project) => project.name);
  if (!taken.includes(template.defaultName)) return template.defaultName;
  let i = 2;
  while (taken.includes(`${template.defaultName}-${i}`)) i++;
  return `${template.defaultName}-${i}`;
}
