"use client";

import { useEffect } from "react";

export function CodeCopyEnhancer() {
  useEffect(() => {
    const managedButtons = new Set<HTMLButtonElement>();

    function enhance() {
      document
        .querySelectorAll<HTMLElement>(".one-docs-body pre")
        .forEach((pre) => {
          if (pre.dataset.copyEnhanced === "true") return;
          pre.dataset.copyEnhanced = "true";

          const button = document.createElement("button");
          button.type = "button";
          button.className = "one-docs-copy-button";
          button.textContent = "Copy";
          button.setAttribute("aria-label", "Copy code block");

          button.addEventListener("click", async (event) => {
            event.preventDefault();
            event.stopPropagation();

            const text =
              pre.querySelector("code")?.textContent?.trimEnd() ?? "";
            if (!text) return;

            await writeClipboard(text);
            button.textContent = "Copied";
            button.dataset.copied = "true";

            window.setTimeout(() => {
              button.textContent = "Copy";
              delete button.dataset.copied;
            }, 1400);
          });

          pre.appendChild(button);
          managedButtons.add(button);
        });
    }

    enhance();

    const observer = new MutationObserver(enhance);
    observer.observe(document.body, { childList: true, subtree: true });

    return () => {
      observer.disconnect();
      for (const button of managedButtons) {
        delete button.parentElement?.dataset.copyEnhanced;
        button.remove();
      }
      managedButtons.clear();
    };
  }, []);

  return null;
}

async function writeClipboard(text: string) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall back to the textarea path below.
    }
  }

  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.top = "-1000px";
  textarea.style.left = "-1000px";
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand("copy");
  textarea.remove();
}
