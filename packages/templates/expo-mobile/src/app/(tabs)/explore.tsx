import React from "react";
import { View, Text } from "react-native";

export default function TabTwoScreen() {
  return (
    <View>
      <Text>REM 方案</Text>
      <View className="w-[1rem] bg-red-500 h-[1rem] dark:text-white text-sm p-[0.1rem]">
        <Text className="text-red-800">Hello</Text>
      </View>

      <Text>px 方案</Text>
      <View className="w-[100px] bg-red-500 h-[100px] dark:text-white text-sm p-[10px]">
        <Text className="text-red-800">Hello</Text>
      </View>
    </View>
  );
}
