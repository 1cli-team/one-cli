/**
 * Shared colors for React Native styles and light / dark themes.
 */

const tintColorLight = "#ea580c";
const tintColorDark = "#fb923c";

export const BrandColors = {
  background: "#0a0a0a",
  text: "#fafafa",
  buttonText: "#fff",
};

export const Colors = {
  light: {
    text: "#11181C",
    mutedText: "#687076",
    border: "#e5e7eb",
    background: "#fff",
    tint: tintColorLight,
    icon: "#687076",
    tabIconDefault: "#687076",
    tabIconSelected: tintColorLight,
  },
  dark: {
    text: "#ECEDEE",
    mutedText: "#9BA1A6",
    border: "#374151",
    background: "#151718",
    tint: tintColorDark,
    icon: "#9BA1A6",
    tabIconDefault: "#9BA1A6",
    tabIconSelected: tintColorDark,
  },
};
