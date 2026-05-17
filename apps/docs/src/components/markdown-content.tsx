import type { ReactNode } from "react";

type MarkdownBlock =
  | { type: "heading"; level: 2 | 3; text: string }
  | { type: "paragraph"; text: string }
  | { type: "list"; ordered: boolean; items: string[] }
  | { type: "blockquote"; text: string }
  | { type: "code"; language?: string; code: string };

export function MarkdownContent({ content }: { content: string }) {
  return (
    <div className="one-docs-body">
      {parseMarkdown(content).map((block, index) => renderBlock(block, index))}
    </div>
  );
}

function parseMarkdown(content: string) {
  const lines = content.replace(/\r\n/g, "\n").split("\n");
  const blocks: MarkdownBlock[] = [];
  const paragraph: string[] = [];

  const flushParagraph = () => {
    if (paragraph.length === 0) return;
    blocks.push({ type: "paragraph", text: paragraph.join(" ") });
    paragraph.length = 0;
  };

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    const trimmed = line.trim();

    if (!trimmed) {
      flushParagraph();
      continue;
    }

    if (trimmed.startsWith("```")) {
      flushParagraph();
      const language = trimmed.slice(3).trim() || undefined;
      const code: string[] = [];
      index += 1;
      while (index < lines.length && !lines[index].trim().startsWith("```")) {
        code.push(lines[index]);
        index += 1;
      }
      blocks.push({ type: "code", language, code: code.join("\n") });
      continue;
    }

    const heading = trimmed.match(/^(##|###)\s+(.+)$/);
    if (heading) {
      flushParagraph();
      blocks.push({
        type: "heading",
        level: heading[1] === "##" ? 2 : 3,
        text: heading[2],
      });
      continue;
    }

    if (trimmed.startsWith(">")) {
      flushParagraph();
      const quote: string[] = [trimmed.replace(/^>\s?/, "")];
      while (lines[index + 1]?.trim().startsWith(">")) {
        index += 1;
        quote.push(lines[index].trim().replace(/^>\s?/, ""));
      }
      blocks.push({ type: "blockquote", text: quote.join(" ") });
      continue;
    }

    if (/^[-*]\s+/.test(trimmed) || /^\d+\.\s+/.test(trimmed)) {
      flushParagraph();
      const ordered = /^\d+\.\s+/.test(trimmed);
      const items = [trimmed.replace(ordered ? /^\d+\.\s+/ : /^[-*]\s+/, "")];
      while (lines[index + 1]) {
        const next = lines[index + 1].trim();
        if (ordered ? !/^\d+\.\s+/.test(next) : !/^[-*]\s+/.test(next)) break;
        index += 1;
        items.push(next.replace(ordered ? /^\d+\.\s+/ : /^[-*]\s+/, ""));
      }
      blocks.push({ type: "list", ordered, items });
      continue;
    }

    paragraph.push(trimmed);
  }

  flushParagraph();
  return blocks;
}

function renderBlock(block: MarkdownBlock, index: number) {
  switch (block.type) {
    case "heading": {
      const id = toHeadingId(block.text);
      if (block.level === 2) {
        return (
          <h2 id={id} key={index}>
            {renderInline(block.text)}
          </h2>
        );
      }
      return (
        <h3 id={id} key={index}>
          {renderInline(block.text)}
        </h3>
      );
    }
    case "paragraph":
      return <p key={index}>{renderInline(block.text)}</p>;
    case "list": {
      const ListTag = block.ordered ? "ol" : "ul";
      return (
        <ListTag key={index}>
          {block.items.map((item) => (
            <li key={item}>{renderInline(item)}</li>
          ))}
        </ListTag>
      );
    }
    case "blockquote":
      return (
        <blockquote key={index}>
          <p>{renderInline(block.text)}</p>
        </blockquote>
      );
    case "code":
      return (
        <pre data-language={block.language} key={index}>
          <code>{block.code}</code>
        </pre>
      );
  }
}

function renderInline(text: string): ReactNode[] {
  const pattern = /(`[^`]+`|\*\*[^*]+\*\*|\[([^\]]+)\]\(([^)]+)\))/g;
  const nodes: ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(text)) !== null) {
    if (match.index > lastIndex) {
      nodes.push(text.slice(lastIndex, match.index));
    }

    const token = match[0];
    if (token.startsWith("`")) {
      nodes.push(<code key={`${match.index}-code`}>{token.slice(1, -1)}</code>);
    } else if (token.startsWith("**")) {
      nodes.push(<strong key={`${match.index}-strong`}>{token.slice(2, -2)}</strong>);
    } else {
      nodes.push(
        <a href={match[3]} key={`${match.index}-link`}>
          {match[2]}
        </a>,
      );
    }

    lastIndex = pattern.lastIndex;
  }

  if (lastIndex < text.length) {
    nodes.push(text.slice(lastIndex));
  }

  return nodes;
}

function toHeadingId(text: string) {
  return text
    .toLowerCase()
    .replace(/[`*_()[\].,!?/\\:;'"，。！？、：；（）]/g, "")
    .trim()
    .replace(/\s+/g, "-");
}
