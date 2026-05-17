---
title: one templates
description: List available templates and their metadata.
---

`one templates` lists every template in the registry. In a terminal it prints human-readable output; when piped or used with `-o json`, it returns structured data.

## Usage

```bash
one templates [-o <fmt>]
```

## Arguments

| Argument | Description |
|---|---|
| `-o, --output <fmt>` | `json` / `yaml` / `text`; default is TTY-aware auto detection |

## Interactive Mode

`one templates` does not have a wizard. It only lists templates. Humans see readable terminal output; scripts and agents use `-o json` to read template IDs, categories, and compatible backends.

To choose a template interactively, use [`one add`](/en/docs/add/).

## Output (schema `one-cli/templates/v1`)

```jsonc
{
  "schema": "one-cli/templates/v1",
  "templates": [
    {
      "id": "nestjs-api",
      "code": "ne",
      "category": "backend",
      "name": "NestJS API service",
      "description": "NestJS + TypeScript for API services and business backends",
      "toolchain": "node",
      "tags": ["api", "nestjs", "typescript", "backend"],
      "domains": {
        "container": { "default": "docker", "compat": ["docker"] },
        "deploy": { "default": "kustomize", "compat": ["kustomize"] }
      }
    },
    {
      "id": "go-api",
      "code": "go",
      "category": "backend",
      "name": "Go API service",
      "toolchain": "go"
    },
    // ... 8 more
  ]
}
```

## Complete Template List

The current registry has 10 templates:

| ID | Category | Details |
|---|---|---|
| `nestjs-api` | backend | [->](/en/docs/templates/) |
| `go-api` | backend | [->](/en/docs/templates/) |
| `astro-site` | frontend | [->](/en/docs/templates/) |
| `starlight-docs` | frontend / docs | [->](/en/docs/templates/) |
| `nextjs-app` | frontend | [->](/en/docs/templates/) |
| `react-spa` | frontend | [->](/en/docs/templates/) |
| `expo-mobile` | frontend / mobile | [->](/en/docs/templates/) |
| `ts-library` | library | [->](/en/docs/templates/) |
| `go-lib` | library | [->](/en/docs/templates/) |
| `electron-app` | frontend / desktop | [->](/en/docs/templates/) |

Not sure which one to pick? Read the [template decision tree](/en/docs/templates/).

## Examples

### Human Output

```bash
one templates
```

### Agent / Script

```bash
# All template IDs
one templates -o json | jq -r '.templates[].id'

# Filter by category
one templates -o json | jq '.templates[] | select(.category == "backend")'

# Inspect a selected template
one templates -o json | jq '.templates[] | select(.id == "nestjs-api")'
```

## Common Errors

| Code | Recovery |
|---|---|
| `REGISTRY_FETCH_FAILED` | Network issue; inspect the registry URL in context |
| `REGISTRY_INVALID` | Registry JSON is malformed; contact the maintainer |
| `REGISTRY_NOT_FOUND` | Registry path does not exist |
| `NO_TEMPLATES` | Registry is empty; this should not normally happen |

Full table: [Error codes](/en/docs/error-codes/).

## Next

- [Template decision tree](/en/docs/templates/) — choose the right template
- [`one add`](/en/docs/add/) — add the selected template to a workspace
