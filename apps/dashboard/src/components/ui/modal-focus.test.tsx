import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import {
	AlertDialog,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Sheet, SheetContent, SheetDescription, SheetTitle } from "@/components/ui/sheet";

describe("controlled modal focus", () => {
	it("returns focus after a controlled Sheet without a rendered trigger closes", async () => {
		const user = userEvent.setup();
		render(<ControlledSheet />);
		const opener = screen.getByRole("button", { name: "Open inspector" });

		await user.click(opener);
		const dialog = await screen.findByRole("dialog");
		await user.click(screen.getByRole("button", { name: "Close" }));
		await waitFor(() => expect(dialog.isConnected).toBe(false));

		expect(document.activeElement).toBe(opener);
	});

	it("returns focus after a controlled AlertDialog without a rendered trigger closes", async () => {
		const user = userEvent.setup();
		render(<ControlledAlertDialog />);
		const opener = screen.getByRole("button", { name: "Forget workspace" });

		await user.click(opener);
		const dialog = await screen.findByRole("alertdialog");
		await user.click(screen.getByRole("button", { name: "Cancel" }));
		await waitFor(() => expect(dialog.isConnected).toBe(false));

		expect(document.activeElement).toBe(opener);
	});

	it("returns focus after a controlled Dialog without a rendered trigger closes", async () => {
		const user = userEvent.setup();
		render(<ControlledDialog />);
		const opener = screen.getByRole("button", { name: "Add profile" });

		await user.click(opener);
		const dialog = await screen.findByRole("dialog");
		await user.click(screen.getByRole("button", { name: "Close" }));
		await waitFor(() => expect(dialog.isConnected).toBe(false));

		expect(document.activeElement).toBe(opener);
	});
});

function ControlledSheet() {
	const [open, setOpen] = useState(false);
	return (
		<>
			<Button onClick={() => setOpen(true)}>Open inspector</Button>
			<Sheet open={open} onOpenChange={setOpen}>
				<SheetContent>
					<SheetTitle>Project inspector</SheetTitle>
					<SheetDescription>Configure this project.</SheetDescription>
				</SheetContent>
			</Sheet>
		</>
	);
}

function ControlledAlertDialog() {
	const [open, setOpen] = useState(false);
	return (
		<>
			<Button onClick={() => setOpen(true)}>Forget workspace</Button>
			<AlertDialog open={open} onOpenChange={setOpen}>
				<AlertDialogContent>
					<AlertDialogTitle>Forget this workspace?</AlertDialogTitle>
					<AlertDialogDescription>This only removes the registry entry.</AlertDialogDescription>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
}

function ControlledDialog() {
	const [open, setOpen] = useState(false);
	return (
		<>
			<Button onClick={() => setOpen(true)}>Add profile</Button>
			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent>
					<DialogTitle>Add profile</DialogTitle>
					<DialogDescription>Add machine-local credentials.</DialogDescription>
				</DialogContent>
			</Dialog>
		</>
	);
}
