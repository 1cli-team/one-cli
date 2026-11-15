package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
)

type Check struct {
	Group        string `json:"group"`
	Target       string `json:"target"`
	Kind         string `json:"kind"`
	URL          string `json:"url"`
	Status       string `json:"status"`
	HTTPStatus   int    `json:"http_status,omitempty"`
	Milliseconds int64  `json:"milliseconds"`
	Detail       string `json:"detail,omitempty"`
}

type probe struct{ target, kind, url, filename string }

// CheckSources checks the selected defaults, or previews a named preset.
// Explicit external configuration is reported as skipped: it can require auth
// or templates, and must never be sent to a public probe or printed verbatim.
func CheckSources(ctx context.Context, directory string, base []string, group, id string, timeout time.Duration) ([]Check, error) {
	if group != "" && !validGroup(group) {
		return nil, i18n.Errorf("mirror.group_unknown", group)
	}
	var r *Resolution
	if id != "" {
		if group == "" {
			group = EnvGroup
		}
		p, ok := preset(group, id)
		if !ok {
			return nil, i18n.Errorf("mirror.preset_unknown", id)
		}
		c := &Config{Version: 2}
		selected := &Selection{Preset: p.ID, Sources: p.Sources}
		if group == EnvGroup {
			c.Env = selected
		} else {
			c.Tools = selected
		}
		r = &Resolution{Config: c}
	} else {
		var err error
		r, err = Resolve(directory, base)
		if err != nil {
			return nil, err
		}
	}
	if timeout <= 0 || timeout > time.Minute {
		return nil, i18n.Errorf("mirror.timeout_invalid")
	}
	preserved := map[string]bool{}
	for _, entry := range r.Entries {
		preserved[entry.Target] = entry.Status == "preserved"
	}
	var requests []probe
	if group == "" || group == EnvGroup {
		requests = append(requests, probes(Current(r.Config, EnvGroup).Sources)...)
	}
	if group == "" || group == ToolsGroup {
		toolSources := Current(r.Config, ToolsGroup).Sources
		requests = append(requests, toolProbes(toolSources)...)
	}
	results := make([]Check, len(requests))
	client := &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || req.URL.User != nil || len(via) >= 10 {
			return i18n.Errorf("mirror.redirect_invalid")
		}
		return nil
	}}
	limit := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for index, request := range requests {
		if preserved[request.target] {
			results[index] = Check{Target: request.target, Kind: request.kind, Status: "skipped", Detail: i18n.T("mirror.check_preserved")}
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case limit <- struct{}{}:
				defer func() { <-limit }()
			case <-ctx.Done():
				results[index] = Check{Target: request.target, Kind: request.kind, Status: "failed", Detail: i18n.T("mirror.check_cancelled")}
				return
			}
			results[index] = checkEndpoint(ctx, client, request)
		}()
	}
	wg.Wait()
	for i := range results {
		results[i].Group = EnvGroup
		if results[i].Target == "mise" || results[i].Target == "process-compose" {
			results[i].Group = ToolsGroup
		}
	}
	return results, ctx.Err()
}

func probes(sources map[string]string) []probe {
	nodeOS, nodeArch, ext := runtime.GOOS, runtime.GOARCH, "tar.xz"
	if nodeArch == "amd64" {
		nodeArch = "x64"
	}
	if nodeOS == "windows" {
		nodeOS, ext = "win", "zip"
	}
	// Representative supported releases, not a promise about every version.
	nodeVersion, goVersion := "24.21.0", "1.27.1"
	nodeFile := fmt.Sprintf("node-v%s-%s-%s.%s", nodeVersion, nodeOS, nodeArch, ext)
	goExt := "tar.gz"
	if runtime.GOOS == "windows" {
		goExt = "zip"
	}
	goFile := fmt.Sprintf("go%s.%s-%s.%s", goVersion, runtime.GOOS, runtime.GOARCH, goExt)
	join := func(base, path string) string { return strings.TrimRight(base, "/") + "/" + path }
	proxy := strings.Split(sources["goproxy"], ",")[0]
	return []probe{
		{"node", "metadata", join(sources["node"], "index.json"), ""},
		{"node", "checksum", join(sources["node"], "v"+nodeVersion+"/SHASUMS256.txt"), nodeFile},
		{"node", "artifact", join(sources["node"], "v"+nodeVersion+"/"+nodeFile), ""},
		{"go", "artifact", join(sources["go"], goFile), ""},
		{"go", "checksum", "https://dl.google.com/go/" + goFile + ".sha256", ""},
		{"npm", "metadata", join(sources["npm"], "pnpm/latest"), ""},
		{"goproxy", "metadata", join(proxy, "golang.org/x/text/@v/list"), ""},
		{"pypi", "metadata", join(sources["pypi"], "pip/"), ""},
		{"pub", "metadata", join(sources["pub"], "api/packages/http"), ""},
		{"flutter", "metadata", join(sources["flutter"], "flutter_infra_release/releases/releases_"+flutterOS()+".json"), ""},
	}
}

func toolProbes(sources map[string]string) []probe {
	miseURL := miseProbeURL(sources["mise"])
	miseFile := miseURL[strings.LastIndex(miseURL, "/")+1:]
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	composeFile := fmt.Sprintf("process-compose_%s_%s.%s", runtime.GOOS, runtime.GOARCH, ext)
	base := strings.TrimRight(sources["process-compose"], "/")
	proxy := strings.TrimSuffix(base, composeReleases)
	return []probe{
		{"mise", "artifact", miseURL, ""},
		{"mise", "checksum", strings.TrimSuffix(miseURL, miseFile) + "SHASUMS256.txt", miseFile},
		{"process-compose", "metadata", proxy + "https://api.github.com/repos/F1bonacc1/process-compose/releases/tags/v" + runtimeport.ManagedComposeVersion, ""},
		{"process-compose", "registry", proxy + "https://raw.githubusercontent.com/aquaproj/aqua-registry/main/pkgs/F1bonacc1/process-compose/registry.yaml", ""},
		{"process-compose", "artifact", base + "/v" + runtimeport.ManagedComposeVersion + "/" + composeFile, ""},
		{"process-compose", "checksum", base + "/v" + runtimeport.ManagedComposeVersion + "/process-compose_checksums.txt", composeFile},
	}
}

func flutterOS() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

func miseProbeURL(base string) string {
	osName, arch, ext := runtime.GOOS, runtime.GOARCH, "tar.gz"
	if arch == "amd64" {
		arch = "x64"
	}
	if osName == "darwin" {
		osName = "macos"
	}
	if osName == "linux" {
		arch += "-musl"
	}
	if osName == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s/v%s/mise-v%s-%s-%s.%s", strings.TrimRight(base, "/"), runtimeport.ManagedMiseVersion, runtimeport.ManagedMiseVersion, osName, arch, ext)
}

func checkEndpoint(ctx context.Context, client *http.Client, p probe) Check {
	start := time.Now()
	c := Check{Target: p.target, Kind: p.kind, URL: p.url, Status: "failed"}
	finish := func(detail string) Check {
		c.Milliseconds = time.Since(start).Milliseconds()
		c.Detail = detail
		return c
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return finish(i18n.T("mirror.check_request"))
	}
	req.Header.Set("User-Agent", "one-cli/mirror-check")
	if p.kind == "artifact" {
		req.Header.Set("Range", "bytes=0-511")
	}
	resp, err := client.Do(req)
	if err != nil {
		// url.Error includes request URLs and may contain private credentials.
		var detail string
		if u, ok := err.(*url.Error); ok {
			detail = u.Err.Error()
		} else {
			detail = err.Error()
		}
		return finish(detail)
	}
	defer resp.Body.Close()
	c.HTTPStatus = resp.StatusCode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return finish(i18n.Tf("mirror.check_http", resp.StatusCode))
	}
	limit := int64(8 << 20)
	if p.kind == "artifact" {
		limit = 512
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return finish(i18n.T("mirror.check_body"))
	}
	valid := len(data) > 0
	if p.kind == "artifact" {
		valid = len(data) >= 4 && (string(data[:2]) == "PK" || data[0] == 0x1f && data[1] == 0x8b || data[0] == 0xfd && string(data[1:4]) == "7zX")
	} else if p.kind == "checksum" {
		if p.filename == "" {
			valid = regexp.MustCompile(`^[a-fA-F0-9]{64}\s*$`).Match(data)
		} else {
			valid = false
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.Fields(line)
				if len(parts) == 2 && strings.TrimPrefix(strings.TrimPrefix(parts[1], "*"), "./") == p.filename && regexp.MustCompile(`^[a-fA-F0-9]{64}$`).MatchString(parts[0]) {
					valid = true
				}
			}
		}
	} else {
		switch p.target {
		case "process-compose":
			if p.kind == "registry" {
				valid = strings.Contains(string(data), "repo_name: process-compose")
			} else {
				var release struct {
					Tag    string `json:"tag_name"`
					Assets []struct {
						Name string `json:"name"`
					} `json:"assets"`
				}
				valid = json.Unmarshal(data, &release) == nil && release.Tag == "v"+runtimeport.ManagedComposeVersion && len(release.Assets) > 0
			}
		case "node":
			var entries []map[string]any
			valid = json.Unmarshal(data, &entries) == nil && len(entries) > 0 && entries[0]["version"] != nil
		case "npm", "pub", "flutter":
			var doc map[string]any
			valid = json.Unmarshal(data, &doc) == nil
			key := "name"
			if p.target == "flutter" {
				key = "releases"
			}
			valid = valid && doc[key] != nil
		case "goproxy":
			valid = regexp.MustCompile(`(?m)^v[0-9]+\.[0-9]+\.[0-9]+`).Match(data)
		case "pypi":
			valid = strings.Contains(strings.ToLower(string(data)), "href=") && strings.Contains(strings.ToLower(string(data)), "pip-")
		}
	}
	if !valid {
		return finish(i18n.T("mirror.check_content"))
	}
	c.Status = "ok"
	return finish("")
}
