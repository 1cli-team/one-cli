import { useId, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { errorText } from "@/features/linear/api";
import { answerExecution, type Execution, type ExecutionQuestion } from "./executions";

export function GatewayQuestions({
	job,
	onAnswered,
}: {
	job: Pick<Execution, "id" | "state" | "questions">;
	onAnswered(question: ExecutionQuestion): void;
}) {
	const { t } = useTranslation();
	const questions = job.questions ?? [];
	if (!questions.length) return null;
	const waiting = job.state === "waiting_for_answer";
	const current = questions.at(-1);
	return (
		<section className="flex flex-col gap-2" aria-label={t("gateway.execution.questions.title")}>
			<h3 className="text-sm font-medium">{t("gateway.execution.questions.title")}</h3>
			{questions.map((question) => (
				<div key={question.id} className="grid gap-3 rounded-md border border-border p-3">
					<p className="whitespace-pre-wrap wrap-anywhere text-sm">{question.text}</p>
					{question.answer && (
						<div className="grid gap-1 border-t border-border pt-3 text-sm">
							<p className="font-medium">{t("gateway.execution.questions.answer")}</p>
							<p className="whitespace-pre-wrap wrap-anywhere">{question.answer}</p>
						</div>
					)}
					{waiting &&
						question.id === current?.id &&
						(question.answer ? (
							<p role="status" className="text-sm text-muted-foreground">
								{t("gateway.execution.questions.received")}
							</p>
						) : (
							<AnswerForm jobId={job.id} questionId={question.id} onAnswered={onAnswered} />
						))}
				</div>
			))}
		</section>
	);
}

function AnswerForm({
	jobId,
	questionId,
	onAnswered,
}: {
	jobId: string;
	questionId: string;
	onAnswered(question: ExecutionQuestion): void;
}) {
	const { t } = useTranslation();
	const id = useId();
	const [answer, setAnswer] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");

	async function submit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		if (busy || !answer.trim()) return;
		setBusy(true);
		setError("");
		try {
			onAnswered(await answerExecution(jobId, questionId, answer.trim()));
		} catch (failure) {
			setError(errorText(failure));
		} finally {
			setBusy(false);
		}
	}

	return (
		<form className="grid gap-2" onSubmit={(event) => void submit(event)}>
			<Label htmlFor={id}>{t("gateway.execution.questions.answer")}</Label>
			<Textarea
				id={id}
				value={answer}
				maxLength={8192}
				required
				disabled={busy}
				onChange={(event) => setAnswer(event.target.value)}
			/>
			{error && (
				<p role="alert" className="wrap-anywhere text-sm text-destructive">
					{error}
				</p>
			)}
			<Button
				type="submit"
				size="sm"
				className="justify-self-end"
				disabled={busy || !answer.trim()}
			>
				{t(busy ? "gateway.execution.questions.sending" : "gateway.execution.questions.send")}
			</Button>
		</form>
	);
}
