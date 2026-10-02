"use client";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { messages, type Locale } from "@/lib/i18n";

export function NoticeButton({ locale }: { locale: Locale }) {
	const text = messages[locale];
	return (
		<Button
			variant="outline"
			onClick={() =>
				toast.add({ title: text.notice, description: text.noticeDescription, type: "success" })
			}
		>
			{text.notify}
		</Button>
	);
}
