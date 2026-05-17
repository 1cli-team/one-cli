import FontAwesome from "@expo/vector-icons/FontAwesome";
import { useFonts } from "expo-font";
import { SplashScreen } from "expo-router";
import { useCallback, useEffect, useMemo, useState } from "react";

import { swrConfig } from "./swr";

export function useSetup() {
  const [layoutReady, setLayoutReady] = useState(false);

  // fonts
  const [fontsLoaded, fontsError] = useFonts({
    SpaceMono: require("@/assets/fonts/SpaceMono-Regular.ttf"),
    ...FontAwesome.font,
  });

  // loaded
  const loaded = useMemo(() => {
    return fontsLoaded;
  }, [fontsLoaded]);

  // error
  const error = useMemo(() => {
    return fontsError;
  }, [fontsError]);

  useEffect(() => {
    if (error) throw error;
  }, [error]);

  // splash
  const onLayoutRootView = useCallback(async () => {
    setLayoutReady(true);
  }, []);

  useEffect(() => {
    if (layoutReady && loaded) {
      SplashScreen.hideAsync();
    }
  }, [layoutReady, loaded]);

  return {
    swrConfig,
    loaded,
    error,
    onLayoutRootView,
  };
}
