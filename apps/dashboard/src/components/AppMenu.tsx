import { Boxes, ChevronDown, House, KeyRound, Settings2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
	EnvironmentLink,
	EnvironmentNavLink,
} from "@/features/environment-context/EnvironmentLink";
import { SessionStatus } from "@/features/infisical-session/AccountSettings";
import { WorkspaceMenu } from "@/features/workspace-registry/WorkspaceMenu";
import { useThemeStore } from "@/lib/stores/theme";

export function AppMenu() {
	const { t } = useTranslation();
	const { mode } = useThemeStore();
	return (
		<DropdownMenu>
			<DropdownMenuTrigger asChild>
				<Button variant="ghost" className="group h-10 gap-2 px-2" aria-label={t("appMenu.label")}>
					<img
						src={mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg"}
						alt=""
						className="size-6"
					/>
					<span className="text-base font-semibold tracking-tight">One CLI</span>
					<span className="hidden text-xs font-normal text-muted-foreground ud-sm:inline">
						Dashboard
					</span>
					<ChevronDown
						className="size-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-180"
						aria-hidden="true"
					/>
				</Button>
			</DropdownMenuTrigger>
			<DropdownMenuContent
				align="start"
				side="bottom"
				sideOffset={8}
				collisionPadding={12}
				className="w-80 max-w-[calc(100vw-1.5rem)] p-2"
			>
				<DropdownMenuGroup>
					<DropdownMenuItem asChild>
						<EnvironmentNavLink
							to="/"
							end
							className="min-h-9 aria-[current=page]:bg-accent aria-[current=page]:text-primary-text"
						>
							<House aria-hidden="true" />
							<span>{t("sidebar.home")}</span>
						</EnvironmentNavLink>
					</DropdownMenuItem>
					<DropdownMenuItem asChild>
						<EnvironmentNavLink
							to="/global"
							className="min-h-9 aria-[current=page]:bg-accent aria-[current=page]:text-primary-text"
						>
							<KeyRound aria-hidden="true" />
							<span>{t("global.title")}</span>
						</EnvironmentNavLink>
					</DropdownMenuItem>
					<DropdownMenuItem asChild>
						<EnvironmentNavLink
							to="/templates"
							className="min-h-9 aria-[current=page]:bg-accent aria-[current=page]:text-primary-text"
						>
							<Boxes aria-hidden="true" />
							<span>{t("templateCatalog.title")}</span>
						</EnvironmentNavLink>
					</DropdownMenuItem>
				</DropdownMenuGroup>
				<DropdownMenuSeparator />
				<WorkspaceMenu />
				<DropdownMenuSeparator />
				<DropdownMenuGroup>
					<DropdownMenuItem asChild>
						<EnvironmentLink to="/settings" className="min-h-12">
							<Settings2 aria-hidden="true" />
							<span className="min-w-0 flex-1">
								<span className="block">{t("sidebar.settings")}</span>
								<SessionStatus />
							</span>
						</EnvironmentLink>
					</DropdownMenuItem>
				</DropdownMenuGroup>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
