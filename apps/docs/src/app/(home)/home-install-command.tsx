"use client";

import { useState } from "react";
import { HomeCopyButton } from "./home-template-preview";

const installCommands = {
  unix: {
    prompt: "$",
    value: "curl -fsSL https://1cli.dev/install.sh | bash",
  },
  windows: {
    prompt: "PS>",
    value: "irm https://1cli.dev/install.ps1 | iex",
  },
} as const;

type InstallPlatform = keyof typeof installCommands;

export function HomeInstallCommand({
  platformLabel,
  unixLabel,
  windowsLabel,
  copyLabel,
  copiedLabel,
}: {
  platformLabel: string;
  unixLabel: string;
  windowsLabel: string;
  copyLabel: string;
  copiedLabel: string;
}) {
  const [platform, setPlatform] = useState<InstallPlatform>("unix");
  const command = installCommands[platform];

  return (
    <div className="w-full max-w-[580px] overflow-hidden rounded-lg border border-[#292524] bg-[#1c1917]">
      <div
        aria-label={platformLabel}
        className="grid grid-cols-2 gap-1 border-b border-[#292524] bg-black/15 p-1"
        role="group"
      >
        <PlatformButton
          active={platform === "unix"}
          label={unixLabel}
          onClick={() => setPlatform("unix")}
        />
        <PlatformButton
          active={platform === "windows"}
          label={windowsLabel}
          onClick={() => setPlatform("windows")}
        />
      </div>
      <div className="flex min-w-0 items-center gap-2 px-3 py-2.5">
        <span className="shrink-0 font-mono text-xs text-[#ea580c]">
          {command.prompt}
        </span>
        <code
          aria-live="polite"
          className="min-w-0 flex-1 truncate font-mono text-xs text-stone-100"
        >
          {command.value}
        </code>
        <HomeCopyButton
          key={platform}
          value={command.value}
          label={copyLabel}
          copiedLabel={copiedLabel}
          className="shrink-0 border-transparent px-1.5 py-1 font-mono text-[10px] lowercase text-stone-500 hover:text-white"
        />
      </div>
    </div>
  );
}

function PlatformButton({
  active,
  label,
  onClick,
}: {
  active: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      aria-pressed={active}
      className={[
        "inline-flex min-w-0 items-center justify-center rounded-md px-3 py-1.5 font-mono text-[11px] font-medium transition-colors",
        "focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-orange-500",
        active
          ? "bg-[#292524] text-white shadow-sm"
          : "text-stone-500 hover:bg-white/[0.03] hover:text-stone-200",
      ].join(" ")}
      onClick={onClick}
      type="button"
    >
      <span className="truncate">{label}</span>
    </button>
  );
}
