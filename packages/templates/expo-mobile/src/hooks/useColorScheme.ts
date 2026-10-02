import { useConfigStore } from "@/store/config";

export function useColorScheme() {
  return useConfigStore((state) => state.theme);
}
