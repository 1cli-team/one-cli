import * as React from "react";
import renderer from "react-test-renderer";
import { ThemedText } from "../ThemedText";

it(`renders correctly`, async () => {
  let component!: renderer.ReactTestRenderer;
  await renderer.act(async () => {
    component = renderer.create(<ThemedText>Snapshot test!</ThemedText>);
  });

  expect(component.toJSON()).toMatchSnapshot();
});
