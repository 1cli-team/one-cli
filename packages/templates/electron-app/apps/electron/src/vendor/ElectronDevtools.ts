import install, {
  REACT_DEVELOPER_TOOLS,
  REDUX_DEVTOOLS,
} from "electron-devtools-installer";
import isDev from "electron-is-dev";
import type { ElectronLogger } from "./ElectronLogger";

export function createElectronDevtools({ logger }: { logger: ElectronLogger }) {
  const init = async (): Promise<void> => {
    if (!isDev || !process.env.LOAD_DEVTOOLS) return;

    try {
      logger.debug("Loading devtools");
      await install([REDUX_DEVTOOLS, REACT_DEVELOPER_TOOLS]);
      logger.debug("Devtools loaded");
    } catch (err: unknown) {
      logger.error("Failed to load devtools", err);
    }
  };

  return { init };
}

export type ElectronDevtools = ReturnType<typeof createElectronDevtools>;
