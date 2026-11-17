import type {} from "@one-desktop/preload";

async function mount() {
	document.documentElement.toggleAttribute("data-desktop", Boolean(window.desktop));
	if (window.desktop && localStorage.getItem("one_desktop_migration_v1") !== "1") {
		try {
			const preferences = await window.desktop.getLegacyPreferences();
			for (const [key, value] of Object.entries(preferences)) {
				if (localStorage.getItem(key) === null && value !== undefined)
					localStorage.setItem(key, value);
			}
			localStorage.setItem("one_desktop_migration_v1", "1");
		} catch {
			/* Existing preferences and Go configuration remain available. */
		}
	}
	await import("./app-root");
}
void mount();
