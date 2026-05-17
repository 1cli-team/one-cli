# expo-mobile — Agent Guide

Mobile app. Stack: **React Native + Expo + expo-router (file-based routing) + NativeWind (Tailwind for RN) + zustand + SWR + axios + MMKV (encrypted KV)**.

The home screen at `src/app/(tabs)/index.tsx` is intentionally minimal. Don't bring back demo galleries — extend by composing the pre-wired infrastructure below.

## Project layout

```
src/
├── api/                  # Pure-function API + keys (common, auth, token)
├── app/                  # expo-router routes (file-based)
│   ├── _layout.tsx       # Root layout (providers, navigation stack)
│   ├── (tabs)/           # Tab group (index, explore)
│   └── +not-found.tsx    # 404 screen
├── components/
│   ├── ThemedView.tsx    # View wrapper that reads theme bg color
│   ├── ThemedText.tsx    # Text wrapper that reads theme text color (types: title, subtitle, default, defaultSemiBold, link)
│   ├── ExternalLink.tsx  # Wraps expo-web-browser to open URLs in-app
│   ├── HelloWave.tsx, Collapsible.tsx, HapticTab.tsx
│   └── ui/IconSymbol.tsx, ui/TabBarBackground.tsx
├── constants/colors.ts   # Theme color map (light + dark)
├── constants/api.ts      # Base URL, headers
├── hooks/
│   ├── useAuth.ts        # Login / logout flow
│   ├── useColorScheme.ts # Returns "light" | "dark"
│   ├── useThemeColor.ts  # Resolve a token to a hex per-theme
│   └── setup/            # SWR provider, init wiring
├── lib/
│   ├── axios.ts          # HTTP client
│   ├── mmkv.ts           # Synchronous encrypted KV (MMKV)
│   ├── auth/jwt.ts       # JWT helpers
│   ├── error.ts          # Error helpers
│   └── helper.ts
├── store/
│   ├── config.ts         # useConfigStore — theme + locale
│   ├── secure.ts         # MMKV-backed secure store
│   └── session.ts        # Session state
└── types/                # Shared TS types
```

## Pre-wired infrastructure — DO import, DON'T recreate

| Need | Where |
|------|-------|
| Theme | `useConfigStore` from `@/store/config` (toggle `theme` / `locale`) |
| Theme color | `useThemeColor` from `@/hooks/useThemeColor` + tokens in `@/constants/colors` |
| Auth | `useAuth` from `@/hooks/useAuth` — `userLogin(code)`, `userLogout()`, `isLogin` |
| HTTP client | `@/lib/axios` |
| Synchronous storage | `@/lib/mmkv` (encrypted, fast) |
| SWR | `@/hooks/setup/swr` (provider) |
| API calls | `@/api/{common,auth,token,index}.ts` |
| External links | `<ExternalLink href="...">` from `@/components/ExternalLink` |
| Themed view / text | `<ThemedView>` / `<ThemedText type="title|subtitle|default">` |

## Atomic design (advisory — physical folders NOT enforced)

| Layer | Where | Examples |
|-------|-------|----------|
| atoms | `src/components/{ThemedView,ThemedText,ExternalLink}.tsx` + `src/components/ui/` | Themed primitives, icons |
| molecules | `src/components/` | HelloWave, Collapsible, HapticTab |
| organisms | screens themselves (`src/app/(tabs)/*.tsx`) | Composed sections within screens |
| pages | `src/app/` | expo-router routes |

## Design tokens — use NativeWind utilities, never hex/rgb

Tokens are defined in `tailwind.config.js` (NativeWind preset). They map to CSS variables (HSL).

| Palette | Classes |
|---------|---------|
| Brand 1 | `bg-brand1-{1..7}`, `text-brand1-{1..7}`, `border-brand1-{1..7}` |
| Brand 2 | `bg-brand2-{1..7}`, `text-brand2-{1..7}` |
| Neutral | `bg-gray-{1..11}`, `text-gray-{1..11}`, `border-gray-{1..11}` |
| Semantic text | `text-{info,success,warning,error}-{1,2,3,5,6,7}` |

❌ DON'T hardcode hex/rgb in `style={{}}` or `className`. Use NativeWind utilities or `useThemeColor`.

## Common patterns

**Theme toggle**

```tsx
import { useShallow } from "zustand/react/shallow";
import { useConfigStore, configSelector } from "@/store/config";
import { Switch } from "react-native";

const { theme, setTheme } = useConfigStore(useShallow(configSelector));
<Switch value={theme === "dark"} onValueChange={(v) => setTheme(v ? "dark" : "light")} />
```

**Auth flow**

```tsx
import { useAuth } from "@/hooks/useAuth";
const { isLogin, userLogin, userLogout } = useAuth();
userLogin("code-from-server"); // sets session token
```

**Public + private API**

```tsx
import useSWR from "swr";
import { commonPublicApi, commonPublicApiKey, commonAuthApi, commonAuthApiKey } from "@/api/common";

const { data: pub } = useSWR(commonPublicApiKey, commonPublicApi);
const { data: priv } = useSWR(isLogin ? commonAuthApiKey : null, () => commonAuthApi({ user }));
```

**External link**

```tsx
import { ExternalLink } from "@/components/ExternalLink";
<ExternalLink href="https://docs.expo.dev">
  <ThemedText>Docs</ThemedText>
</ExternalLink>
```

## RN-specific gotchas

- No CSS — use `style={{}}` (RN style objects) or NativeWind `className`. Many web Tailwind utilities don't exist (no `gap` on older RN, no `grid`).
- `<View>` ≠ `<div>`. Default to `<ThemedView>` for theme-awareness.
- Always wrap touchable areas with `<Pressable>` or `<TouchableOpacity>` — `onPress` won't fire on plain `<View>`.
- Don't use `localStorage` / `sessionStorage` — use `@/lib/mmkv`.

## Quality gates

```bash
pnpm lint
pnpm test          # jest (snapshot tests for ThemedText etc.)
pnpm typecheck     # tsc --noEmit
```

All must pass before declaring a change complete.
