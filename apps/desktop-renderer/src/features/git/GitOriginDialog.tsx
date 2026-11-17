import { useState } from "react";
import { useTranslation } from "react-i18next";
import { setGitOrigin, type GitRemote, type GitScope } from "@/api/git";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import type { HttpError } from "@/types/api";

export function GitOriginDialog({
	scope,
	origin,
	onClose,
	onSaved,
}: {
	scope: GitScope;
	origin?: GitRemote;
	onClose(): void;
	onSaved(origin: GitRemote): void;
}) {
	const { t } = useTranslation();
	const [url, setURL] = useState(origin?.fetchUrl ?? "");
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState("");

	async function save() {
		if (saving || !url.trim() || url.trim() === origin?.fetchUrl) return;
		setSaving(true);
		setError("");
		try {
			const result = await setGitOrigin(scope, url.trim(), origin);
			onSaved(result);
			onClose();
		} catch (e) {
			setError((e as HttpError | Error).message || t("git.failed"));
		} finally {
			setSaving(false);
		}
	}

	return (
		<Dialog open onOpenChange={(open) => !open && !saving && onClose()}>
			<DialogContent showCloseButton={!saving}>
				<DialogHeader>
					<DialogTitle>{t(origin ? "git.editOrigin" : "git.bindOrigin")}</DialogTitle>
					<DialogDescription>{t("git.originDescription")}</DialogDescription>
				</DialogHeader>
				<form
					className="grid gap-4"
					onSubmit={(event) => {
						event.preventDefault();
						void save();
					}}
				>
					<div className="space-y-2">
						<Label htmlFor="git-origin-address">{t("git.repositoryURL")}</Label>
						<Input
							id="git-origin-address"
							value={url}
							onChange={(event) => setURL(event.target.value)}
							placeholder="https://github.com/owner/repo.git"
							autoComplete="off"
							spellCheck={false}
							required
							disabled={saving}
						/>
					</div>
					{error && <ErrorNotice>{error}</ErrorNotice>}
					<DialogFooter>
						<Button type="button" variant="outline" onClick={onClose} disabled={saving}>
							{t("git.cancel")}
						</Button>
						<Button
							type="submit"
							disabled={saving || !url.trim() || url.trim() === origin?.fetchUrl}
						>
							{t(saving ? "git.working" : "groups.save")}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
