module.exports = {
  preset: "jest-expo",
  transformIgnorePatterns: [
    "node_modules/(?!(?:\\.pnpm/)?((jest-)?react-native|@react-native|@react-navigation|react-navigation|@expo|expo.*|@expo-google-fonts|native-base|react-native-svg))",
  ],
};
