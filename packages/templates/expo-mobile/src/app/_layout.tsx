import NetInfo from "@react-native-community/netinfo";
import { Stack } from "expo-router";
import { DarkTheme, DefaultTheme, ThemeProvider } from "expo-router/react-navigation";
import { StatusBar } from "expo-status-bar";
import { AppState, Platform } from "react-native";
import { SafeAreaProvider, initialWindowMetrics } from "react-native-safe-area-context";
import { SWRConfig } from "swr";
import { createNativeSWRConfig } from "@/hooks/setup/swr";
import { useColorScheme } from "@/hooks/useColorScheme";
import { fetcher } from "@/lib/axios";

const swrConfig = {
  fetcher,
  ...(Platform.OS === "web" ? {} : createNativeSWRConfig(AppState, NetInfo)),
};
export default function RootLayout() {
  const theme = useColorScheme();
  return (
    <SafeAreaProvider initialMetrics={initialWindowMetrics}>
      <SWRConfig value={swrConfig}>
        <ThemeProvider value={theme === "dark" ? DarkTheme : DefaultTheme}>
          <Stack>
            <Stack.Screen name="index" options={{ title: "One CLI" }} />
          </Stack>
          <StatusBar style={theme === "dark" ? "light" : "dark"} />
        </ThemeProvider>
      </SWRConfig>
    </SafeAreaProvider>
  );
}
