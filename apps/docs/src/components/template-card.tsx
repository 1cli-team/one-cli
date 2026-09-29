import { Link } from "next-view-transitions";
import { Boxes, Globe2, Server } from "lucide-react";
import type { TemplateKind, TemplateMeta } from "@/data/templates";
import { TemplateCoverImage } from "@/components/template-cover-image";

export const templateKindLabels = {
  frontend: { zh: "前端应用", en: "Frontend app" },
  backend: { zh: "后端服务", en: "Backend service" },
  library: { zh: "共享库", en: "Shared library" },
};
const categoryIcons = { frontend: Globe2, backend: Server, library: Boxes } satisfies Record<TemplateKind, typeof Boxes>;

export function TemplateCard({ template, lang }: { template: TemplateMeta; lang: "zh" | "en" }) {
  const Icon = categoryIcons[template.kind];
  return (
    <Link
      href={`/${lang}/templates/${template.id}/`}
      id={template.id}
      style={{ viewTransitionName: `example-card-${template.id}` }}
      className="group focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-orange-600 flex scroll-mt-24 flex-col overflow-hidden rounded-xl border border-stone-200 bg-white shadow-[0_1px_2px_rgba(10,10,10,0.04)] transition hover:-translate-y-0.5 hover:border-stone-300 hover:shadow-[0_4px_16px_rgba(10,10,10,0.06)]"
    >
      <div className="relative aspect-[16/10] w-full overflow-hidden bg-stone-100">
        <TemplateCoverImage
          src={template.cover}
          alt=""
          className="object-cover transition group-hover:scale-[1.02]"
          sizes="(min-width: 1024px) 370px, (min-width: 640px) 50vw, 100vw"
        />
        <div className="absolute left-3 top-3 inline-flex items-center gap-1.5 rounded-full bg-white/90 px-2.5 py-1 text-xs font-medium text-stone-700 shadow-[0_1px_2px_rgba(10,10,10,0.06)] backdrop-blur">
          <Icon className="size-3.5" aria-hidden="true" />
          {templateKindLabels[template.kind][lang]}
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-3 p-5">
        <h2 className="text-lg font-semibold text-stone-900">{template.title[lang]}</h2>
        <p className="text-sm leading-6 text-stone-600">{template.tagline[lang]}</p>
      </div>
    </Link>
  );
}
