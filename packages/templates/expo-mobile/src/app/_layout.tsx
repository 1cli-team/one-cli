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
import { Appearance, Platform, StyleSheet } from "react-native";
import { useColorScheme } from "@/hooks/useColorScheme";
import { DarkTheme, DefaultTheme, ThemeProvider } from "expo-router/react-navigation";

// Prevent the splash screen from auto-hiding before asset loading is complete.
SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const { swrConfig, loaded, onLayoutRootView } = useSetup();
  const theme = useColorScheme();

  useEffect(() => {
    if (Platform.OS !== "web") {
      Appearance.setColorScheme(theme);
    }
  }, [theme]);

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
            <StatusBar style={theme === "dark" ? "light" : "dark"} />
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
    ...StyleSheet.absoluteFill,
  },
});
