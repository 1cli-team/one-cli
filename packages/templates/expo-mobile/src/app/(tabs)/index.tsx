/**
 * Welcome screen — kept intentionally minimal for AI agents to extend.
 *
 * Pre-wired infrastructure (don't recreate; import from these paths):
 *   - Theme:         useConfigStore (src/store/config.ts)  — toggles 'light' / 'dark'
 *   - Theme color:   useThemeColor (src/hooks/useThemeColor.ts) + src/constants/colors.ts
 *   - Auth:          useAuth (src/hooks/useAuth.ts)        — userLogin(code) / userLogout()
 *   - HTTP client:   src/lib/axios.ts
 *   - Storage:       src/lib/mmkv.ts                        (synchronous, encrypted KV)
 *   - SWR setup:     src/hooks/setup/swr.ts
 *   - API modules:   src/api/{common,auth,token,index}.ts  — `key + fetcher` shape
 *   - Routing:       expo-router (src/app/, file-based)
 *   - Themed atoms:  ThemedView / ThemedText (src/components/) — read theme color from tokens
 *   - External link: ExternalLink (src/components/ExternalLink.tsx) — wraps expo-web-browser
 *   - Design tokens: tailwind.config.js (NativeWind) — see palette below
 *
 * Atomic design (advisory — physical folders NOT enforced):
 *   - atoms      → src/components/{ThemedView,ThemedText,ExternalLink}.tsx + ui/
 *   - molecules  → src/components/                (compose atoms; e.g. ResourceLink below)
 *   - organisms  → src/app/(tabs)/<screen>.tsx    (screens are organisms here)
 *   - pages      → src/app/                       (expo-router routes)
 *
 * Design tokens — DO use NativeWind utilities (they map to CSS vars in tailwind.config.js):
 *   - Brand:      bg-brand1-{1..7} / text-brand1-{...} / border-brand1-{...}
 *                 bg-brand2-{1..7} / text-brand2-{...}
 *   - Neutral:    bg-gray-{1..11} / text-gray-{...} / border-gray-{...}
 *   - Semantic:   text-{info,success,warning,error}-{1,2,3,5,6,7}  (text only for most)
 *   DON'T hardcode hex / rgb in styles.
 */

import React from "react";
import { Pressable, View } from "react-native";
import { ThemedText } from "@/components/ThemedText";
import { ThemedView } from "@/components/ThemedView";
import { ExternalLink } from "@/components/ExternalLink";

export default function HomeScreen() {
  return (
    <ThemedView className="flex-1 items-center justify-center px-6 py-12">
      <View className="rounded-2xl bg-[#0a0a0a] px-6 py-4 mb-8">
        <ThemedText
          style={{
            color: "#fafafa",
            fontSize: 24,
            fontWeight: "500",
            letterSpacing: 4,
          }}
        >
          ONE CLI
        </ThemedText>
      </View>

      <ThemedText type="title" className="mb-2 text-center">
        Welcome to One CLI
      </ThemedText>
      <ThemedText className="mb-2 text-center text-gray-7" style={{ fontSize: 12, letterSpacing: 1.5 }}>
        EXPO · REACT NATIVE
      </ThemedText>
      <ThemedText className="mb-6 max-w-xs text-center text-gray-7">
        一个由 One CLI 生成的 Expo 移动端脚手架，已预置 NativeWind、SWR、MMKV 与设计令牌。
      </ThemedText>

      <ThemedText className="mb-8 text-center text-gray-7" style={{ fontSize: 13 }}>
        Edit src/app/(tabs)/index.tsx and save to start.
      </ThemedText>

      <View className="w-full max-w-xs gap-3">
        <ExternalLink href="https://one.torchstellar.com/zh/docs/quick-start/" asChild>
          <Pressable
            style={{ backgroundColor: "#ea580c" }}
            className="rounded-full px-5 py-3"
          >
            <ThemedText style={{ color: "#ffffff", textAlign: "center", fontWeight: "500" }}>
              开始构建
            </ThemedText>
          </Pressable>
        </ExternalLink>
        <ExternalLink href="https://github.com/torchstellar-team/one-cli" asChild>
          <Pressable className="rounded-full border border-gray-3 px-5 py-3">
            <ThemedText style={{ textAlign: "center", fontWeight: "500" }}>GitHub</ThemedText>
          </Pressable>
        </ExternalLink>
      </View>
    </ThemedView>
  );
}
