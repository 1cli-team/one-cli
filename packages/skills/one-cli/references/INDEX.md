# Mode Index — Decision Tree

Use this when the user's intent isn't immediately clear from their
request. Otherwise route directly using the table in the main `SKILL.md`.

## Step 1 — Is the user inside a One workspace?

Walk upward from cwd looking for `one.manifest.json`.

- Found → workspace exists. Continue below.
- Not found → no workspace.
  - User wants to create one → `bootstrap.md`
  - User is confused → ask before mutating

## Step 2 — What's the user's intent?

| Intent signal | Mode |
|---|---|
| Asks command / JSON / schema / error code questions | `REFERENCE.md` |
| Wants to add a project from one of the built-in templates | `add-feature.md` |
| Needs to build / test / run and dependencies are missing | `dependencies.md` |
| Reports a problem ("manifest is out of sync", "container fails to build") | Read `one.manifest.json` and the relevant per-domain command in `REFERENCE.md`; re-run that domain's `one add` / `one container` / `one env` etc. |

## Step 3 — When the user truly just wants info

Read `REFERENCE.md` and answer from it. Do **not** run mutating
commands when the user is asking exploratory questions. Mistaking
"what does `one add` do?" for "I want to add something" is a common
failure mode.
