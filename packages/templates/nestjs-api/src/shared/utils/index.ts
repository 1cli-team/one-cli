// safeParseJSON
export function safeParseJSON(str?: string): any {
  if (!str) {
    return {};
  }
  try {
    return JSON.parse(str);
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
  } catch (e) {
    return {};
  }
}

export { logger, logInfo, logError, logWarn, logDebug } from "./logger";
