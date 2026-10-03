---
title: Login and local settings
description: Browser login, system keyring storage, and shared Infisical credentials.
---

## Browser login

```bash
one login
one whoami
one logout
one login --site-url https://secrets.example.com
```

One keeps one active Infisical account. Complete login in the browser; the session token is stored in the system keyring. Client ID, Client Secret, and Profiles are no longer used. Log out before changing accounts or instances. An unavailable keyring causes an error with no plaintext fallback. Expired sessions require explicit login; reading variables never launches a browser automatically. Old credential files are neither read nor automatically deleted.

## Shared credentials

Both the Dashboard and `one login` prepare shared credentials after successful authentication. They reuse an accessible `shared-credentials` project in the current instance and organization, or create it if absent, with `dev` as the initial default environment. A project created by another account can be reused; an old account's local binding is updated silently. Legacy bindings to differently named projects switch to `shared-credentials`; the original projects and credentials are retained without migration or deletion.

Bindings are stored locally per instance, account and organization. Signing in again preserves a valid default environment for that identity. Signing out clears the session and displayed values and cancels preparation, while retaining bindings and remote data. `one serve` also checks an existing signed-in session at startup. Permission, network or preparation failures do not undo authentication. The Dashboard shows the reason and a retry action; use `one env bind --global` for CLI recovery.

Use `--global` to access shared credentials. `one env bind --global --env ENV` changes the default environment, which must exist remotely. `--project-id` remains an advanced compatibility option; subsequent login or Dashboard startup reapplies automatic binding to `shared-credentials`:

```bash
one env bind --global
one env bind --global --project-id PROJECT_ID --env dev
one env --global
one env list --global --env dev --path /
one env list --global --env dev --path /docker
one exec --global --env dev --path /docker --keys REGISTRY_USER,REGISTRY_PASSWORD -- docker-push-script
```

Listings contain immediate folders, names, and descriptions, without values. Execution requires an explicit environment and path. It does not recurse, import other folders, or expand secret references. Use `--keys` to narrow injection further. Global mode works outside a workspace and preserves the current directory. Project commands, `one dev`, and `one build` do not automatically receive shared credentials.

The CLI never displays secret values. Inject shared credentials with `one exec --global --env dev --path /docker -- command`. Write with the interactive password prompt or `one env set KEY --global --env dev --path /docker --stdin`. Overwrites require `--yes`. Delete with `one env unset KEY --global --env dev --path /docker`.

## Dashboard

Run `one serve`. Settings manages browser login, pending callbacks, cancellation, logout, and language. Shared credentials manages the storage project, browsing environment, folders, and variables. Values are fetched only on reveal or copy and cleared when the account, environment, folder, or page changes. Remote edits take effect immediately. Workspace bindings are reviewed and confirmed in the **Bind Infisical** dialog. Save or discard pending configuration first; stale configuration requires a refresh. Other Manifest changes keep the draft review and save workflow.

## Security boundaries

One's own messages and structured results omit injected values. Child logs pass through unchanged, with no secret scanning or replacement; values printed by a child can appear in the terminal and log files. Injection does not isolate an Agent running as the same OS user. Limit Infisical and cloud permissions, folders, environments, and credential lifetime to the task. External tools such as Docker may persist credentials themselves.

## Preferences and workspace tools

```bash
one locale en-US
one locale zh-CN
one locale auto
one init mise --dry-run
one init mise
one init hooks --dry-run
one init hooks
```

`one init mise` generates tool configuration while preserving user configuration. `one init hooks` configures hk checks and Git hooks for the current checkout. `one mise` and `one hk` continue to forward tool commands. Language preferences are stored locally.
