import { useState } from "react";
import {
	AlertDialog,
	AlertDialogContent,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogCancel,
	AlertDialogAction,
} from "@/components/ui/alert-dialog";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import { useManifestDraftStore } from "@/features/manifest-draft/manifest-draft-store";
import { http } from "@/lib/http";
import { ChevronDown, House, KeyRound, Settings2 } from "lucide-react";
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
import { useDesktop, native } from "@/lib/desktop/runtime";
import { useThemeStore } from "@/lib/stores/theme";

export function AppMenu() {
	const { t } = useTranslation();
	const { mode } = useThemeStore();
	const [confirmSwitch, setConfirmSwitch] = useState(false);
	function disconnect() {
		useEnvironmentDirtyStore.getState().discardAll();
		useManifestDraftStore.setState({ drafts: {} });
		http.disconnect();
		useDesktop.setState({ connected: false, status: "chooseServer" });
	}
	function switchServer() {
		if (
			useEnvironmentDirtyStore.getState().dirty ||
			Object.keys(useManifestDraftStore.getState().drafts).length
		)
			setConfirmSwitch(true);
		else disconnect();
	}
	return (
		<>
			<DropdownMenu>
				<DropdownMenuTrigger asChild>
					<Button variant="ghost" className="group h-10 gap-2 px-2" aria-label={t("appMenu.label")}>
						<img
							src={mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg"}
							alt=""
							className="size-6"
						/>
						<span className="text-base font-semibold tracking-tight">
							{native ? "One Desktop" : "One Server"}
						</span>
						<span className="hidden text-xs font-normal text-muted-foreground ud-sm:inline"></span>
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
						<DropdownMenuItem onSelect={switchServer}>{t("server.switch")}</DropdownMenuItem>
						<DropdownMenuItem asChild>
							<EnvironmentNavLink to="/desktop" className="min-h-9">
								<Settings2 />
								<span>{t("desktop.settings")}</span>
							</EnvironmentNavLink>
						</DropdownMenuItem>
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
			<AlertDialog open={confirmSwitch} onOpenChange={setConfirmSwitch}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t("server.discardTitle")}</AlertDialogTitle>
						<AlertDialogDescription>{t("server.discardHint")}</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
						<AlertDialogAction onClick={disconnect}>{t("server.switch")}</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
}
