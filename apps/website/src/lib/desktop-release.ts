export const desktopSystems = ["macos", "windows", "linux"] as const;
export type DesktopSystem = (typeof desktopSystems)[number];

const installers = [
	{ system: "macos", architecture: "Apple Silicon", suffix: "darwin_arm64.dmg", format: "DMG" },
	{ system: "macos", architecture: "Intel", suffix: "darwin_amd64.dmg", format: "DMG" },
	{ system: "windows", architecture: "x64", suffix: "windows_amd64.exe", format: "EXE" },
	{ system: "linux", architecture: "x64", suffix: "linux_amd64.AppImage", format: "AppImage" },
	{ system: "linux", architecture: "ARM64", suffix: "linux_arm64.AppImage", format: "AppImage" },
] as const;

export type DesktopInstaller = (typeof installers)[number] & {
	name: string;
	url: string;
	size: number;
};

export type DesktopRelease = {
	version: string;
	installers: DesktopInstaller[];
};

export class DesktopReleaseError extends Error {
	constructor(public readonly reason: "unavailable" | "rateLimited") {
		super(reason);
	}
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}

export async function fetchDesktopRelease(signal: AbortSignal): Promise<DesktopRelease> {
	const response = await fetch("https://api.github.com/repos/1cli-team/one-cli/releases/latest", {
		headers: { Accept: "application/vnd.github+json" },
		credentials: "omit",
		signal,
	});
	if (!response.ok) {
		throw new DesktopReleaseError(
			response.status === 403 || response.status === 429 ? "rateLimited" : "unavailable",
		);
	}

	const release: unknown = await response.json();
	if (
		!isRecord(release) ||
		release.draft !== false ||
		release.prerelease !== false ||
		typeof release.tag_name !== "string" ||
		!/^v\d+\.\d+\.\d+$/.test(release.tag_name) ||
		!Array.isArray(release.assets)
	) {
		throw new DesktopReleaseError("unavailable");
	}

	const version = release.tag_name.slice(1);
	const assets: unknown[] = release.assets;
	const downloads: DesktopInstaller[] = [];
	for (const installer of installers) {
		const name = `one-desktop_${version}_${installer.suffix}`;
		const url = `https://github.com/1cli-team/one-cli/releases/download/${release.tag_name}/${name}`;
		const asset = assets.find((item) => isRecord(item) && item.name === name);
		if (
			isRecord(asset) &&
			asset.state === "uploaded" &&
			asset.browser_download_url === url &&
			typeof asset.size === "number" &&
			Number.isSafeInteger(asset.size) &&
			asset.size > 0
		) {
			downloads.push({ ...installer, name, url, size: asset.size });
		}
	}
	if (downloads.length === 0) throw new DesktopReleaseError("unavailable");
	return { version, installers: downloads };
}

export function preferredDesktopSystem(): DesktopSystem {
	const userAgent = navigator.userAgent;
	if (/Android|iPhone|iPad|iPod/i.test(userAgent)) return "macos";
	if (/Windows/i.test(userAgent)) return "windows";
	if (/Linux/i.test(userAgent)) return "linux";
	return "macos";
}
