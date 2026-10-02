import { Link, Stack } from "expo-router";
import { StyleSheet } from "react-native";
import { ThemedText } from "@/components/ThemedText";
import { ThemedView } from "@/components/ThemedView";

export default function NotFoundScreen() {
  return (
    <ThemedView style={styles.container}>
      <Stack.Screen options={{ title: "404" }} />
      <ThemedText>页面不存在 / Page not found</ThemedText>
      <Link href="/">
        <ThemedText>返回首页 / Back home</ThemedText>
      </Link>
    </ThemedView>
  );
}
const styles = StyleSheet.create({
  container: { flex: 1, padding: 24, gap: 16, alignItems: "center", justifyContent: "center" },
});
