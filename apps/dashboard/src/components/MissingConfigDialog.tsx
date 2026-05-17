// MissingConfigDialog turns an Overview missing-config badge into an
// in-place fix: pick a backend kind (and, for container, an optional
// image), PUT it to the manifest-mutate endpoint, and feed the returned
// fresh Overview back to the caller's SWR cache. It deliberately only
// writes the *kind* — credentials still live in the profile editor, which
// is the whole point (keep secrets in the UI, never in agent context).

import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { setProjectContainer, setProjectDeploy, setWorkspaceEnv } from "@/api/workspace";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useToast } from "@/hooks/useToast";
import {
	CONTAINER_KINDS,
	DEPLOY_KINDS,
	ENV_KINDS,
	type Overview,
	type OverviewIssueDomain,
} from "@/types/api";

const KIND_OPTIONS: Record<OverviewIssueDomain, readonly string[]> = {
	env: ENV_KINDS,
	deploy: DEPLOY_KINDS,
	container: CONTAINER_KINDS,
};

interface MissingConfigDialogProps {
	domain: OverviewIssueDomain | null;
	// projectName is required for deploy/container; ignored for env (which
	// is a workspace-level setting). null closes the dialog.
	projectName?: string;
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onUpdated: (overview: Overview) => void;
}

export const MissingConfigDialog: React.FC<MissingConfigDialogProps> = ({
	domain,
	projectName,
	open,
	onOpenChange,
	onUpdated,
}) => {
	const { t } = useTranslation();
	const toast = useToast();
	const [kind, setKind] = useState("");
	const [image, setImage] = useState("");
	const [saving, setSaving] = useState(false);

	// Reset transient form state whenever the dialog (re)opens for a
	// different issue.
	const reset = () => {
		setKind("");
		setImage("");
		setSaving(false);
	};

	const handleOpenChange = (next: boolean) => {
		if (!next) reset();
		onOpenChange(next);
	};

	if (!domain) return null;

	const options = KIND_OPTIONS[domain];

	async function handleSave() {
		if (!domain || !kind) return;
		setSaving(true);
		try {
			let overview: Overview;
			if (domain === "env") {
				overview = await setWorkspaceEnv(kind);
			} else if (domain === "deploy") {
				if (!projectName) throw new Error("missing project name");
				overview = await setProjectDeploy(projectName, kind);
			} else {
				if (!projectName) throw new Error("missing project name");
				overview = await setProjectContainer(projectName, {
					kind,
					image: image.trim() || undefined,
				});
			}
			onUpdated(overview);
			toast.success(t("overview.fix.saved", { domain }));
			handleOpenChange(false);
		} catch (err) {
			const message = err instanceof Error ? err.message : String(err);
			toast.error(t("overview.fix.failed"), { description: message });
			setSaving(false);
		}
	}

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>{t(`overview.fix.title.${domain}`)}</DialogTitle>
					<DialogDescription>
						{t(`overview.fix.description.${domain}`, { project: projectName ?? "" })}
					</DialogDescription>
				</DialogHeader>

				<div className="grid gap-3">
					<div className="grid gap-1.5">
						<Label htmlFor="missing-config-kind">{t("overview.fix.kindLabel")}</Label>
						<select
							id="missing-config-kind"
							value={kind}
							onChange={(e) => setKind(e.target.value)}
							className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm shadow-sm transition-[border-color,box-shadow] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
						>
							<option value="" disabled>
								{t("overview.fix.kindPlaceholder")}
							</option>
							{options.map((opt) => (
								<option key={opt} value={opt}>
									{opt}
								</option>
							))}
						</select>
					</div>

					{domain === "container" ? (
						<div className="grid gap-1.5">
							<Label htmlFor="missing-config-image">{t("overview.fix.imageLabel")}</Label>
							<Input
								id="missing-config-image"
								value={image}
								onChange={(e) => setImage(e.target.value)}
								placeholder={t("overview.fix.imagePlaceholder")}
							/>
						</div>
					) : null}

					<p className="text-xs text-muted-foreground">
						{t("overview.fix.note", { section: domain })}{" "}
						<Link to="/profile" className="text-primary underline-offset-4 hover:underline">
							{t("overview.fix.sectionLink")}
						</Link>
					</p>
				</div>

				<div className="flex items-center justify-end gap-2">
					<Button variant="ghost" onClick={() => handleOpenChange(false)} disabled={saving}>
						{t("overview.fix.cancel")}
					</Button>
					<Button onClick={handleSave} disabled={!kind || saving}>
						{saving ? t("overview.fix.saving") : t("overview.fix.save")}
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
};
