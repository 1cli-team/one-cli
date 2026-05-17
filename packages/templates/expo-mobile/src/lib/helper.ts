export function isAppInactive(state: string) {
  return state.match(/inactive|background/);
}

export function safeParse(jsonStr?: string | null) {
  if (!jsonStr) {
    return null;
  }
  try {
    return JSON.parse(jsonStr);
  } catch (e: unknown) {
    console.error(e);
    return null;
  }
}
