import { Pressable, StyleSheet } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { ThemedText } from "@/components/ThemedText";
import { ThemedView } from "@/components/ThemedView";
import { useConfigStore } from "@/store/config";

export default function HomeScreen() {
  const theme = useConfigStore((state) => state.theme);
  const setTheme = useConfigStore((state) => state.setTheme);
  return (
    <ThemedView style={styles.screen}>
      <SafeAreaView style={styles.content} edges={["bottom", "left", "right"]}>
        <ThemedText type="title">欢迎 / Welcome</ThemedText>
        <ThemedText>从这里开始构建你的应用。Start building your application here.</ThemedText>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="切换主题 / Change theme"
          onPress={() =>
            setTheme(theme === "system" ? "dark" : theme === "dark" ? "light" : "system")
          }
        >
          <ThemedText>主题 / Theme: {theme}</ThemedText>
        </Pressable>
      </SafeAreaView>
    </ThemedView>
  );
}
const styles = StyleSheet.create({
  screen: { flex: 1 },
  content: { flex: 1, padding: 24, gap: 20, justifyContent: "center" },
});
