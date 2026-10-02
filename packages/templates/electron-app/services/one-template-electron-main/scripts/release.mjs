import { createRequire } from "node:module";
import path from "node:path";
import builder from "electron-builder";
import { prepareResources } from "./resources.mjs";

const root = path.resolve(import.meta.dirname, "..");
const require = createRequire(import.meta.url);
const rendererDir = path.join(
  path.dirname(require.resolve("@one-template-electron/renderer/package.json")),
  "dist",
);
const preloadEntry = require.resolve("@one-template-electron/preload");
const development = process.env.NODE_ENV === "development";

await prepareResources({ mainDir: root, rendererDir, preloadEntry });

/** @type {import('electron-builder').Configuration} */
const config = {
  productName: process.env.APP_NAME,
  appId: process.env.APP_ID,
  copyright: process.env.APP_COPYRIGHT,
  artifactName: "${productName}-setup-${version}.${ext}",
  directories: { output: "dist" },
  files: ["build/**/*", "assets/**/*", "package.json"],
  win: { icon: path.join(root, "assets/icon.ico"), target: ["nsis"] },
  mac: { target: [{ target: "default", arch: ["x64", "arm64"] }] },
  nsis: {
    oneClick: false,
    allowElevation: true,
    allowToChangeInstallationDirectory: true,
    createDesktopShortcut: true,
    createStartMenuShortcut: true,
  },
  publish: {
    provider: "github",
    repo: "repo",
    owner: "owner",
    releaseType: "draft",
  },
};

await builder.build({
  projectDir: root,
  targets: builder.Platform.current().createTarget(
    development ? builder.DIR_TARGET : builder.DEFAULT_TARGET,
  ),
  publish: development ? "never" : undefined,
  config,
});
