import React from "react";
import { StyleSheet, View } from "react-native";
import { ThemedText } from "@/components/ThemedText";
import { ThemedView } from "@/components/ThemedView";
import { useThemeColor } from "@/hooks/useThemeColor";

export default function TabTwoScreen() {
  const borderColor = useThemeColor({}, "border");

  return (
    <ThemedView style={styles.container}>
      <ThemedText type="subtitle">原生样式 / Native styles</ThemedText>
      <View style={[styles.example, { borderColor }]}>
        <ThemedText style={styles.exampleText}>StyleSheet</ThemedText>
      </View>

      <ThemedText type="subtitle">内联样式 / Inline styles</ThemedText>
      <View style={{ width: 100, height: 100, padding: 10, borderWidth: 1, borderColor }}>
        <ThemedText style={styles.exampleText}>style</ThemedText>
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: 24,
    gap: 16,
  },
  example: {
    width: 100,
    height: 100,
    padding: 10,
    borderWidth: 1,
  },
  exampleText: {
    fontSize: 14,
  },
});
