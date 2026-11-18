export function knowledgeErrorMessage(error: unknown, fallback: string) {
	return error &&
		typeof error === "object" &&
		"message" in error &&
		typeof error.message === "string"
		? error.message
		: fallback;
}
