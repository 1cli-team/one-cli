import React from "react";
import { Stack } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { StatusBar } from "expo-status-bar";
import { useEffect } from "react";
import "react-native-reanimated";
import { GestureHandlerRootView } from "react-native-gesture-handler";
import { initialWindowMetrics, SafeAreaProvider } from "react-native-safe-area-context";
import { SWRConfig } from "swr";
import { useSetup } from "@/hooks/setup";
import { StyleSheet } from "react-native";
import "../../global.css";
import { useColorScheme } from "nativewind";
import { configSelector, useConfigStore } from "@/store/config";
import { useShallow } from "zustand/react/shallow";
import { DarkTheme, DefaultTheme, ThemeProvider } from "@react-navigation/native";

// Prevent the splash screen from auto-hiding before asset loading is complete.
SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const { swrConfig, loaded, onLayoutRootView } = useSetup();
  const { theme } = useConfigStore(useShallow(configSelector));
  const { setColorScheme } = useColorScheme();
  setColorScheme(theme);

  useEffect(() => {
    if (loaded) {
      SplashScreen.hideAsync();
    }
  }, [loaded]);

  if (!loaded) {
    return null;
  }

  return (
    <SafeAreaProvider initialMetrics={initialWindowMetrics}>
      <SWRConfig value={swrConfig}>
        <GestureHandlerRootView style={[styles.container]} onLayout={onLayoutRootView}>
          <ThemeProvider value={theme === "dark" ? DarkTheme : DefaultTheme}>
            <Stack>
              <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
              <Stack.Screen name="+not-found" />
            </Stack>
            <StatusBar style="auto" />
          </ThemeProvider>
        </GestureHandlerRootView>
      </SWRConfig>
    </SafeAreaProvider>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  backgroundImage: {
    ...StyleSheet.absoluteFillObject,
  },
});
