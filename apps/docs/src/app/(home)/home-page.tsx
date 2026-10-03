import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  ClipboardCheck,
  Code2,
  FileJson2,
  Layers3,
  PackageCheck,
  Route,
  ShieldCheck,
  Sparkles,
  Wrench,
} from "lucide-react";
import { GithubIcon as Github } from "@/components/github-icon";
import {
  defaultLocale,
  localizedDocsPath,
  localizedTutorialsPath,
  type Locale,
} from "@/i18n";
import { BrandMark } from "@/components/brand-mark";
import { LanguageSwitcher } from "@/components/language-switcher";
import { HomeHeroCanvas } from "./hero-canvas";
import { HomeInstallCommand } from "./home-install-command";
import { HomeCopyButton } from "@/components/copy-button";
import { WorkflowSidebarNav, type WorkflowNavIcon } from "./workflow-nav";
import {
  createPageMetadata,
  jsonLdScriptProps,
  softwareApplicationJsonLd,
  websiteJsonLd,
} from "@/lib/seo";

const homeCopy = {
  zh: {
    meta: {
      title: "One CLI | 创建工作区，运行每个项目",
      description:
        "One CLI 帮助你创建多项目工作区，统一运行开发、构建和测试，通过 Dashboard 管理项目、变量与共享凭据。",
    },
    nav: {
      docs: "文档",
      tutorials: "教程",
      github: "GitHub",
      home: "One CLI 首页",
      navAria: "站点导航",
    },
    hero: {
      byline: "来自 · TORCHSTELLAR",
      title: (
        <>
          从第一个项目
          <br />
          到整个工作区。
          <br />
          用同一套命令。
        </>
      ),
      body: "创建多项目工作区，统一运行开发、构建和测试。通过 Dashboard 管理项目、变量与共享凭据，让人和 AI 使用同一套项目约定。",
      canvasAria:
        "One CLI 工作区模块画布，展示应用、API、文档、Packages、Manifest、Env、Build 和 CLI 接口模块",
      install: "开始使用",
      github: "查看 GitHub",
      copy: "复制",
      copied: "已复制",
      installPlatform: "选择安装平台",
      unixPlatform: "macOS / Linux",
      windowsPlatform: "Windows PowerShell",
    },
    workflow: {
      eyebrow: "常用命令",
      title: "从创建项目，到日常开发。",
      body: "创建和添加项目时按提示选择，日常开发用 one run 执行任务。脚本和 AI 可以通过明确参数与 JSON 输出完成操作。",
      commandBodies: [
        "创建空工作区",
        "追加一个模板项目",
        "运行开发、构建与测试",
        "管理项目环境变量",
        "打开工作区 Dashboard",
        "查阅完整命令",
      ],
      createEyebrow: "one create",
      createTitle: (
        <>
          一条命令，
          <br />
          先把项目搭起来。
        </>
      ),
      createBody:
        "one create 创建空工作区，准备好目录、项目说明和任务入口。之后通过 one add 逐个添加需要的项目。",
      bullets: [
        ["直接创建", "one create my-app --yes 会写入基础项目文件。"],
        ["添加项目", "通过 one add 选择模板，为工作区添加项目。"],
        ["结果可查", "完成后返回工作区位置和生成的文件，方便继续添加项目。"],
      ],
      explore: "查看 one create",
      details: [
        {
          command: "one create",
          navBody: "创建空工作区",
          eyebrow: "one create",
          title: (
            <>
              一条命令，
              <br />
              先把项目搭起来。
            </>
          ),
          body: "one create 可以直接运行并填写目标目录，也可以指定目录名，创建空工作区后再用 one add 添加项目。",
          bullets: [
            ["交互创建", "直接输入 one create，会引导你填写目标目录。"],
            ["添加项目", "运行 one templates 查看模板，再通过 one add 添加所需项目。"],
            ["自动化模式", "CI 或 AI 使用 --yes 和 -o json，方便确认输出和错误。"],
          ],
          href: ["create"],
          cta: "查看 one create",
          secondaryCta: "查看模板指南",
          secondaryHref: "templates",
          sample: "one create",
          output: [
            "打开创建向导",
            "填写目标目录",
            "生成工作区目录、AGENTS.md 和 mise 任务配置",
          ],
        },
        {
          command: "one add",
          navBody: "追加一个模板项目",
          eyebrow: "one add",
          title: (
            <>
              项目已经有了，
              <br />
              也能继续加。
            </>
          ),
          body: "one add 在已有 One 工作区里添加项目。你可以直接输入 one add 进入交互式选择，也可以在脚本里写明模板名和项目名。",
          bullets: [
            ["交互添加", "直接输入 one add，会让你选择模板和项目名。"],
            ["自动化模式", "脚本或 AI 可使用 one add nextjs-app --name web --yes。"],
            ["同步默认值", "按模板生成项目代码，并登记本地开发命令。"],
          ],
          href: ["add"],
          cta: "查看 one add",
          sample: "one add",
          output: [
            "确认当前目录属于 One 项目",
            "打开模板和项目名选择器",
            "写入新目录并登记到项目清单",
          ],
        },
        {
          command: "one run",
          navBody: "运行开发、构建与测试",
          eyebrow: "one run",
          title: (
            <>
              不同项目，
              <br />
              同一套运行方式。
            </>
          ),
          body: "one run 列出工作区任务，通过 mise 执行开发、构建和测试。用 -p 选择项目，One 会按需准备工具和依赖。",
          bullets: [
            ["发现任务", "运行 one run 查看可用任务，再选择要执行的操作。"],
            ["日常开发", "one dev -p web 启动项目；one build -p web 构建项目。"],
            ["查看日志", "多任务支持树形日志界面，可搜索、滚动和切换任务。"],
          ],
          href: ["run"],
          cta: "查看任务管理",
          sample: "one dev -p web",
          output: [
            "解析项目任务与依赖关系",
            "准备工具、依赖和已绑定的项目变量",
            "启动开发服务并显示原始日志",
          ],
        },
        {
          command: "one env",
          navBody: "管理项目环境变量",
          eyebrow: "one env",
          title: (
            <>
              变量集中管理，
              <br />
              运行时按需注入。
            </>
          ),
          body: "通过 one login 在浏览器登录 Infisical，再用 one env 管理项目变量。新工作区可先直接运行，需要托管变量时再绑定。",
          bullets: [
            ["浏览器登录", "登录会话保存在系统钥匙串，使用 one whoami 查看状态。"],
            ["项目变量", "首次保存变量时初始化工作区绑定，按环境与项目目录组织变量。"],
            ["共享凭据", "在 Dashboard 配置共享凭据，通过 one exec --global 显式注入。"],
          ],
          href: ["env-vars"],
          cta: "查看环境变量",
          sample: "one env set API_URL -p web",
          output: [
            "先通过 one login 登录",
            "在终端隐藏输入变量值",
            "运行项目时从 Infisical 拉取并注入变量",
          ],
        },
        {
          command: "one serve",
          navBody: "打开工作区 Dashboard",
          eyebrow: "one serve",
          title: (
            <>
              项目和环境变量，
              <br />
              在浏览器里管理。
            </>
          ),
          body: "one serve 打开本机 Dashboard，浏览工作区、添加项目、管理环境变量和共享凭据。",
          bullets: [
            ["项目管理", "查看项目配置、选择模板添加项目，审阅 Manifest 修改后保存。"],
            ["共享凭据", "集中管理发布与运维凭据，在终端任务中按需注入。"],
            ["账号与变量", "在设置中登录 Infisical，按环境和目录管理变量。"],
          ],
          href: ["serve"],
          cta: "查看 Dashboard",
          sample: "one serve",
          output: [
            "在本机浏览器中打开 Dashboard",
            "管理工作区、项目与共享凭据",
            "按环境管理项目变量",
          ],
        },
        {
          command: "one help",
          navBody: "查阅完整命令",
          eyebrow: "one help",
          title: (
            <>
              让 AI 助手，
              <br />
              找到正确的命令。
            </>
          ),
          body: "使用 one help --all 查看完整命令目录，再通过子命令的 --help 获取参数说明。AI 可以结合 README 和 one.manifest.toml 里的项目事实选择操作。",
          bullets: [
            ["完整目录", "列出当前版本支持的全部命令。"],
            ["参数说明", "使用子命令的 --help 确认参数和用法。"],
            ["结构化结果", "执行时使用 -o json，根据 error.code 处理失败。"],
          ],
          href: ["cli-overview"],
          cta: "查看命令参考",
          sample: "one help --all",
          output: [
            "查看当前版本支持的命令",
            "确认子命令参数",
            "结合项目说明选择操作",
          ],
        },
      ],
    },
    why: {
      eyebrow: "AGENT-READY WORKSPACE",
      title: "让 agent 先读事实，再动项目。",
      body: "One CLI 不是另一个聊天入口。它把 manifest、项目配置和结构化错误组织成一套工程契约，让 Codex、Claude Code、Cursor 这类 AI 编程工具能先确认项目事实，再选择命令、修改代码和处理失败。",
      cards: [
        {
          icon: Sparkles,
          title: "命令帮助说明怎么操作",
          body: "agent 通过 `one help --all` 和子命令的 `--help` 确认当前版本支持的命令、参数和用法，再执行工作区操作。",
          chip: "one help --all",
        },
        {
          icon: ClipboardCheck,
          title: "Manifest 记录项目事实",
          body: "`one.manifest.toml` 记录工作区身份、项目路径、工具链和 Infisical 绑定。agent 先读这份事实，再决定进入哪个子项目、调用哪个命令。",
          chip: "one.manifest.toml",
        },
        {
          icon: Wrench,
          title: "项目说明由团队维护",
          body: "one create 按当前 CLI 语言生成 AGENTS.md。之后由团队维护，添加项目不会覆盖已有内容。",
          chip: "AGENTS.md",
        },
        {
          icon: FileJson2,
          title: "错误给出下一步",
          body: "命令支持 JSON 输出，错误包含 `code`、`context` 和 `remediation[]`。agent 不需要解析自由文本，可以按结构化信息选择恢复动作。",
          chip: "error.remediation[]",
        },
      ],
    },
    json: {
      eyebrow: "可读的错误与恢复建议",
      title: "命令没跑通，不用自己猜。",
      body: "创建、添加和运行项目遇到问题时，One CLI 会保留具体原因，并为常见错误提供恢复建议。人和 AI 可以根据相同的信息继续处理。",
      cards: [
        ["创建项目", "目录不对或目标已存在，会告诉你怎么继续"],
        ["添加项目", "模板、名称、位置不对，会给出修复命令"],
        ["运行任务", "缺少配置或工具，会提示先补什么"],
        ["接给 AI", "同样结果可以输出 JSON，方便工具处理"],
      ],
      sampleLabel: "one add react-spa --name web --yes",
      sampleOutput: `未检测到 One CLI 项目，请在项目根目录执行。
  - 当前目录缺少 one.manifest.toml；
    请先创建工作区，或 cd 到已有工作区：
    one create <dir>`,
      automationNote: "结构化错误包含错误码和上下文；提供恢复建议时，脚本和 AI 可读取 remediation 字段。",
    },
    startWays: {
      eyebrow: "开始方式",
      title: "三种开始方式。",
      body: "直接运行 one create 创建工作区，或先用 one templates 查看可用模板；也可以向 AI 描述目标，由它通过 One CLI 完成创建。",
      direct: {
        label: "方式一",
        title: "直接运行 one create",
        body: "先创建空工作区。输入 one create，按提示填写目录；之后用 one add 添加应用、服务或共享库。",
        bullets: ["不需要先记参数", "按提示填写目标目录", "创建后再用 one add 继续添加项目"],
        cta: "查看 one create",
        metaLabel: "创建工作区",
        metaValue: "one create",
      },
      template: {
        label: "方式二",
        title: "运行 one templates",
        body: "在终端查看当前版本内置的应用、服务和共享库模板，再通过 one add 添加所需项目。",
        bullets: ["终端查看模板清单", "了解模板类型与用途", "通过 one add 添加所需项目"],
        cta: "查看模板命令",
        metaLabel: "查看可用模板",
        metaValue: "one templates",
      },
      ai: {
        label: "方式三",
        title: "交给 AI 执行",
        body: "把目标和 One CLI 命令交给 Codex、Claude Code、Cursor 等 AI 编程工具；已有工作区先阅读 README 和 one.manifest.toml。",
        bullets: ["直接描述要做什么", "AI 使用 One CLI 创建工作区或追加模板", "通过 one run 准备依赖并运行任务"],
        cta: "查看 AI 指南",
        promptLabel: "给 agent 的一句话",
        prompt: "请使用 One CLI 创建 media-stack 工作区，添加一个 Expo 移动应用，再通过 one dev 启动它。",
      },
    },
    final: {
      title: "把项目放在一起，把开发跑起来。",
      body: "从一个空工作区开始，按需添加项目，用相同的命令完成每天的开发。",
      install: "开始使用",
    },
    footer: {
      body: "面向人和 AI 的工作区开发工具。",
      docs: "文档",
      tutorials: "教程",
      templates: "模板",
      project: "项目",
      links: {
        installation: "安装",
        quickStart: "一条命令开始",
        tutorialsHome: "项目模板教程",
        firstWorkspace: "第一个工作区",
        envVars: "配置环境变量",
        templateCommand: "查看模板命令",
        templateGuide: "怎么选模板",
        commandOverview: "命令总览",
        releases: "版本发布",
      },
      built: "基于 Next.js、Fumadocs 和 One CLI 构建。",
    },
  },
  en: {
    meta: {
      title: "One CLI | Create workspaces. Run your projects.",
      description:
        "Create workspaces, run development, builds, and tests, and manage projects, variables, and shared credentials in Dashboard.",
    },
    nav: {
      docs: "Docs",
      tutorials: "Tutorials",
      github: "GitHub",
      home: "One CLI Home",
      navAria: "Site navigation",
    },
    hero: {
      byline: "BY · TORCHSTELLAR",
      title: (
        <>
          Every project.
          <br />
          One workspace.
          <br />
          One workflow.
        </>
      ),
      body: "Create workspaces and run development, builds, and tests. Manage projects, variables, and shared credentials in Dashboard, with shared conventions for people and AI.",
      canvasAria:
        "One CLI workspace module canvas showing apps, API, docs, packages, manifest, env, build, and CLI interface modules",
      install: "Start building",
      github: "View on GitHub",
      copy: "copy",
      copied: "copied",
      installPlatform: "Choose an install platform",
      unixPlatform: "macOS / Linux",
      windowsPlatform: "Windows PowerShell",
    },
    workflow: {
      eyebrow: "Common commands",
      title: "From project setup to daily development.",
      body: "Follow prompts to create workspaces and add projects, then use one run for daily tasks. Scripts and AI can use explicit arguments and JSON output.",
      commandBodies: [
        "create an empty workspace",
        "add a template project",
        "run development, builds, and tests",
        "manage project variables",
        "open the workspace Dashboard",
        "find the right command",
      ],
      createEyebrow: "ONE CREATE",
      createTitle: (
        <>
          Start a project
          <br />
          with one command.
        </>
      ),
      createBody:
        "one create prepares an empty workspace with directories, project guidance, and task configuration. Add the projects you need with one add.",
      bullets: [
        ["DIRECT CREATE", "one create my-app --yes writes the base project files."],
        ["ADD PROJECTS", "Use one add to select a template and add a project."],
        ["CHECKABLE", "The result includes the workspace location and generated files so you can add projects next."],
      ],
      explore: "Explore one create",
      details: [
        {
          command: "one create",
          navBody: "create an empty workspace",
          eyebrow: "ONE CREATE",
          title: (
            <>
              Start a project
              <br />
              with one command.
            </>
          ),
          body: "Run one create to choose a target directory, or pass the directory explicitly. Then use one add to add projects to the empty workspace.",
          bullets: [
            ["PROMPTED CREATE", "Run one create by itself to choose the target directory."],
            ["ADD PROJECTS", "Run one templates to list templates, then add projects with one add."],
            ["AUTOMATION", "CI and AI use --yes and -o json for predictable output and errors."],
          ],
          href: ["create"],
          cta: "Explore one create",
          secondaryCta: "Read the template guide",
          secondaryHref: "templates",
          sample: "one create",
          output: [
            "opens the create wizard",
            "asks for a target directory",
            "writes workspace directories, AGENTS.md, and mise task configuration",
          ],
        },
        {
          command: "one add",
          navBody: "add a template project",
          eyebrow: "ONE ADD",
          title: (
            <>
              Keep building
              <br />
              after the project exists.
            </>
          ),
          body: "one add adds a project to an existing One workspace. You can run one add directly to use the picker, or pass the template name and project name in scripts.",
          bullets: [
            ["PROMPTED ADD", "Run one add to choose the template and project name."],
            ["AUTOMATION", "CI and AI use one add nextjs-app --name web --yes."],
            ["SYNC DEFAULTS", "Templates generate project code and register local development commands."],
          ],
          href: ["add"],
          cta: "Explore one add",
          sample: "one add",
          output: [
            "checks that cwd belongs to a One project",
            "opens template and project-name prompts",
            "writes the new directory and records it in the project list",
          ],
        },
        {
          command: "one run",
          navBody: "run development, builds, and tests",
          eyebrow: "ONE RUN",
          title: (
            <>
              Different projects.
              <br />
              One way to run them.
            </>
          ),
          body: "one run lists workspace tasks and uses mise to run development, builds, and tests. Select a project with -p; One prepares tools and dependencies as needed.",
          bullets: [
            ["DISCOVER TASKS", "Run one run to see available tasks and choose what to execute."],
            ["DAILY DEVELOPMENT", "one dev -p web starts a project; one build -p web builds it."],
            ["FOLLOW LOGS", "Multi-task runs offer a log tree with search, scrolling, and task selection."],
          ],
          href: ["run"],
          cta: "Explore task management",
          sample: "one dev -p web",
          output: [
            "resolve project tasks and dependencies",
            "prepare tools, dependencies, and bound project variables",
            "start the dev service and show its original logs",
          ],
        },
        {
          command: "one env",
          navBody: "manage project variables",
          eyebrow: "ONE ENV",
          title: (
            <>
              Manage variables.
              <br />
              Inject them when needed.
            </>
          ),
          body: "Sign in to Infisical in your browser with one login, then manage project variables with one env. New workspaces can run before you connect variable storage.",
          bullets: [
            ["BROWSER LOGIN", "The system keyring stores your session. Check its status with one whoami."],
            ["PROJECT VARIABLES", "Saving the first variable initializes the binding. Organize values by environment and project folder."],
            ["SHARED CREDENTIALS", "Configure shared credentials in Dashboard and inject them explicitly with one exec --global."],
          ],
          href: ["env-vars"],
          cta: "Explore environment variables",
          sample: "one env set API_URL -p web",
          output: [
            "sign in first with one login",
            "enter the value in a hidden terminal prompt",
            "fetch and inject variables from Infisical when the project runs",
          ],
        },
        {
          command: "one serve",
          navBody: "open the workspace Dashboard",
          eyebrow: "ONE SERVE",
          title: (
            <>
              Manage projects
              <br />
              in your browser.
            </>
          ),
          body: "one serve opens your local Dashboard to browse workspaces, add projects, and manage variables and shared credentials.",
          bullets: [
            ["PROJECTS", "Inspect configuration, add projects from templates, and review Manifest changes before saving."],
            ["SHARED CREDENTIALS", "Manage publishing and operations credentials centrally, and inject them into terminal tasks when needed."],
            ["ACCOUNTS AND VARIABLES", "Sign in to Infisical in Settings and manage variables by environment and folder."],
          ],
          href: ["serve"],
          cta: "Explore Dashboard",
          sample: "one serve",
          output: [
            "open Dashboard in your local browser",
            "manage workspaces, projects, and shared credentials",
            "manage project variables by environment",
          ],
        },
        {
          command: "one help",
          navBody: "find the right command",
          eyebrow: "ONE HELP",
          title: (
            <>
              Help AI find
              <br />
              the right command.
            </>
          ),
          body: "Use one help --all for the complete command catalogue, then a subcommand's --help for its options. Agents can combine this with README files and one.manifest.toml to choose project operations.",
          bullets: [
            ["FULL CATALOGUE", "List every command supported by the installed version."],
            ["OPTIONS", "Check a subcommand's --help for arguments and usage."],
            ["JSON RESULTS", "Use -o json and handle failures through error.code."],
          ],
          href: ["cli-overview"],
          cta: "Explore command reference",
          sample: "one help --all",
          output: [
            "find commands supported by this version",
            "check subcommand options",
            "choose operations using project guidance",
          ],
        },
      ],
    },
    why: {
      eyebrow: "AGENT-READY WORKSPACE",
      title: "Let agents read facts before touching the project.",
      body: "One CLI is not another chat surface. It organizes manifests, project configuration, and structured errors into one engineering contract so Codex, Claude Code, and Cursor can confirm project facts before choosing commands, editing code, or recovering from failures.",
      cards: [
        {
          icon: Sparkles,
          title: "Command help explains usage",
          body: "Agents use `one help --all` and subcommand `--help` to check the commands, options, and usage supported by the installed version before acting.",
          chip: "one help --all",
        },
        {
          icon: ClipboardCheck,
          title: "Manifest records project facts",
          body: "`one.manifest.toml` records workspace identity, project paths, toolchains, and the Infisical binding. Agents read those facts before choosing a subproject or command.",
          chip: "one.manifest.toml",
        },
        {
          icon: Wrench,
          title: "Teams own project instructions",
          body: "one create generates AGENTS.md in the current CLI language. Teams maintain it afterward; adding projects does not overwrite it.",
          chip: "AGENTS.md",
        },
        {
          icon: FileJson2,
          title: "Errors include next steps",
          body: "Commands support JSON output, and errors include `code`, `context`, and `remediation[]`. Agents can choose a recovery action without parsing free text.",
          chip: "error.remediation[]",
        },
      ],
    },
    json: {
      eyebrow: "Clear errors and recovery hints",
      title: "When a command fails, you do not have to guess.",
      body: "When creating, adding, or running projects fails, One CLI preserves the cause and provides recovery hints for common errors. People and AI can use the same details to continue.",
      cards: [
        ["Create projects", "Wrong folder or existing target: it tells you how to continue"],
        ["Add projects", "Template, name, or location issues come with a fix command"],
        ["Run tasks", "Missing config or tools are called out before you continue"],
        ["Connect AI", "The same result can be emitted as JSON for tools"],
      ],
      sampleLabel: "one add react-spa --name web --yes",
      sampleOutput: `No One CLI workspace found. Run this from a workspace root.
  - This folder is missing one.manifest.toml.
    Create a workspace first, or cd into an existing one:
    one create <dir>`,
      automationNote: "Structured errors include a code and context. Scripts and AI can read the remediation field when recovery hints are available.",
    },
    startWays: {
      eyebrow: "Start here",
      title: "Three ways to start.",
      body: "Create a workspace with one create, list available templates with one templates, or describe your goal to an agent that uses One CLI.",
      direct: {
        label: "Option one",
        title: "Run one create",
        body: "Start with an empty workspace. Run one create and choose a directory, then add apps, services, or shared libraries with one add.",
        bullets: ["No flags to memorize first", "Choose a target directory", "Use one add later for more projects"],
        cta: "Explore one create",
        metaLabel: "Create a workspace",
        metaValue: "one create",
      },
      template: {
        label: "Option two",
        title: "Run one templates",
        body: "List the apps, services, and shared library templates bundled with your CLI, then add projects with one add.",
        bullets: ["List templates in the terminal", "Compare their type and purpose", "Add projects with one add"],
        cta: "Explore the templates command",
        metaLabel: "List available templates",
        metaValue: "one templates",
      },
      ai: {
        label: "Option three",
        title: "Hand it to AI",
        body: "Give your goal and One CLI commands to an AI coding tool. In an existing workspace, have it read README files and one.manifest.toml first.",
        bullets: ["Describe what you want to build", "AI uses One CLI to create workspaces or add templates", "Use one run to prepare dependencies and execute tasks"],
        cta: "Explore AI guide",
        promptLabel: "one-line agent prompt",
        prompt: "Please use One CLI to create a media-stack workspace, add an Expo mobile app, and start it with one dev.",
      },
    },
    final: {
      title: "Bring your projects together. Get them running.",
      body: "Start with an empty workspace, add projects as you need them, and use the same commands for daily development.",
      install: "Start building",
    },
    footer: {
      body: "Workspace development for people and AI.",
      docs: "Docs",
      tutorials: "Tutorials",
      templates: "Templates",
      project: "Project",
      links: {
        installation: "Installation",
        quickStart: "Start with one command",
        tutorialsHome: "Template tutorial",
        firstWorkspace: "First workspace",
        envVars: "Configure env vars",
        templateCommand: "List templates",
        templateGuide: "Choose templates",
        commandOverview: "Command overview",
        releases: "Releases",
      },
      built: "Built with Next.js, Fumadocs, and One CLI.",
    },
  },
} as const;

type HomeText = (typeof homeCopy)[Locale];
type WorkflowDetail = HomeText["workflow"]["details"][number];

const commandNames = [
  "one create",
  "one add",
  "one run",
  "one env",
  "one serve",
  "one help",
] as const;

const commandIcons = [Code2, Layers3, Wrench, FileJson2, Route, Sparkles] as const;
const commandIconNames = [
  "code",
  "layers",
  "wrench",
  "file-json",
  "route",
  "sparkles",
] as const satisfies readonly WorkflowNavIcon[];
const commandSlugs = commandNames.map((command) => command.replace(/\s+/g, "-"));

export function generateHomeMetadata(lang: Locale): Metadata {
  const text = homeCopy[lang];

  return createPageMetadata({
    title: text.meta.title,
    description: text.meta.description,
    path: localizedHomePath(lang),
    locale: lang,
    alternates: alternateHomeLanguages(),
  });
}

export function LocalizedHomePage({ lang }: { lang: Locale }) {
  const text = homeCopy[lang];

  return (
    <main className="min-h-screen w-full bg-[#0a0a0a] text-[#fafaf9]">
      <script
        {...jsonLdScriptProps([
          websiteJsonLd(lang),
          softwareApplicationJsonLd(lang),
        ])}
      />
      <HomeNav lang={lang} text={text} />
      <Hero lang={lang} text={text} />
      <StartWaysSection lang={lang} text={text} />
      <WorkflowSection lang={lang} text={text} />
      <WhySection text={text} />
      <JsonSection text={text} />
      <FinalCTA lang={lang} text={text} />
      <Footer lang={lang} text={text} />
    </main>
  );
}

function HomeNav({ lang, text }: { lang: Locale; text: HomeText }) {
  const navItems = [
    [text.nav.tutorials, localizedTutorialsPath(lang, ["templates"])],
    [text.nav.docs, localizedDocsPath(lang, ["quick-start"])],
  ] as const;

  return (
    <header className="sticky top-0 z-30 border-b border-[#292524] bg-[#0a0a0a]/92 backdrop-blur-xl">
      <div className="mx-auto grid w-full max-w-[1440px] grid-cols-[1fr_auto] grid-rows-[60px_44px] items-center gap-x-3 px-4 md:flex md:h-16 md:justify-between md:gap-4 md:px-5 lg:px-20">
        <Link href={localizedHomePath(lang)} className="inline-flex w-fit items-center md:order-1" aria-label={text.nav.home}>
          <BrandMark variant="dark" />
        </Link>
        <nav aria-label={text.nav.navAria} className="col-span-2 row-start-2 flex h-full items-center gap-6 border-t border-white/10 text-sm md:order-2 md:gap-7 md:border-0">
          {navItems.map(([label, href]) => (
            <Link key={label} href={href} className="inline-flex h-full items-center text-stone-400 transition hover:text-white">
              {label}
            </Link>
          ))}
        </nav>
        <div className="col-start-2 row-start-1 flex items-center justify-end gap-2 md:order-3">
          <LanguageSwitcher lang={lang} variant="dark" />
          <a
            href="https://github.com/1cli-team/one-cli"
            target="_blank"
            rel="noreferrer"
            className="inline-flex size-9 shrink-0 items-center justify-center rounded-md text-stone-300 transition hover:bg-white/5 hover:text-white"
            aria-label={text.nav.github}
          >
            <Github className="size-5" />
          </a>
        </div>
      </div>
    </header>
  );
}

function Hero({ lang, text }: { lang: Locale; text: HomeText }) {
  const heroTitleClassName =
    lang === "zh"
      ? "text-[2.75rem] font-bold leading-[1.08] text-white md:text-[3.75rem]"
      : "text-[2.7rem] font-bold leading-[1.06] text-white md:text-[3rem]";

  return (
    <section className="relative border-b border-[#292524]">
      <div className="relative mx-auto grid min-h-[620px] w-full max-w-[1440px] items-start gap-12 px-5 py-16 md:items-center lg:grid-cols-[minmax(0,0.95fr)_minmax(380px,0.9fr)] lg:px-24 lg:py-20">
        <div className="flex w-full min-w-0 max-w-[620px] flex-col gap-7">
          <p className="font-mono text-xs text-stone-500">{text.hero.byline}</p>
          <h1 className={heroTitleClassName}>
            {text.hero.title}
          </h1>
          <p className="w-full max-w-[520px] text-base leading-7 text-stone-400">
            {text.hero.body}
          </p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Link
              href={localizedDocsPath(lang, ["installation"])}
              className="inline-flex h-11 items-center justify-center gap-2 rounded-md bg-[#ea580c] px-5 text-sm font-semibold text-white transition hover:bg-[#c2410c]"
            >
              {text.hero.install}
            </Link>
            <a
              href="https://github.com/1cli-team/one-cli"
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-11 items-center justify-center gap-2 rounded-md border border-white/10 px-5 text-sm font-semibold text-stone-100 transition hover:border-orange-500/60 hover:bg-white/5"
            >
              {text.hero.github}
            </a>
          </div>
          <HomeInstallCommand
            platformLabel={text.hero.installPlatform}
            unixLabel={text.hero.unixPlatform}
            windowsLabel={text.hero.windowsPlatform}
            copyLabel={text.hero.copy}
            copiedLabel={text.hero.copied}
          />
        </div>
        <HomeHeroCanvas ariaLabel={text.hero.canvasAria} lang={lang} />
      </div>
    </section>
  );
}

function WorkflowSection({ lang, text }: { lang: Locale; text: HomeText }) {
  const workflowCommands = text.workflow.details.map((item, index) => ({
    ...item,
    Icon: commandIcons[index] ?? Code2,
    iconName: commandIconNames[index] ?? "code",
    slug: commandSlugs[index] ?? item.command.replace(/\s+/g, "-"),
  }));
  const workflowNavItems = workflowCommands.map(
    ({ command, navBody, iconName, slug }) => ({
      command,
      navBody,
      icon: iconName,
      slug,
    }),
  );

  return (
    <section className="border-b border-[#292524] bg-[#0a0a0a]">
      <div className="mx-auto max-w-[1440px]">
        <div className="px-5 py-16 lg:px-24 lg:py-[5.5rem]">
          <SectionHeader
            eyebrow={text.workflow.eyebrow}
            title={text.workflow.title}
            body={text.workflow.body}
          />
        </div>
        <div className="border-t border-[#292524] lg:grid lg:grid-cols-[292px_minmax(0,1fr)]">
          <aside className="sticky top-16 hidden h-[calc(100vh-4rem)] overflow-y-auto border-r border-[#292524] lg:block">
            <WorkflowSidebarNav items={workflowNavItems} />
          </aside>
          <div className="lg:hidden">
            <div className="flex gap-2 overflow-x-auto border-b border-[#292524] px-5 py-4">
              {workflowCommands.map(({ command, Icon, slug }) => (
                <a
                  key={command}
                  href={`#${slug}`}
                  className="inline-flex h-10 shrink-0 items-center gap-2 rounded-md border border-[#292524] bg-[#1c1917] px-3 font-mono text-xs font-semibold text-stone-200"
                >
                  <Icon className="size-4 text-[#ea580c]" />
                  {command}
                </a>
              ))}
            </div>
          </div>
          <div>
            {workflowCommands.map((item, index) => (
              <section
                key={item.command}
                id={item.slug}
                className={[
                  "grid scroll-mt-20 border-[#292524] lg:min-h-[540px] lg:grid-cols-[minmax(0,0.82fr)_minmax(420px,1fr)]",
                  index === 0 ? "" : "border-t",
                ].join(" ")}
              >
                <div className="flex flex-col justify-start px-5 py-12 md:px-10 lg:px-14 lg:py-16">
                  <p className="font-mono text-xs font-semibold uppercase tracking-[0.08em] text-[#ea580c]">
                    {item.eyebrow}
                  </p>
                  <h3 className="mt-5 max-w-[560px] text-3xl font-bold leading-[1.12] text-white md:text-[2.5rem]">
                    {item.title}
                  </h3>
                  <p className="mt-6 max-w-[660px] text-base leading-8 text-stone-400">
                    {item.body}
                  </p>
                  <div className="mt-8 grid gap-4">
                    {item.bullets.map(([title, body]) => (
                      <div
                        key={title}
                        className="grid gap-3 text-sm md:grid-cols-[18px_156px_minmax(0,1fr)] md:items-start"
                      >
                        <CheckCircle2 className="mt-0.5 size-4 text-[#ea580c]" />
                        <span className="font-mono text-xs font-bold uppercase text-white">
                          {title}
                        </span>
                        <span className="leading-6 text-stone-400">{body}</span>
                      </div>
                    ))}
                  </div>
                  <div className="mt-9 flex flex-col gap-3 sm:flex-row sm:items-center">
                    <Link
                      href={localizedDocsPath(lang, [...item.href])}
                      className="no-style inline-flex h-10 w-fit min-w-[136px] items-center justify-center gap-2 whitespace-nowrap rounded-md bg-[#ea580c] px-4 text-sm font-semibold text-white transition hover:bg-[#c2410c]"
                    >
                      <span className="relative z-10">{item.cta}</span>
                      <ArrowRight className="relative z-10 size-4" />
                    </Link>
                    {"secondaryCta" in item && item.secondaryHref === "templates" ? (
                      <Link
                        href={localizedDocsPath(lang, ["templates"])}
                        className="no-style inline-flex h-10 w-fit items-center justify-center gap-2 whitespace-nowrap rounded-md border border-white/10 px-4 text-sm font-semibold text-stone-100 transition hover:border-orange-500/60 hover:bg-white/5"
                      >
                        <span>{item.secondaryCta}</span>
                        <ArrowRight className="size-4" />
                      </Link>
                    ) : null}
                    <span className="inline-flex items-center gap-2 px-2 py-2 font-mono text-xs text-stone-500">
                      <Github className="size-4" />
                      one-cli/{item.command.replace("one ", "")}
                    </span>
                  </div>
                </div>
                <WorkflowCommandVisual
                  detail={item}
                  copyLabel={text.hero.copy}
                  copiedLabel={text.hero.copied}
                />
              </section>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function WorkflowCommandVisual({
  detail,
  copyLabel,
  copiedLabel,
}: {
  detail: WorkflowDetail;
  copyLabel: string;
  copiedLabel: string;
}) {
  return (
    <div className="relative flex min-h-[320px] items-start overflow-hidden border-t border-[#292524] bg-[#11100f] p-5 md:p-8 lg:min-h-full lg:border-l lg:border-t-0 lg:p-16">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_35%_30%,rgba(234,88,12,0.22),transparent_34%),linear-gradient(135deg,rgba(234,88,12,0.13),transparent_38%),repeating-linear-gradient(115deg,rgba(255,255,255,0.045)_0,rgba(255,255,255,0.045)_1px,transparent_1px,transparent_28px)] opacity-80" />
      <div className="relative w-full rounded-lg border border-orange-500/35 bg-[#0a0a0a]/94 shadow-[0_24px_80px_rgba(0,0,0,0.45)]">
        <div className="flex items-center justify-between gap-4 border-b border-[#292524] px-4 py-3">
          <div className="flex min-w-0 items-center gap-3">
            <span className="font-mono text-sm text-[#ea580c]">$</span>
            <code className="min-w-0 truncate font-mono text-sm text-stone-100">
              {detail.sample}
            </code>
          </div>
          {detail.command !== "one create" && <HomeCopyButton
            value={detail.sample}
            label={copyLabel}
            copiedLabel={copiedLabel}
            className="shrink-0 border-white/10 px-2 py-1 font-mono text-[11px] lowercase text-stone-400 hover:text-white"
          />}
        </div>
        <div className="space-y-3 px-4 py-5 font-mono text-sm leading-6">
          {detail.output.map((line, index) => (
            <div key={line} className="grid grid-cols-[18px_minmax(0,1fr)] gap-3">
              <span className={index === 0 ? "text-[#ea580c]" : "text-stone-600"}>
                {index === 0 ? ">" : "·"}
              </span>
              <span className="text-stone-300">{line}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function WhySection({ text }: { text: HomeText }) {
  return (
    <section className="border-b border-[#292524] bg-[#0a0a0a] px-5 py-16 lg:px-24 lg:py-[5.5rem]">
      <div className="mx-auto max-w-[1248px]">
        <SectionHeader
          eyebrow={text.why.eyebrow}
          title={text.why.title}
          body={text.why.body}
        />
        <div className="mt-10 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          {text.why.cards.map(({ icon: Icon, title, body, chip }) => (
            <div key={title} className="rounded-lg border border-[#292524] bg-[#1c1917] p-6">
              <div className="flex items-start gap-4">
                <span className="flex size-10 shrink-0 items-center justify-center rounded-md border border-orange-500/25 bg-orange-500/10 text-orange-400">
                  <Icon className="size-5" />
                </span>
                <div>
                  <h3 className="text-lg font-semibold text-white">{title}</h3>
                  <p className="mt-2 text-sm leading-6 text-stone-400">{body}</p>
                  <span className="mt-4 inline-flex rounded-md border border-[#292524] bg-[#292524] px-2.5 py-1.5 font-mono text-xs text-stone-300">
                    {chip}
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function JsonSection({ text }: { text: HomeText }) {
  return (
    <section className="border-b border-[#292524] bg-[#0a0a0a] px-5 py-16 lg:px-24 lg:py-[5.5rem]">
      <div className="mx-auto grid max-w-[1248px] items-center gap-12 lg:grid-cols-[0.9fr_1.1fr]">
        <div>
          <SectionHeader
            eyebrow={text.json.eyebrow}
            title={text.json.title}
            body={text.json.body}
          />
          <div className="mt-8 grid gap-3 sm:grid-cols-2">
            {text.json.cards.map(([title, body]) => (
              <div key={title} className="rounded-md border border-[#292524] bg-[#1c1917] p-4">
                <div className="font-mono text-sm text-orange-300">{title}</div>
                <div className="mt-1 text-sm text-stone-400">{body}</div>
              </div>
            ))}
          </div>
        </div>
        <div className="space-y-3">
          <CodePanel
            label={text.json.sampleLabel}
            code={text.json.sampleOutput}
            compact
            copyLabel={text.hero.copy}
            copiedLabel={text.hero.copied}
            copyValue={`${text.json.sampleLabel}\n\n${text.json.sampleOutput}`}
          />
          <p className="text-sm leading-6 text-stone-500">{text.json.automationNote}</p>
        </div>
      </div>
    </section>
  );
}

function StartWaysSection({ lang, text }: { lang: Locale; text: HomeText }) {
  const createHref = localizedDocsPath(lang, ["create"]);
  const templatesHref = localizedDocsPath(lang, ["templates-cmd"]);
  const aiGuideHref = localizedDocsPath(lang, ["ai-native"]);

  return (
    <section className="border-b border-[#292524] bg-[#0a0a0a] px-5 py-16 lg:px-24 lg:py-[5.5rem]">
      <div className="mx-auto max-w-[1248px]">
        <div className="mb-9">
          <SectionHeader
            eyebrow={text.startWays.eyebrow}
            title={text.startWays.title}
            body={text.startWays.body}
          />
        </div>
        <div className="grid overflow-hidden rounded-lg border border-[#292524] bg-[#1c1917] lg:grid-cols-3">
          <div className="flex min-h-[420px] flex-col border-b border-[#292524] p-6 lg:border-b-0 lg:border-r lg:p-7">
            <div className="flex items-center gap-2 font-mono text-xs font-semibold text-orange-400">
              <Code2 className="size-4" />
              {text.startWays.direct.label}
            </div>
            <h3 className="mt-6 text-2xl font-bold leading-tight text-white md:text-3xl">
              {text.startWays.direct.title}
            </h3>
            <p className="mt-4 text-sm leading-7 text-stone-400">
              {text.startWays.direct.body}
            </p>
            <div className="mt-6 rounded-md border border-[#292524] bg-[#0a0a0a]">
              <div className="flex items-center justify-between gap-3 border-b border-[#292524] px-4 py-3">
                <p className="font-mono text-[11px] text-stone-500">
                  {text.startWays.direct.metaLabel}
                </p>
              </div>
              <p className="px-4 py-3 font-mono text-sm text-stone-200">
                <span className="text-orange-400">$</span> {text.startWays.direct.metaValue}
              </p>
            </div>
            <div className="mt-6 space-y-3">
              {text.startWays.direct.bullets.map((item) => (
                <div key={item} className="flex items-start gap-3 text-sm leading-6 text-stone-300">
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-orange-400" />
                  <span>{item}</span>
                </div>
              ))}
            </div>
            <div className="mt-auto pt-8">
              <Link
                href={createHref}
                className="inline-flex h-10 w-fit items-center justify-center gap-2 rounded-md bg-[#ea580c] px-4 text-sm font-semibold text-white transition hover:bg-[#c2410c]"
              >
                {text.startWays.direct.cta}
                <ArrowRight className="size-4" />
              </Link>
            </div>
          </div>
          <div className="flex min-h-[420px] flex-col border-b border-[#292524] p-6 lg:border-b-0 lg:border-r lg:p-7">
            <div className="flex items-center gap-2 font-mono text-xs font-semibold text-orange-400">
              <PackageCheck className="size-4" />
              {text.startWays.template.label}
            </div>
            <h3 className="mt-6 text-2xl font-bold leading-tight text-white md:text-3xl">
              {text.startWays.template.title}
            </h3>
            <p className="mt-4 text-sm leading-7 text-stone-400">
              {text.startWays.template.body}
            </p>
            <div className="mt-6 rounded-md border border-[#292524] bg-[#0a0a0a]">
              <div className="flex items-center justify-between gap-3 border-b border-[#292524] px-4 py-3">
                <p className="font-mono text-[11px] text-stone-500">
                  {text.startWays.template.metaLabel}
                </p>
                <HomeCopyButton
                  value={text.startWays.template.metaValue}
                  label={text.hero.copy}
                  copiedLabel={text.hero.copied}
                  className="shrink-0 px-2 py-1 font-mono text-[10px] text-stone-500 hover:text-white"
                />
              </div>
              <p className="truncate px-4 py-3 font-mono text-sm text-stone-200">
                <span className="text-orange-400">$</span> {text.startWays.template.metaValue}
              </p>
            </div>
            <div className="mt-6 space-y-3">
              {text.startWays.template.bullets.map((item) => (
                <div key={item} className="flex items-start gap-3 text-sm leading-6 text-stone-300">
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-orange-400" />
                  <span>{item}</span>
                </div>
              ))}
            </div>
            <div className="mt-auto pt-8">
              <Link
                href={templatesHref}
                className="inline-flex h-10 w-fit items-center justify-center gap-2 rounded-md border border-white/10 px-4 text-sm font-semibold text-stone-100 transition hover:border-orange-500/60 hover:bg-white/5"
              >
                {text.startWays.template.cta}
                <ArrowRight className="size-4" />
              </Link>
            </div>
          </div>
          <div className="flex min-h-[420px] flex-col p-6 lg:p-7">
            <div className="flex items-center gap-2 font-mono text-xs font-semibold text-orange-400">
              <ShieldCheck className="size-4" />
              {text.startWays.ai.label}
            </div>
            <h3 className="mt-6 text-2xl font-bold leading-tight text-white md:text-3xl">
              {text.startWays.ai.title}
            </h3>
            <p className="mt-4 text-sm leading-7 text-stone-400">
              {text.startWays.ai.body}
            </p>
            <div className="mt-6 rounded-md border border-[#292524] bg-[#0a0a0a]">
              <div className="flex items-center justify-between gap-3 border-b border-[#292524] px-4 py-3">
                <p className="font-mono text-[11px] text-stone-500">
                  {text.startWays.ai.promptLabel}
                </p>
                <HomeCopyButton
                  value={text.startWays.ai.prompt}
                  label={text.hero.copy}
                  copiedLabel={text.hero.copied}
                  className="shrink-0 px-2 py-1 font-mono text-[10px] text-stone-500 hover:text-white"
                />
              </div>
              <div className="px-4 py-3 text-sm leading-6 text-stone-200">
                {text.startWays.ai.prompt}
              </div>
            </div>
            <div className="mt-6 space-y-3">
              {text.startWays.ai.bullets.map((item) => (
                <div key={item} className="flex items-start gap-3 text-sm leading-6 text-stone-300">
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-orange-400" />
                  <span>{item}</span>
                </div>
              ))}
            </div>
            <div className="mt-auto pt-8">
              <Link
                href={aiGuideHref}
                className="inline-flex h-10 w-fit items-center justify-center gap-2 rounded-md border border-white/10 px-4 text-sm font-semibold text-stone-100 transition hover:border-orange-500/60 hover:bg-white/5"
              >
                {text.startWays.ai.cta}
                <ArrowRight className="size-4" />
              </Link>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function FinalCTA({ lang, text }: { lang: Locale; text: HomeText }) {
  return (
    <section className="relative overflow-hidden border-b border-[#292524] bg-[#0a0a0a] px-5 py-[4.5rem] text-center lg:px-24 lg:py-24">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_100%,rgba(234,88,12,0.16),transparent_34rem)]" />
      <div className="relative mx-auto flex max-w-[920px] flex-col items-center gap-5">
        <h2 className="text-4xl font-bold leading-tight text-white md:text-6xl">
          {text.final.title}
        </h2>
        <p className="max-w-[680px] text-base leading-7 text-stone-300">
          {text.final.body}
        </p>
        <div className="flex flex-col gap-3 pt-3 sm:flex-row">
          <Link
            href={localizedDocsPath(lang, ["installation"])}
            className="inline-flex h-11 items-center justify-center gap-2 rounded-md bg-[#ea580c] px-5 text-sm font-semibold text-white transition hover:bg-[#c2410c]"
          >
            {text.final.install}
            <ArrowRight className="size-4" />
          </Link>
          <a
            href="https://github.com/1cli-team/one-cli"
            target="_blank"
            rel="noreferrer"
            className="inline-flex h-11 items-center justify-center gap-2 rounded-md border border-white/10 px-5 text-sm font-semibold text-stone-100 transition hover:border-orange-500/60 hover:bg-white/5"
          >
            <Github className="size-4" />
            GitHub
          </a>
        </div>
      </div>
    </section>
  );
}

function Footer({ lang, text }: { lang: Locale; text: HomeText }) {
  return (
    <footer className="bg-[#0a0a0a] px-5 py-10 lg:px-20">
      <div className="mx-auto grid max-w-[1248px] gap-9 lg:grid-cols-[minmax(260px,1fr)_minmax(0,2.8fr)]">
        <div>
          <BrandMark variant="dark" />
          <p className="mt-4 max-w-[360px] text-sm leading-6 text-stone-500">
            {text.footer.body}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-x-8 gap-y-8 sm:grid-cols-4">
          <FooterLinks
            title={text.footer.docs}
            links={[
              [text.footer.links.installation, localizedDocsPath(lang, ["installation"])],
              [text.footer.links.quickStart, localizedDocsPath(lang, ["quick-start"])],
              [text.footer.links.commandOverview, localizedDocsPath(lang, ["cli-overview"])],
            ]}
          />
          <FooterLinks
            title={text.footer.tutorials}
            links={[
              [text.footer.links.tutorialsHome, localizedTutorialsPath(lang, ["templates"])],
              [text.footer.links.firstWorkspace, localizedTutorialsPath(lang, ["first-workspace"])],
              [text.footer.links.envVars, localizedTutorialsPath(lang, ["env-vars"])],
            ]}
          />
          <FooterLinks
            title={text.footer.templates}
            links={[
              [text.footer.links.templateCommand, localizedDocsPath(lang, ["templates-cmd"])],
              [text.footer.links.templateGuide, localizedDocsPath(lang, ["templates"])],
            ]}
          />
          <FooterLinks
            title={text.footer.project}
            links={[
              ["GitHub", "https://github.com/1cli-team/one-cli"],
              [text.footer.links.releases, "https://github.com/1cli-team/one-cli/releases"],
            ]}
          />
        </div>
      </div>
      <div className="mx-auto mt-8 flex max-w-[1248px] flex-col gap-2 border-t border-[#292524] pt-5 text-xs text-stone-600 sm:flex-row sm:items-center sm:justify-between">
        <span>© 2026 torchstellar-team · MIT</span>
        <span>{text.footer.built}</span>
      </div>
    </footer>
  );
}

function FooterLinks({ title, links }: { title: string; links: [string, string][] }) {
  return (
    <nav className="flex flex-col gap-2.5">
      <h3 className="text-xs font-semibold text-stone-400">{title}</h3>
      {links.map(([label, href]) =>
        href.startsWith("http") ? (
          <a key={href} href={href} target="_blank" rel="noreferrer" className="text-sm leading-6 text-stone-500 transition hover:text-white">
            {label}
          </a>
        ) : (
          <Link key={href} href={href} className="text-sm leading-6 text-stone-500 transition hover:text-white">
            {label}
          </Link>
        ),
      )}
    </nav>
  );
}

function SectionHeader({ eyebrow, title, body }: { eyebrow: string; title: string; body: string }) {
  return (
    <div className="max-w-[780px]">
      <p className="text-xs font-semibold uppercase text-[#ea580c]">{eyebrow}</p>
      <h2 className="mt-3 text-3xl font-bold leading-tight text-white md:text-5xl">{title}</h2>
      <p className="mt-4 text-base leading-7 text-stone-400">{body}</p>
    </div>
  );
}

function CodePanel({
  label,
  code,
  compact,
  copyLabel = "Copy",
  copiedLabel = "Copied",
  copyValue,
}: {
  label: string;
  code: string;
  compact?: boolean;
  copyLabel?: string;
  copiedLabel?: string;
  copyValue?: string;
}) {
  return (
    <div className="overflow-hidden rounded-lg border border-[#292524] bg-[#0a0a0a]">
      <div className="flex items-center justify-between gap-3 border-b border-[#292524] px-4 py-3">
        <div className="flex min-w-0 items-center gap-2">
          <span className="font-mono text-xs text-stone-500">&gt;</span>
          <span className="truncate font-mono text-xs text-stone-400">{label}</span>
        </div>
        {copyValue ? <HomeCopyButton value={copyValue} label={copyLabel} copiedLabel={copiedLabel} /> : null}
      </div>
      <pre
        className={[
          "max-w-full overflow-x-auto p-4 font-mono text-sm leading-6 text-stone-200",
          compact ? "whitespace-pre-wrap break-words" : "",
        ].join(" ")}
      >
        {code}
      </pre>
    </div>
  );
}

export function localizedHomePath(lang: Locale) {
  return `/${lang}/`;
}

function alternateHomeLanguages() {
  return {
    "zh-Hans": localizedHomePath("zh"),
    en: localizedHomePath("en"),
    "x-default": localizedHomePath(defaultLocale),
  };
}
