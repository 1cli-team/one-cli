import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import type { ConnectionState } from "./api";

export function ConnectionBadge({ state }: { state: ConnectionState }) {
	const { t } = useTranslation();
	const variant = state === "connected" ? "success" : state === "unconfigured" ? "muted" : "error";
	return (
		<Badge variant={variant} role="status">
			<span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
			{t(`gateway.connectionState.${state}`)}
		</Badge>
	);
}
