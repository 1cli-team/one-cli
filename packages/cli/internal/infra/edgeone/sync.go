package edgeone

// sync.go scaffolds a project-side `edgeone.json` hint the first time
// a project picks deploy/edgeone. The CLI doesn't strictly require
// this file, but pre-seeding the project name + asset directory keeps
// the first-run flow non-interactive.
//
// NOTE: edgeone CLI's exact config-file shape is not yet stabilized;
// the schema below is the minimum that CI runs survive. If the CLI
// ever rejects unknown keys, narrow this further.

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// EdgeOneConfigFilename is the canonical config file edgeone reads.
const EdgeOneConfigFilename = "edgeone.json"

// ShouldSync reports whether a Sync call would do work — only when no
// edgeone.json is present (we never overwrite a hand-edited file).
func ShouldSync(projectDir string) bool {
	_, err := os.Stat(filepath.Join(projectDir, EdgeOneConfigFilename))
	return errors.Is(err, fs.ErrNotExist)
}

// Sync writes a minimal edgeone.json hint. Idempotent: existing files
// are left alone. projectName / outputDir are persisted as the
// project's identity in the EdgeOne console; outputDir defaults to
// "dist" when empty.
func Sync(projectDir, templateID, projectName string) error {
	target := filepath.Join(projectDir, EdgeOneConfigFilename)
	if !ShouldSync(projectDir) {
		return nil
	}
	cfg := defaultConfig(templateID, projectName)
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(target, raw, 0o644)
}

// defaultConfig maps One CLI template ids to edgeone.json hints.
func defaultConfig(templateID, projectName string) map[string]any {
	out := map[string]any{}
	if projectName != "" {
		out["projectName"] = projectName
	}
	if outputDir := defaultOutputDir(templateID); outputDir != "" {
		out["outputDir"] = outputDir
	}
	return out
}

func defaultOutputDir(templateID string) string {
	switch templateID {
	case "react-spa", "astro-site", "starlight-docs":
		return "dist"
	case "nextjs-app":
		// Next.js on EdgeOne typically goes through edge-runtime build;
		// `.next` is the default output.
		return ".next"
	default:
		return ""
	}
}
