# Linear

The compiled-in Linear plugin connects One Desktop home projects to a Linear project. Each One group or standalone workspace has its own binding and default team. A group shares its task list across member repositories. Workspace aliases resolve to the same binding.

## Setup

Open **Settings → Linear connection**. Create an OAuth application on [Linear’s application creation page](https://linear.app/settings/api/applications/new) and register the redirect URI displayed by One. The default is `http://127.0.0.1:39127/oauth/linear/callback`. Enter the application's public Client ID, save, and connect. Keep One running while completing authorization in the browser. A different local port can be configured if the default is occupied.

Authorization uses PKCE S256 with a one-use state and a ten-minute timeout. The requested scopes are `read`, `issues:create`, and `comments:create`. A client secret is unnecessary for this PKCE flow. Tokens rotate under a cross-process file lock and are saved together using atomic replacement.

Local credentials are stored in `ONE_HOME/config/linear/connections.json`, with a private directory and mode `0600` for the file. This follows One's local credential storage pattern. Project bindings are stored in `ONE_HOME/config/issue-bindings.json`. These files are machine-local and the connection file contains credentials. Disconnect removes the local connection. The authorization can also be revoked in Linear's application settings.

## Project and Issue management

Open a home project's **Issues** page, choose a connected account, Linear project, and default team. Available projects use cursor pagination. Teams, workflow statuses, members, and labels are loaded from Linear. Team choices come from the selected project. Add a new team to that project in Linear before selecting it in One. Refresh in the binding form reloads project choices and project teams. Refresh on the Issues page reloads the live project, team metadata, and Issue list; saved bindings provide identity and the default team without freezing the displayed team list.

The Issue list supports title search, team, status, assignee, and priority filters. Select a team to load its status and assignee options. Issues and comments use cursor pagination. The detail panel supports creating and editing title, Markdown description, status, assignee, priority, labels, and posting comments. Editing preserves the current team. Reloading latest content refreshes an edit that conflicts with a remote update. The pre-save `updatedAt` check detects changes already present when validation starts. Linear's update mutation does not provide atomic compare-and-swap here, so simultaneous changes after that check can still race.

Read failures caused by network errors or rate limiting can display an explicitly dated, in-memory cached list. Cache entries are scoped to the connection, organization, local project, and filters. Restarting One clears the cache.

Creating an Issue or comment persists an operation UUID before submitting it. The same UUID becomes the Linear resource ID. An uncertain response preserves the operation for readback and prevents automatic replay. Pending operations appear on the project's Issues page and survive a restart. **Check result** only queries Linear. If Linear still has no visible resource, the operation remains unconfirmed. Preserve that operation ID for diagnosis before submitting another item.

## CLI

```sh
one linear configure --client-id PUBLIC_CLIENT_ID
one linear login
one linear projects
one linear bind --kind group --scope ONE_GROUP_ID --project-id LINEAR_PROJECT_ID --team-id LINEAR_TEAM_ID
one linear issues --kind group --scope ONE_GROUP_ID
one linear issue LINEAR_ISSUE_ID --kind group --scope ONE_GROUP_ID
one linear create --kind group --scope ONE_GROUP_ID --input issue.json
one linear update LINEAR_ISSUE_ID --kind group --scope ONE_GROUP_ID --input issue.json
one linear comment LINEAR_ISSUE_ID --kind group --scope ONE_GROUP_ID --input comment.json
one linear pending --kind group --scope ONE_GROUP_ID
one linear operation OPERATION_UUID --kind group --scope ONE_GROUP_ID
```

Use `--connection` when multiple accounts are connected. Use `--kind workspace` with the registered workspace entry ID for a standalone workspace. Creation defaults to the binding's team and generates an operation UUID when omitted.

Issue JSON uses `title`, `description`, `teamId`, `stateId`, `assigneeId`, `priority`, and `labelIds`. Updates require the `expectedUpdatedAt` value from the latest Issue read. Empty `assigneeId` clears assignment and an empty `labelIds` array clears labels. Comment JSON uses `body` and optionally `operationId`. Reusing an operation ID with different content is rejected.

## Verification

Run `one run linear:check` for the plugin, binding, CLI input, and HTTP regression tests with Go's race detector. The tests use isolated local servers and temporary stores. A real OAuth application and account are required to accept the live authorization and mutation flow. UI acceptance should cover connecting, binding, creating an Issue, changing its status and assignee, posting a comment, and reconnecting after revocation.

API behavior follows [Linear's OAuth documentation](https://linear.app/developers/oauth-2-0-authentication) and [GraphQL documentation](https://linear.app/developers/graphql). UUID creation fields were checked against the official `@linear/sdk` 102.0.0 type definitions.
