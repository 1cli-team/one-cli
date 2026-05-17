"use client";

import { useTransitionRouter } from "next-view-transitions";
import { useCallback } from "react";

/**
 * Returns a navigate function that pushes through next-view-transitions,
 * which internally wraps router.push in `document.startViewTransition`
 * AND waits for the new RSC payload + React render to commit before
 * the transition's "new snapshot" is taken. This is the part the
 * plain `document.startViewTransition(() => router.push())` pattern
 * gets wrong under Next.js App Router.
 *
 * Falls back to a normal push on browsers without View Transition API
 * (Firefox / older Safari) — handled inside `useTransitionRouter`.
 */
export function useViewTransitionNavigate() {
  const router = useTransitionRouter();
  return useCallback(
    (href: string) => {
      router.push(href);
    },
    [router],
  );
}

/**
 * Intercept anchor clicks (ignoring modifier-keyed / non-primary clicks
 * so cmd-click still opens in a new tab) and route through the
 * view-transition navigate function.
 */
export function shouldInterceptClick(
  e: React.MouseEvent<HTMLAnchorElement>,
): boolean {
  if (e.defaultPrevented) return false;
  if (e.button !== 0) return false;
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return false;
  return true;
}
