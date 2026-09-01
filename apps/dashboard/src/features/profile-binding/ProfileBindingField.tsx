import { AlertCircle, ExternalLink, Info } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { useBackendCatalog } from "@/api/catalog";
import { getSection, sectionKey } from "@/api/configure";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Field, FieldLabel } from "@/components/ui/field";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import type { BackendDomain, ProfileBinding } from "@/types/api";

type ProfileBindingScope = "project" | "workspace";
const AUTOMATIC_PROFILE_VALUE = "__automatic_profile__";

function errorMessage(error: unknown): string {
	if (error && typeof error === "object" && "message" in error) {
		return String(error.message);
	}
	return String(error);
}

export const ProfileBindingField: React.FC<{
	id: string;
	scope: ProfileBindingScope;
	directSource: string;
	domain: BackendDomain;
	backend?: string;
	configurable?: boolean;
	binding?: ProfileBinding;
	value: string;
	onChange(value: string): void;
	disabled?: boolean;
}> = ({
	id,
	scope,
	directSource,
	domain,
	backend,
	configurable,
	binding,
	value,
	onChange,
	disabled,
}) => {
	const { t } = useTranslation();
	const catalog = useBackendCatalog();
	const spec = backend ? catalog.byID.get(`${domain}/${backend}`) : undefined;
	const profileConfigurable = configurable ?? spec?.profile.configurable;
	const key = backend && profileConfigurable ? sectionKey(domain, backend) : null;
	const section = useSWR(key, () => getSection(domain, backend ?? ""));
	const copyRoot =
		scope === "workspace" ? "overview.workspaceEnv.profile" : "projectInspector.profile";
	const names = Array.from(
		new Set([...Object.keys(section.data?.section.profiles ?? {}), ...(value ? [value] : [])]),
	).sort();
	const automaticLabel =
		binding && binding.source !== directSource
			? t(`${copyRoot}.inherited`, {
					name: binding.name,
					source: binding.source,
				})
			: t(`${copyRoot}.automatic`);

	if (!backend || profileConfigurable === false) {
		return (
			<Alert className="rounded-lg bg-muted/25 py-3 text-xs text-muted-foreground">
				<Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
				<AlertDescription className="text-xs leading-relaxed">
					{t(`${copyRoot}.notRequired`)}
				</AlertDescription>
			</Alert>
		);
	}

	if (profileConfigurable === undefined && catalog.isLoading) {
		return (
			<div
				className="space-y-2 rounded-lg border border-border p-4"
				aria-label={t(`${copyRoot}.loading`)}
			>
				<Skeleton className="h-3 w-24" />
				<Skeleton className="h-9 w-full" />
			</div>
		);
	}

	if (profileConfigurable === undefined && catalog.error) {
		return (
			<Alert variant="destructive" className="rounded-lg py-3 text-xs">
				<AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
				<AlertDescription className="text-xs leading-relaxed">
					{t(`${copyRoot}.loadFailed`)} {errorMessage(catalog.error)}
				</AlertDescription>
			</Alert>
		);
	}

	return (
		<Field className="gap-3 rounded-lg border border-border p-4">
			<div>
				<FieldLabel htmlFor={id}>{t(`${copyRoot}.label`)}</FieldLabel>
				<p className="mt-1 text-xs leading-relaxed text-muted-foreground">
					{t(`${copyRoot}.description`)}
				</p>
			</div>
			<Select
				value={value || AUTOMATIC_PROFILE_VALUE}
				onValueChange={(next) => onChange(next === AUTOMATIC_PROFILE_VALUE ? "" : next)}
				disabled={disabled || section.isLoading || Boolean(section.error)}
			>
				<SelectTrigger id={id}>
					<SelectValue>{value || automaticLabel}</SelectValue>
				</SelectTrigger>
				<SelectContent>
					<SelectItem value={AUTOMATIC_PROFILE_VALUE}>{automaticLabel}</SelectItem>
					{names.map((name) => (
						<SelectItem key={name} value={name}>
							{name}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
			{section.error ? (
				<Alert variant="destructive" className="mt-2 rounded-lg py-2.5 text-[11px]">
					<AlertCircle className="mt-0.5 h-3 w-3 shrink-0" />
					<AlertDescription className="text-[11px] leading-relaxed">
						{t(`${copyRoot}.loadFailed`)} {errorMessage(section.error)}
					</AlertDescription>
				</Alert>
			) : null}
			<div className="mt-2 flex items-center justify-between text-[11px] text-muted-foreground">
				<span>{binding ? `${binding.name} · ${binding.source}` : t(`${copyRoot}.none`)}</span>
				<EnvironmentLink
					to={`/settings/${domain}/${backend}`}
					className="inline-flex items-center gap-1 hover:text-primary"
				>
					{t(`${copyRoot}.manage`)}
					<ExternalLink className="h-3 w-3" />
				</EnvironmentLink>
			</div>
		</Field>
	);
};
