"use client";

import { Dialog } from "@base-ui/react/dialog";
import { Tabs } from "@base-ui/react/tabs";
import { Download, ExternalLink, LoaderCircle, Monitor, RefreshCw, X } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import type { Locale } from "@/i18n";
import {
	DesktopReleaseError,
	desktopSystems,
	fetchDesktopRelease,
	preferredDesktopSystem,
	type DesktopRelease,
	type DesktopSystem,
} from "@/lib/desktop-release";
import { releasesUrl } from "@/lib/links";
import { desktopDownloadCopy } from "./desktop-download-copy";

type DownloadState =
	| { status: "idle" }
	| { status: "loading" }
	| { status: "ready"; release: DesktopRelease }
	| { status: "error"; reason: "network" | "rateLimited" | "unavailable" };

const focusStyle =
	"focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-orange-500";

export function DesktopDownloadTrigger({
	children,
	className,
}: {
	children: ReactNode;
	className?: string;
}) {
	return (
		<Dialog.Trigger className={`${focusStyle} cursor-pointer ${className ?? ""}`}>
			{children}
		</Dialog.Trigger>
	);
}

export function DesktopDownloadDialog({ children, lang }: { children: ReactNode; lang: Locale }) {
	const text = desktopDownloadCopy[lang];
	const [state, setState] = useState<DownloadState>({ status: "idle" });
	const [system, setSystem] = useState<DesktopSystem>("macos");
	const initialized = useRef(false);
	const request = useRef<AbortController | null>(null);

	useEffect(
		() => () => {
			request.current?.abort();
			request.current = null;
		},
		[],
	);

	async function loadRelease() {
		if (request.current) return;
		const controller = new AbortController();
		request.current = controller;
		setState({ status: "loading" });
		const timeout = window.setTimeout(() => controller.abort(), 10_000);
		try {
			const release = await fetchDesktopRelease(controller.signal);
			if (request.current === controller) setState({ status: "ready", release });
		} catch (error) {
			if (request.current === controller) {
				setState({
					status: "error",
					reason: error instanceof DesktopReleaseError ? error.reason : "network",
				});
			}
		} finally {
			window.clearTimeout(timeout);
			if (request.current === controller) request.current = null;
		}
	}

	return (
		<Dialog.Root
			onOpenChange={(open) => {
				if (!open) return;
				if (!initialized.current) {
					setSystem(preferredDesktopSystem());
					initialized.current = true;
				}
				if (state.status !== "ready") void loadRelease();
			}}
		>
			{children}
			<Dialog.Portal>
				<Dialog.Backdrop className="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm transition-opacity duration-150 data-ending-style:opacity-0 data-starting-style:opacity-0 motion-reduce:transition-none" />
				<Dialog.Popup className="fixed left-1/2 top-1/2 z-50 max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-xl -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border border-[#292524] bg-[#11100f] p-5 text-stone-100 shadow-2xl outline-none transition-[opacity,scale] duration-150 data-ending-style:scale-95 data-ending-style:opacity-0 data-starting-style:scale-95 data-starting-style:opacity-0 motion-reduce:transition-none sm:p-7">
					<div className="mb-5 flex size-11 items-center justify-center rounded-lg border border-orange-500/20 bg-orange-500/10 text-orange-500">
						<Monitor aria-hidden="true" className="size-5" />
					</div>
					<Dialog.Title className="pr-8 text-2xl font-semibold tracking-tight">
						{text.title}
					</Dialog.Title>
					<Dialog.Description className="mt-2 text-sm leading-6 text-stone-400">
						{text.description}
					</Dialog.Description>
					<Dialog.Close
						aria-label={text.close}
						className={`absolute right-4 top-4 grid size-9 cursor-pointer place-items-center rounded-md text-stone-400 transition hover:bg-white/5 hover:text-white ${focusStyle}`}
					>
						<X aria-hidden="true" className="size-5" />
					</Dialog.Close>
					<Tabs.Root
						value={system}
						onValueChange={(value) => {
							if (desktopSystems.includes(value)) setSystem(value);
						}}
						className="mt-6"
					>
						<Tabs.List
							aria-label={text.system}
							className="grid grid-cols-3 gap-1 rounded-lg border border-[#292524] bg-[#0a0a0a] p-1"
						>
							{desktopSystems.map((os) => (
								<Tabs.Tab
									key={os}
									value={os}
									className={`cursor-pointer rounded-md px-2 py-2.5 text-sm font-medium text-stone-400 transition hover:text-white data-active:bg-[#292524] data-active:text-white ${focusStyle}`}
								>
									{text[os]}
								</Tabs.Tab>
							))}
						</Tabs.List>
						{desktopSystems.map((os) => (
							<Tabs.Panel key={os} value={os} className="mt-5 outline-none">
								<p className="min-h-10 text-xs leading-5 text-stone-400">{text.hints[os]}</p>
								{state.status === "idle" || state.status === "loading" ? (
									<div
										role="status"
										className="flex min-h-36 items-center justify-center gap-2 text-sm text-stone-400"
									>
										<LoaderCircle
											aria-hidden="true"
											className="size-4 animate-spin motion-reduce:animate-none"
										/>
										{text.loading}
									</div>
								) : state.status === "error" ? (
									<div className="mt-3 rounded-lg border border-orange-500/20 bg-orange-500/5 p-4">
										<p role="alert" className="text-sm leading-6 text-stone-300">
											{text.errors[state.reason]}
										</p>
										<button
											type="button"
											onClick={() => void loadRelease()}
											className={`mt-3 inline-flex cursor-pointer items-center gap-2 rounded-md border border-[#44403c] px-3 py-2 text-sm transition hover:bg-white/5 ${focusStyle}`}
										>
											<RefreshCw aria-hidden="true" className="size-3.5" />
											{text.retry}
										</button>
									</div>
								) : (
									<div>
										<p role="status" className="mb-3 mt-2 font-mono text-xs text-stone-400">
											{text.stable}{" "}
											<span className="text-orange-400">v{state.release.version}</span>
										</p>
										<div className="grid gap-2">
											{state.release.installers
												.filter((installer) => installer.system === os)
												.map((installer) => (
													<div
														key={installer.name}
														className="flex items-center justify-between gap-3 rounded-lg border border-[#292524] bg-[#1c1917] p-4"
													>
														<div className="min-w-0">
															<p className="text-sm font-semibold">{installer.architecture}</p>
															<p className="mt-1 font-mono text-xs text-stone-400">
																{installer.format} ·{" "}
																{new Intl.NumberFormat(lang, { maximumFractionDigits: 1 }).format(
																	installer.size / 1_000_000,
																)}{" "}
																MB
															</p>
														</div>
														<a
															href={installer.url}
															aria-label={text.downloadLabel
																.replace("{system}", text[os])
																.replace("{architecture}", installer.architecture)}
															className={`inline-flex shrink-0 items-center gap-2 rounded-md bg-[#ea580c] px-3 py-2 text-sm font-semibold text-white transition hover:bg-[#c2410c] ${focusStyle}`}
														>
															<Download aria-hidden="true" className="size-4" />
															{text.download}
														</a>
													</div>
												))}
											{!state.release.installers.some((installer) => installer.system === os) && (
												<p className="rounded-lg border border-[#292524] p-4 text-sm leading-6 text-stone-400">
													{text.empty}
												</p>
											)}
										</div>
									</div>
								)}
							</Tabs.Panel>
						))}
					</Tabs.Root>
					<div className="mt-6 border-t border-[#292524] pt-4">
						<a
							href={releasesUrl}
							target="_blank"
							rel="noreferrer"
							className={`inline-flex items-center gap-2 rounded-sm text-xs text-stone-400 transition hover:text-white ${focusStyle}`}
						>
							{text.github}
							<ExternalLink aria-hidden="true" className="size-3.5" />
						</a>
					</div>
				</Dialog.Popup>
			</Dialog.Portal>
		</Dialog.Root>
	);
}
