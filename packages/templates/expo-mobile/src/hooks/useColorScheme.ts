import { useColorScheme as useSystemColorScheme } from "react-native";
import { useConfigStore } from "@/store/config";

export function useColorScheme() {
  const systemTheme = useSystemColorScheme();
  const theme = useConfigStore((state) => state.theme);
  return theme === "system" ? (systemTheme === "dark" ? "dark" : "light") : theme;
}
