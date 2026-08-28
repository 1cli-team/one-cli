import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import i18n from "@/lib/i18n";
import { ProfileForm } from "@/pages/SectionDetail";

describe("local connection form", () => {
	beforeAll(async () => {
		await i18n.changeLanguage("en-US");
	});

	it("keeps a masked API token hidden and accessible while editing", async () => {
		const user = userEvent.setup();
		const onSubmit = vi.fn(async () => {});
		render(
			<MemoryRouter>
				<Dialog open>
					<DialogContent>
						<ProfileForm
							sectionKey="deploy/vercel"
							initialName="production"
							initialProfile={{ team: "one-team", credentials: { apiToken: "********" } }}
							mode="edit"
							hasDefault
							onCancel={() => {}}
							onSubmit={onSubmit}
						/>
					</DialogContent>
				</Dialog>
			</MemoryRouter>,
		);

		const token = screen.getByLabelText("API Token") as HTMLInputElement;
		expect(token.type).toBe("password");
		expect(token.value).toBe("");
		expect(token.placeholder).toBe("Leave blank to keep unchanged");
		expect(screen.queryByDisplayValue("********")).toBeNull();

		await user.click(screen.getByRole("button", { name: "Save" }));
		expect(onSubmit).toHaveBeenCalledWith(
			"production",
			{ team: "one-team", credentials: { apiToken: "********" } },
			false,
		);
	});
});
