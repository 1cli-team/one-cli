import { Text, type TextProps, StyleSheet } from "react-native";
import { useThemeColor } from "@/hooks/useThemeColor";

export function ThemedText({
  style,
  type = "default",
  ...props
}: TextProps & { type?: "default" | "title" }) {
  const color = useThemeColor({}, "text");
  return <Text style={[styles[type], { color }, style]} {...props} />;
}
const styles = StyleSheet.create({
  default: { fontSize: 16, lineHeight: 24 },
  title: { fontSize: 32, fontWeight: "bold", lineHeight: 40 },
});
