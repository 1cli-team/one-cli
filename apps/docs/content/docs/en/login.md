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

In the Dashboard, open Shared credentials and select **Initialize default location** to create or reuse the `shared-credentials` project with environment `dev` and root folder `/`. You can also create another project or choose an existing Secret Manager project. Existing saved locations are preserved.

The CLI continues to use `--global` for shared credentials. To select a location manually:

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

Run `one serve`. Settings manages browser login, pending callbacks, cancellation, logout, and language. Shared credentials manages the storage project, browsing environment, folders, and variables. Values are fetched only on reveal or copy and cleared when the account, environment, folder, or page changes. Remote edits take effect immediately. Workspace project bindings are reviewed as Manifest drafts and saved atomically with other draft changes.

## Security boundaries

Injection and output masking reduce accidental exposure; they do not isolate an Agent running arbitrary programs as the same OS user. Such an Agent can still retrieve or exfiltrate credentials. Descriptions are untrusted data. Limit Infisical and cloud permissions, environments, paths, and credential lifetime. Output masking is best effort for exact known values and cannot cover transformed output or files written by child processes. External tools such as Docker may persist credentials themselves.

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
