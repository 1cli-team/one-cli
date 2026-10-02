import * as React from "react";
import { StyleSheet, Text, View } from "react-native";
import renderer from "react-test-renderer";
import { Colors } from "@/constants/colors";
import { useConfigStore } from "@/store/config";
import { ThemedText } from "../ThemedText";
import { ThemedView } from "../ThemedView";

jest.mock("@react-native-async-storage/async-storage", () =>
  require("@react-native-async-storage/async-storage/jest/async-storage-mock"),
);

let component: renderer.ReactTestRenderer;

beforeEach(() => {
  useConfigStore.setState({ theme: "light" });
});

afterEach(async () => {
  await renderer.act(async () => {
    component.unmount();
  });
});

it(`renders correctly`, async () => {
  await renderer.act(async () => {
    component = renderer.create(<ThemedText>Snapshot test!</ThemedText>);
  });

  expect(component.toJSON()).toMatchSnapshot();
});

it("updates native text and background colors when the persisted theme changes", async () => {
  await renderer.act(async () => {
    component = renderer.create(
      <ThemedView>
        <ThemedText>Theme test</ThemedText>
      </ThemedView>,
    );
  });

  for (const theme of ["light", "dark", "light"] as const) {
    await renderer.act(async () => {
      useConfigStore.getState().setTheme(theme);
    });

    expect(StyleSheet.flatten(component.root.findByType(Text).props.style).color).toBe(
      Colors[theme].text,
    );
    expect(StyleSheet.flatten(component.root.findByType(View).props.style).backgroundColor).toBe(
      Colors[theme].background,
    );
  }
});

it("keeps native style and light / dark color overrides", async () => {
  await renderer.act(async () => {
    component = renderer.create(
      <ThemedText lightColor="#123456" darkColor="#abcdef" style={{ fontSize: 18 }}>
        Custom styles
      </ThemedText>,
    );
  });

  expect(StyleSheet.flatten(component.root.findByType(Text).props.style)).toMatchObject({
    color: "#123456",
    fontSize: 18,
  });

  await renderer.act(async () => {
    useConfigStore.getState().setTheme("dark");
  });

  expect(StyleSheet.flatten(component.root.findByType(Text).props.style)).toMatchObject({
    color: "#abcdef",
    fontSize: 18,
  });
});
