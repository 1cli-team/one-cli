import type { TFunction } from "i18next";

const stateKeys = new Map([
	["Backlog", "linear.workflowStates.backlog"],
	["Todo", "linear.workflowStates.todo"],
	["In Progress", "linear.workflowStates.inProgress"],
	["Done", "linear.workflowStates.done"],
	["Canceled", "linear.workflowStates.canceled"],
	["Duplicate", "linear.workflowStates.duplicate"],
	["In Review", "linear.workflowStates.inReview"],
]);

const issueLabelKeys = new Map([
	["Feature", "linear.issueLabels.feature"],
	["Bug", "linear.issueLabels.bug"],
	["Improvement", "linear.issueLabels.improvement"],
]);

export function workflowStateLabel(name: string, t: TFunction) {
	const key = stateKeys.get(name);
	return key ? t(key) : name;
}

export function issueLabelName(name: string, t: TFunction) {
	const key = issueLabelKeys.get(name);
	return key ? t(key) : name;
}
