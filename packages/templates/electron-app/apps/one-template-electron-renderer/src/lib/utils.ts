import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// shadcn 标配：合并 className，处理 tailwind 冲突
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
