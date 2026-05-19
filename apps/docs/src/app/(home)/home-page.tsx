import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  ClipboardCheck,
  Code2,
  FileJson2,
  Github,
  Layers3,
  PackageCheck,
  Route,
  ShieldCheck,
  Sparkles,
  Wrench,
} from "lucide-react";
import {
  defaultLocale,
  localeLabels,
  localizedBlogPath,
  localizedDocsPath,
  localizedTutorialsPath,
  type Locale,
} from "@/i18n";
import { BrandMark } from "@/components/brand-mark";
import { HomeHeroCanvas } from "./hero-canvas";
import { HomeCopyButton } from "./home-template-preview";
import { WorkflowSidebarNav, type WorkflowNavIcon } from "./workflow-nav";
import {
  createPageMetadata,
  jsonLdScriptProps,
  softwareApplicationJsonLd,
  websiteJsonLd,
} from "@/lib/seo";

const installCommand = "curl -fsSL https://1cli.dev/install.sh | bash";

const homeCopy = {
  zh: {
    meta: {
      title: "One CLI | 从产品想法到能上线的项目",
      description:
        "One CLI 让 AI 从标准工程底座开始开发：网站、后台、文档、环境配置和上线流程一次准备好。",
    },
    nav: {
      docs: "文档",
      tutorials: "教程",
      templates: "模板",
      blog: "博客",
      changelog: "更新日志",
      github: "GitHub",
      start: "开始使用",
      startShort: "开始",
    },
    hero: {
      byline: "来自 · TORCHSTELLAR",
      title: (
        <>
          从一个产品想法
          <br />
          到能上线的项目。
          <br />
          一条命令开始。
        </>
      ),
      body: "让 AI 从标准工程底座开始开发：网站、后台、文档、环境配置和上线流程，一次准备好。",
      canvasAria:
        "One CLI 分层工作区画布，展示 create、manifest、templates、JSON output 和 agent skills 层级",
      install: "开始使用",
      github: "查看 GitHub",
      copy: "复制",
      copied: "已复制",
    },
    workflow: {
      eyebrow: "常用命令",
      title: "把复杂建项目，拆成几步。",
      body: "人可以直接输入命令，按提示选择；脚本和 AI 再补全模板名、项目名、--yes、-o json 这些明确参数。",
      commandBodies: [
        "新建项目或套用模板",
        "追加一个模板项目",
        "保存环境与上线账号",
        "查看模板清单",
        "打开本地配置页",
        "把 CLI 规则交给 AI",
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
        "one create 有两种开始方式：直接创建一个项目根目录，或者使用模板页生成的 preset 一次展开网站、后台、文档或库。",
      bullets: [
        ["直接创建", "one create my-app --yes 会写入基础项目文件。"],
        ["模板创建", "模板页会生成 --preset 命令，一次创建选中的组合。"],
        ["结果可查", "完成后会说明项目创建在哪里、用了哪种环境方式。"],
      ],
      explore: "查看 one create",
      details: [
        {
          command: "one create",
          navBody: "新建项目或套用模板",
          eyebrow: "one create",
          title: (
            <>
              一条命令，
              <br />
              先把项目搭起来。
            </>
          ),
          body: "one create 可以直接运行进入向导，也可以带目录名和模板页生成的 --preset，一次生成网站、后台、文档或库。",
          bullets: [
            ["交互创建", "直接输入 one create，会引导你填写目标目录和环境方式。"],
            ["模板创建", "模板页会把选择结果转成 one create my-app --preset ...。"],
            ["自动化模式", "CI 或 AI 使用 --yes 和 -o json，方便确认输出和错误。"],
          ],
          href: ["create"],
          cta: "查看 one create",
          secondaryCta: "去模板页选择",
          secondaryHref: "templates",
          sample: "one create",
          output: [
            "打开创建向导",
            "填写目标目录和环境方式",
            "生成项目说明、自动化流程和本地启动入口",
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
          body: "one add 只在已有 One 项目里使用。你可以直接输入 one add 进入交互式选择，也可以在脚本里写明模板名、项目名和部署方式。",
          bullets: [
            ["交互添加", "直接输入 one add，会让你选择模板、项目名和可选部署方式。"],
            ["自动化模式", "CI 或 AI 才需要写 one add nextjs-app --name web --yes。"],
            ["同步默认值", "会按模板补齐运行、部署和 AI 说明文件。"],
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
          command: "one configure",
          navBody: "保存环境与上线账号",
          eyebrow: "one configure",
          title: (
            <>
              把密钥和上线信息，
              <br />
              放在安全位置。
            </>
          ),
          body: "one configure 保存本机要用的环境、部署和镜像账号。直接运行会进入配置向导；脚本里才需要写 env/infisical、deploy/*、container/docker 这些完整路径。",
          bullets: [
            ["本机保存", "配置写到 ~/.config/one；密钥文件只有本人可读。"],
            ["远程环境", "远程 env 指 Infisical 云服务或你自己的 Infisical；不需要额外启动 One CLI 服务。"],
            ["后续复用", "one env、one run、one deploy 会自动读取当前配置。"],
          ],
          href: ["cli-overview"],
          cta: "查看 CLI 参考",
          sample: "one configure",
          output: [
            "打开 env / deploy / container 配置向导",
            "保存本机配置档和凭据",
            "供 one env、one run、one deploy 后续读取",
          ],
        },
        {
          command: "one templates",
          navBody: "查看模板清单",
          eyebrow: "one templates",
          title: (
            <>
              选一个起点，
              <br />
              不用猜模板名。
            </>
          ),
          body: "one templates 会在终端里列出当前内置模板清单，适合人先看有哪些后端、前端、文档、移动端、桌面端和库模板。",
          bullets: [
            ["当前 10 个", "包含后端、前端、移动端、桌面端、文档和 TS / Go 库。"],
            ["人类可读", "直接运行会显示模板名、分类和用途。"],
            ["两种用途", "模板名给 one add 用；模板页组合给 one create 用。"],
          ],
          href: ["templates-cmd"],
          cta: "查看 templates",
          secondaryCta: "打开模板页",
          secondaryHref: "templates",
          sample: "one templates",
          output: [
            "列出可以直接使用的模板",
            "显示模板 ID、分类和用途",
            "复制模板名给 one add 使用",
          ],
        },
        {
          command: "one serve",
          navBody: "打开本地配置页",
          eyebrow: "one serve",
          title: (
            <>
              用浏览器，
              <br />
              手动填写敏感配置。
            </>
          ),
          body: "one serve 启动只绑定 127.0.0.1 的本地配置页，用来编辑 one configure 管理的配置档。它适合让人手动录入密钥，避免把明文暴露给 agent 对话。",
          bullets: [
            ["仅本机访问", "默认随机端口，只允许本机浏览器访问，不会直接暴露到局域网。"],
            ["一次性 token", "启动时打印带 token 的 URL，进程退出后旧链接失效。"],
            ["远程机器", "在远端跑 one serve，再用 ssh -L 做端口转发访问。"],
          ],
          href: ["serve"],
          cta: "查看 one serve",
          sample: "one serve",
          output: [
            "打开本机浏览器里的配置页",
            "URL 自带一次性访问 token",
            "按 Ctrl-C 退出，旧链接失效",
          ],
        },
        {
          command: "one skills",
          navBody: "把 CLI 规则交给 AI",
          eyebrow: "one skills",
          title: (
            <>
              让 AI 助手，
              <br />
              知道项目怎么做。
            </>
          ),
          body: "one skills install 会把 One CLI 自带的工作流安装或刷新到本机 AI 编程工具里。AI 先读项目说明，再按场景新建、追加、迁移或补依赖。直接运行会让你选择目标；--yes 才会安装到所有检测到的工具。",
          bullets: [
            ["创建或迁移", "按你的描述新建项目，或把已有项目纳入 One CLI 管理。"],
            ["可交互选择", "直接运行会让你选择 AI 工具；--yes 会安装到检测到的所有工具。"],
            ["自动补依赖", "按项目类型处理依赖：JS/TS 看包管理器，Go 看各子项目。"],
          ],
          href: ["ai-native"],
          cta: "查看 AI 指南",
          sample: "one skills install",
          output: [
            "检测本机 AI 编程工具",
            "选择要安装到哪些工具",
            "同步命令、模板和依赖规则",
          ],
        },
      ],
    },
    why: {
      eyebrow: "AGENT-READY WORKSPACE",
      title: "让 agent 先读事实，再动项目。",
      body: "One CLI 不是另一个聊天入口。它把 skills、manifest、AI 指南和结构化错误组织成一套工程契约，让 Codex、Claude Code、Cursor 这类 AI 编程工具能先确认项目事实，再选择命令、修改代码和处理失败。",
      cards: [
        {
          icon: Sparkles,
          title: "Skills 说明怎么操作",
          body: "`one skills install` 把 One CLI 的工作流安装到本机 AI 编程工具。agent 可以按说明创建 workspace、追加模板、迁移项目或补齐依赖，而不是每次从 prompt 里猜流程。",
          chip: "one skills install",
        },
        {
          icon: ClipboardCheck,
          title: "Manifest 记录项目事实",
          body: "`one.manifest.json` 记录项目、模板、toolchain 和环境边界。agent 先读这份事实，再决定进入哪个子项目、调用哪个命令。",
          chip: "one.manifest.json",
        },
        {
          icon: Wrench,
          title: "AI 指南跟着模板更新",
          body: "成功执行 `one add` 后，One CLI 会生成或更新 `AGENTS.md` / `CLAUDE.md` 的受管理内容块，把模板的工程规则写进 workspace；已有自定义内容不会被强行覆盖。",
          chip: "AGENTS.md / CLAUDE.md",
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
      eyebrow: "所有命令都有下一步",
      title: "命令没跑通，不用自己猜。",
      body: "从创建、添加，到运行、上线，One CLI 遇到问题时都会先说明卡在哪里，再给你下一步可以做什么。新手能直接照着走，AI 工具也能接着处理。",
      cards: [
        ["创建项目", "目录不对或目标已存在，会告诉你怎么继续"],
        ["添加项目", "模板、名称、位置不对，会给出修复命令"],
        ["运行和上线", "缺少配置或工具，会提示先补什么"],
        ["接给 AI", "同样结果可以输出 JSON，方便工具处理"],
      ],
      sampleLabel: "one add --name web --yes",
      sampleOutput: `未检测到 One CLI 项目，请在项目根目录执行。
  - 当前目录缺少 one.manifest.json；
    请先创建工作区，或 cd 到已有工作区：
    one create <dir>`,
      automationNote: "这只是其中一个例子。创建、添加、运行、上线等命令遇到问题时，也会用同样的方式给出原因和下一步。",
    },
    templateBuilder: {
      eyebrow: "模板构建器",
      title: "模板选择在模板页完成。",
      body: "首页只给入口：进入模板页后，可以直接选现成示例，也可以自定义组合，再复制 One CLI 命令自己运行或交给 AI 执行。",
      action: "进入模板页",
    },
    startWays: {
      eyebrow: "开始方式",
      title: "三种开始方式。",
      body: "想最快开始，直接运行 one create；想先挑组合，去模板页；想让 AI 接手，就先安装官方规则。",
      direct: {
        label: "方式一",
        title: "直接运行 one create",
        body: "适合先搭一个项目根目录。直接输入 one create，按提示填写目录和环境；之后再用 one add 继续添加网站、后台或文档。",
        bullets: ["不需要先记参数", "按提示填写目标目录和环境方式", "创建后再用 one add 继续添加项目"],
        cta: "查看 one create",
        metaLabel: "可复制命令",
        metaValue: "one create",
      },
      template: {
        label: "方式二",
        title: "去模板页选择",
        body: "模板页已经按网站、后台、文档、移动端和桌面端整理好。选现成示例，或自定义组合，再复制生成的 One CLI 命令。",
        bullets: ["按产品类型浏览", "现成示例和自定义组合都支持", "复制命令后自己运行或交给 AI"],
        cta: "进入模板页",
        metaLabel: "模板页入口",
        metaValue: "1cli.dev /zh/templates/",
      },
      ai: {
        label: "方式三",
        title: "交给 AI 执行",
        body: "先运行 one skills install，把 One CLI 的规则装进 Codex、Claude Code、Cursor 等本地 AI 编程工具。",
        bullets: ["直接描述要做什么", "AI 按 One CLI 规则创建或迁移项目", "按项目类型补依赖"],
        cta: "查看 AI 指南",
        promptLabel: "给 agent 的一句话",
        prompt: "请使用 One CLI，帮我创建一个名为 media-stack 的移动端项目，并安装依赖。",
      },
    },
    skill: {
      eyebrow: "官方 AI 说明",
      title: "安装 CLI，也把项目规则交给 agent。",
      body: "`one skills install` 会把 One CLI 自带的工作流装到 Codex、Claude Code、Cursor 等本地 AI 编程工具。agent 先读 `one.manifest.json`，再按场景新建、追加、迁移项目，并按 JS / Go 的规则补依赖。",
      promptLabel: "给 agent 的一句话",
      bullets: [
        "创建新项目：按你的描述生成项目结构",
        "迁移到 One CLI：把现有项目纳入 One CLI 管理",
        "自动补依赖：按项目类型安装需要的依赖",
      ],
      prompt: "请使用 One CLI，帮我创建一个名为 media-stack 的移动端项目，并安装依赖。",
      result: (
        <>
          ✓ 已选择移动端模板
          <br />
          ✓ 已创建 media-stack 项目
          <br />
          ✓ 已生成项目结构
          <br />
          ✓ 已安装依赖
        </>
      ),
    },
    final: {
      title: "停止手工拼工程底座。",
      body: "安装一个 Go 二进制，选择模板，把清晰的项目上下文交给人和 AI agent。",
      install: "开始使用",
    },
    footer: {
      body: "面向 agent 的项目脚手架与治理 CLI。",
      docs: "文档",
      tutorials: "教程",
      templates: "模板",
      project: "项目",
      links: {
        installation: "安装",
        quickStart: "一条命令开始",
        tutorialsHome: "教程总览",
        firstWorkspace: "第一个工作区",
        envVars: "配置环境变量",
        skillsInstall: "安装 Skill 到 Agent",
        templateBuilder: "模板页面",
        templateGuide: "怎么选模板",
        commandOverview: "命令总览",
        releases: "版本发布",
        changelog: "更新日志",
      },
      built: "基于 Next.js、Fumadocs 和 One CLI 构建。",
    },
  },
  en: {
    meta: {
      title: "One CLI | From product idea to launch-ready project",
      description:
        "One CLI gives AI a real product foundation: website, backend, docs, environment config, and deployment flow.",
    },
    nav: {
      docs: "Docs",
      tutorials: "Tutorials",
      templates: "Templates",
      blog: "Blog",
      changelog: "Changelog",
      github: "GitHub",
      start: "Get started",
      startShort: "Start",
    },
    hero: {
      byline: "BY · TORCHSTELLAR",
      title: (
        <>
          From product idea to
          <br />
          launch-ready project.
          <br />
          Start with one command.
        </>
      ),
      body: "Give AI a real product foundation: website, backend, docs, environment config, and deployment flow, ready from day one.",
      canvasAria:
        "One CLI layered workspace canvas showing create, manifest, templates, JSON output, and bundled skill layers",
      install: "Start building",
      github: "View on GitHub",
      copy: "copy",
      copied: "copied",
    },
    workflow: {
      eyebrow: "Common commands",
      title: "Complex project setup, split into simple steps.",
      body: "Humans can run commands directly and follow prompts; scripts and AI can add template names, project names, --yes, and -o json.",
      commandBodies: [
        "create or apply templates",
        "add a template project",
        "save env and launch accounts",
        "inspect the template list",
        "open the local config UI",
        "give CLI rules to AI",
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
        "one create has two starting paths: create a project root directly, or apply a preset from the templates page to scaffold selected apps and libraries in one run.",
      bullets: [
        ["DIRECT CREATE", "one create my-app --yes writes the base project files."],
        ["PRESET CREATE", "The templates page generates a --preset command for the selected stack."],
        ["CHECKABLE", "The result tells you where the project was created and which env mode it uses."],
      ],
      explore: "Explore one create",
      details: [
        {
          command: "one create",
          navBody: "create or apply templates",
          eyebrow: "ONE CREATE",
          title: (
            <>
              Start a project
              <br />
              with one command.
            </>
          ),
          body: "one create can run directly as a prompt, or run with a directory and the --preset generated by the templates page to scaffold selected web apps, APIs, docs, and libraries.",
          bullets: [
            ["PROMPTED CREATE", "Run one create by itself to choose the target directory and environment mode."],
            ["PRESET CREATE", "The templates page turns your selection into one create my-app --preset ... ."],
            ["AUTOMATION", "CI and AI use --yes and -o json for predictable output and errors."],
          ],
          href: ["create"],
          cta: "Explore one create",
          secondaryCta: "Open templates",
          secondaryHref: "templates",
          sample: "one create",
          output: [
            "opens the create wizard",
            "asks for target directory and env mode",
            "writes project metadata, automation, and local dev entry",
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
          body: "one add is for an existing One project. You can run one add directly to use the picker, or pass the template name, project name, and deploy option in scripts.",
          bullets: [
            ["PROMPTED ADD", "Run one add to choose the template, project name, and optional deploy backend."],
            ["AUTOMATION", "CI and AI use one add nextjs-app --name web --yes."],
            ["SYNC DEFAULTS", "Templates can add the expected run, deploy, and AI guide files."],
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
          command: "one configure",
          navBody: "save env and launch accounts",
          eyebrow: "ONE CONFIGURE",
          title: (
            <>
              Keep secrets
              <br />
              in the right place.
            </>
          ),
          body: "one configure saves the env, deploy, and container accounts this machine can use. Run it directly for the wizard; scripts use full paths such as env/infisical, deploy/*, or container/docker.",
          bullets: [
            ["LOCAL FILES", "Writes under ~/.config/one; secret files are readable only by you."],
            ["REMOTE ENV", "Remote env means Infisical Cloud or your own Infisical. You do not run a separate One CLI service."],
            ["REUSED LATER", "one env, one run, and one deploy read the default profile automatically."],
          ],
          href: ["cli-overview"],
          cta: "Explore CLI reference",
          sample: "one configure",
          output: [
            "open env / deploy / container profile prompts",
            "save local profile and credential files",
            "feed later one env, one run, and one deploy commands",
          ],
        },
        {
          command: "one templates",
          navBody: "inspect the template list",
          eyebrow: "ONE TEMPLATES",
          title: (
            <>
              Pick a starting point
              <br />
              without guessing.
            </>
          ),
          body: "one templates prints the bundled template list in a human-readable terminal format, so people can see the available backend, frontend, docs, mobile, desktop, and library templates first.",
          bullets: [
            ["TEN TODAY", "Backend, frontend, mobile, desktop, docs, and TS / Go libraries are included."],
            ["HUMAN READABLE", "Bare output shows template names, categories, and what each template is for."],
            ["TWO USES", "Template names go to one add; template-page combinations go to one create."],
          ],
          href: ["templates-cmd"],
          cta: "Explore templates",
          secondaryCta: "Open template page",
          secondaryHref: "templates",
          sample: "one templates",
          output: [
            "lists templates you can use directly",
            "shows template ID, category, and purpose",
            "copy a template name for one add",
          ],
        },
        {
          command: "one serve",
          navBody: "open the local config UI",
          eyebrow: "ONE SERVE",
          title: (
            <>
              Fill sensitive config
              <br />
              in your browser.
            </>
          ),
          body: "one serve starts a loopback-only local config page for the same profiles managed by one configure. It is meant for human secret entry, so cleartext credentials do not need to pass through an agent chat.",
          bullets: [
            ["LOCAL ONLY", "Uses a random local port by default and only accepts your own browser, not LAN access."],
            ["ONE TOKEN", "The printed URL includes a session token; old URLs stop working after the process exits."],
            ["REMOTE HOSTS", "Run one serve on the remote machine, then use ssh -L port forwarding from your laptop."],
          ],
          href: ["serve"],
          cta: "Explore one serve",
          sample: "one serve",
          output: [
            "opens the local config page in your browser",
            "URL includes a one-time access token",
            "Ctrl-C stops the process and expires old links",
          ],
        },
        {
          command: "one skills",
          navBody: "give CLI rules to AI",
          eyebrow: "ONE SKILLS",
          title: (
            <>
              Help AI understand
              <br />
              how this project works.
            </>
          ),
          body: "one skills install installs or refreshes One CLI's bundled workflows for local AI coding tools. Agents read the project facts first, then create, add, migrate, or bootstrap dependencies by context. Bare install lets you choose tools; --yes installs to every detected tool.",
          bullets: [
            ["CREATE OR MIGRATE", "Create a new project from your description, or bring an existing one under One CLI management."],
            ["CHOOSABLE", "Bare install opens a picker; --yes installs to every detected tool."],
            ["BOOTSTRAP DEPS", "Handle dependencies by project type: JS/TS uses the package manager; Go uses each subproject."],
          ],
          href: ["ai-native"],
          cta: "Explore AI guide",
          sample: "one skills install",
          output: [
            "detects local AI coding tools",
            "lets you choose where to install",
            "syncs command, template, and dependency rules",
          ],
        },
      ],
    },
    why: {
      eyebrow: "AGENT-READY WORKSPACE",
      title: "Let agents read facts before touching the project.",
      body: "One CLI is not another chat surface. It organizes skills, manifests, AI guides, and structured errors into one engineering contract so Codex, Claude Code, and Cursor can confirm project facts before choosing commands, editing code, or recovering from failures.",
      cards: [
        {
          icon: Sparkles,
          title: "Skills explain the workflow",
          body: "`one skills install` installs One CLI workflows into local AI coding tools. Agents can create workspaces, add templates, migrate projects, or bootstrap dependencies from instructions instead of guessing from a prompt.",
          chip: "one skills install",
        },
        {
          icon: ClipboardCheck,
          title: "Manifest records project facts",
          body: "`one.manifest.json` records projects, templates, toolchains, and environment boundaries. Agents read those facts before choosing a subproject or command.",
          chip: "one.manifest.json",
        },
        {
          icon: Wrench,
          title: "AI guides follow templates",
          body: "After a successful `one add`, One CLI creates or updates managed `AGENTS.md` / `CLAUDE.md` blocks with template-specific engineering rules. Existing custom content is not overwritten blindly.",
          chip: "AGENTS.md / CLAUDE.md",
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
      eyebrow: "Next steps for every command",
      title: "When a command fails, you do not have to guess.",
      body: "From creating and adding projects to running and shipping them, One CLI explains where you are stuck and what to do next. Beginners can follow the message, and AI tools can continue from the same result.",
      cards: [
        ["Create projects", "Wrong folder or existing target: it tells you how to continue"],
        ["Add projects", "Template, name, or location issues come with a fix command"],
        ["Run and ship", "Missing config or tools are called out before you continue"],
        ["Connect AI", "The same result can be emitted as JSON for tools"],
      ],
      sampleLabel: "one add --name web --yes",
      sampleOutput: `No One CLI workspace found. Run this from a workspace root.
  - This folder is missing one.manifest.json.
    Create a workspace first, or cd into an existing one:
    one create <dir>`,
      automationNote: "This is one example. Create, add, run, and ship commands use the same pattern when something needs attention.",
    },
    templateBuilder: {
      eyebrow: "Template builder",
      title: "Template choices happen on the templates page.",
      body: "The homepage is only the entry point. Open the templates page to choose a ready example or build a custom mix, then copy the One CLI command to run yourself or hand to AI.",
      action: "Open templates",
    },
    startWays: {
      eyebrow: "Start here",
      title: "Three ways to start.",
      body: "Run one create for the fastest path, open templates to choose a stack first, or install the official AI rules and hand the task to an agent.",
      direct: {
        label: "Option one",
        title: "Run one create",
        body: "Use this when you want a project root first. Run one create, follow the prompts for directory and env mode, then use one add to keep adding web apps, APIs, or docs.",
        bullets: ["No flags to memorize first", "Prompts ask for directory and env mode", "Use one add later for more projects"],
        cta: "Explore one create",
        metaLabel: "Copyable command",
        metaValue: "one create",
      },
      template: {
        label: "Option two",
        title: "Open the templates page",
        body: "Templates are already grouped by websites, APIs, docs, mobile apps, and desktop apps. Pick a ready example or build a custom mix, then copy the generated One CLI command.",
        bullets: ["Browse by product type", "Use ready examples or custom mixes", "Run the command yourself or hand it to AI"],
        cta: "Open templates",
        metaLabel: "Template page",
        metaValue: "1cli.dev /en/templates/",
      },
      ai: {
        label: "Option three",
        title: "Hand it to AI",
        body: "Run one skills install first to install One CLI rules into local AI coding tools such as Codex, Claude Code, and Cursor.",
        bullets: ["Describe what you want to build", "AI creates or migrates projects using One CLI rules", "Dependencies are handled by project type"],
        cta: "Explore AI guide",
        promptLabel: "one-line agent prompt",
        prompt: "Please use One CLI to create a mobile project named media-stack and install dependencies.",
      },
    },
    skill: {
      eyebrow: "Bundled AI instructions",
      title: "Install the CLI and hand agents the project rules.",
      body: "`one skills install` installs One CLI's bundled workflows into local AI coding tools like Codex, Claude Code, and Cursor. Agents read `one.manifest.json`, then create, add, migrate, and bootstrap JS / Go dependencies by context.",
      promptLabel: "one-line agent prompt",
      bullets: [
        "Create new projects from your description",
        "Migrate existing projects to One CLI",
        "Install dependencies by project type",
      ],
      prompt: "Please use One CLI to create a mobile project named media-stack and install dependencies.",
      result: (
        <>
          ✓ selected the mobile template
          <br />
          ✓ created media-stack
          <br />
          ✓ generated the project structure
          <br />
          ✓ installed dependencies
        </>
      ),
    },
    final: {
      title: "Stop hand-assembling the project foundation.",
      body: "Install one Go binary, choose templates, and hand clear project context to humans and AI agents.",
      install: "Start building",
    },
    footer: {
      body: "Agent-native project scaffolding and governance CLI.",
      docs: "Docs",
      tutorials: "Tutorials",
      templates: "Templates",
      project: "Project",
      links: {
        installation: "Installation",
        quickStart: "Start with one command",
        tutorialsHome: "Tutorials home",
        firstWorkspace: "First workspace",
        envVars: "Configure env vars",
        skillsInstall: "Install skill to agent",
        templateBuilder: "Templates page",
        templateGuide: "Choose templates",
        commandOverview: "Command overview",
        releases: "Releases",
        changelog: "Changelog",
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
  "one configure",
  "one templates",
  "one serve",
  "one skills",
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
    [text.nav.templates, localizedTemplatesPath(lang)],
    [text.nav.blog, localizedBlogPath(lang)],
    [text.nav.changelog, "https://github.com/1cli-team/one-cli/blob/master/CHANGELOG.md"],
  ] as const;

  return (
    <header className="sticky top-0 z-30 border-b border-[#292524] bg-[#0a0a0a]/92 backdrop-blur-xl">
      <div className="mx-auto flex h-16 w-full max-w-[1440px] items-center justify-between gap-4 px-5 lg:px-20">
        <Link href={localizedHomePath(lang)} className="inline-flex items-center" aria-label="One CLI home">
          <BrandMark variant="dark" />
        </Link>
        <nav className="hidden items-center gap-7 text-sm lg:flex">
          {navItems.map(([label, href]) => {
            const external = href.startsWith("http");
            if (external) {
              return (
                <a key={label} href={href} target="_blank" rel="noreferrer" className="text-stone-400 transition hover:text-white">
                  {label}
                </a>
              );
            }
            return (
              <Link key={label} href={href} className="text-stone-400 transition hover:text-white">
                {label}
              </Link>
            );
          })}
        </nav>
        <div className="flex items-center gap-2">
          <HomeLanguageSwitcher lang={lang} />
          <a
            href="https://github.com/1cli-team/one-cli"
            target="_blank"
            rel="noreferrer"
            className="hidden size-9 items-center justify-center rounded-md text-stone-300 transition hover:bg-white/5 hover:text-white sm:inline-flex"
            aria-label={text.nav.github}
          >
            <Github className="size-5" />
          </a>
          <Link
            href={localizedDocsPath(lang, ["installation"])}
            className="inline-flex h-9 shrink-0 items-center gap-2 rounded-md bg-[#ea580c] px-3.5 text-sm font-semibold text-white transition hover:bg-[#c2410c]"
          >
            <span className="hidden sm:inline">{text.nav.start}</span>
            <span className="sm:hidden">{text.nav.startShort}</span>
            <ArrowRight className="size-4" />
          </Link>
        </div>
      </div>
    </header>
  );
}

function HomeLanguageSwitcher({ lang }: { lang: Locale }) {
  return (
    <div
      aria-label="Language switcher"
      className="hidden h-9 items-center rounded-md border border-white/10 bg-white/[0.03] p-1 sm:inline-flex"
    >
      {(["zh", "en"] as const).map((locale) => (
        <Link
          aria-current={locale === lang ? "true" : undefined}
          className={[
            "inline-flex h-7 items-center rounded px-2.5 text-xs font-medium transition",
            locale === lang
              ? "bg-white text-[#0a0a0a]"
              : "text-stone-400 hover:text-white",
          ].join(" ")}
          href={localizedHomePath(locale)}
          key={locale}
        >
          {localeLabels[locale]}
        </Link>
      ))}
    </div>
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
          <div className="flex w-full max-w-[580px] min-w-0 items-center gap-2 rounded-lg border border-[#292524] bg-[#1c1917] px-3 py-2">
            <span className="font-mono text-xs text-[#ea580c]">$</span>
            <code className="min-w-0 flex-1 truncate font-mono text-xs text-stone-100">
              {installCommand}
            </code>
            <HomeCopyButton
              value={installCommand}
              label={text.hero.copy}
              copiedLabel={text.hero.copied}
              className="shrink-0 border-transparent px-1.5 py-1 font-mono text-[10px] lowercase text-stone-500 hover:text-white"
            />
          </div>
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
                        href={localizedTemplatesPath(lang)}
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
          <HomeCopyButton
            value={detail.sample}
            label={copyLabel}
            copiedLabel={copiedLabel}
            className="shrink-0 border-white/10 px-2 py-1 font-mono text-[11px] lowercase text-stone-400 hover:text-white"
          />
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
  const templatesHref = localizedTemplatesPath(lang);
  const templateUrl = `https://1cli.dev${templatesHref}`;
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
                <HomeCopyButton
                  value={text.startWays.direct.metaValue}
                  label={text.hero.copy}
                  copiedLabel={text.hero.copied}
                  className="shrink-0 px-2 py-1 font-mono text-[10px] text-stone-500 hover:text-white"
                />
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
                  value={templateUrl}
                  label={text.hero.copy}
                  copiedLabel={text.hero.copied}
                  className="shrink-0 px-2 py-1 font-mono text-[10px] text-stone-500 hover:text-white"
                />
              </div>
              <p className="truncate px-4 py-3 font-mono text-sm text-stone-200">
                <span className="text-orange-400">1cli.dev</span>
                {templatesHref}
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
              [text.footer.links.tutorialsHome, localizedTutorialsPath(lang)],
              [text.footer.links.firstWorkspace, localizedTutorialsPath(lang, ["first-workspace"])],
              [text.footer.links.envVars, localizedTutorialsPath(lang, ["env-vars"])],
              [text.footer.links.skillsInstall, localizedTutorialsPath(lang, ["skills-install"])],
            ]}
          />
          <FooterLinks
            title={text.footer.templates}
            links={[
              [text.footer.links.templateBuilder, localizedTemplatesPath(lang)],
              [text.footer.links.templateGuide, localizedDocsPath(lang, ["templates"])],
            ]}
          />
          <FooterLinks
            title={text.footer.project}
            links={[
              ["GitHub", "https://github.com/1cli-team/one-cli"],
              [text.nav.blog, localizedBlogPath(lang)],
              [text.footer.links.releases, "https://github.com/1cli-team/one-cli/releases"],
              [text.footer.links.changelog, "https://github.com/1cli-team/one-cli/blob/master/CHANGELOG.md"],
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

function localizedTemplatesPath(lang: Locale) {
  return `/${lang}/templates/`;
}

function alternateHomeLanguages() {
  return {
    "zh-Hans": localizedHomePath("zh"),
    en: localizedHomePath("en"),
    "x-default": localizedHomePath(defaultLocale),
  };
}
