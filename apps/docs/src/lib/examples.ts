import { examples, type Example, type ExampleCategory } from "@/data/examples";

export function getExamples(): Example[] {
  return examples;
}

export function getExample(id: string): Example | null {
  return examples.find((e) => e.id === id) ?? null;
}

export function getExamplesByCategory(
  category: ExampleCategory | "all",
): Example[] {
  if (category === "all") return examples;
  return examples.filter((e) => e.category === category);
}

export function getExampleIds(): string[] {
  return examples.map((e) => e.id);
}
