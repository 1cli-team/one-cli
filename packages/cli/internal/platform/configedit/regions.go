package configedit

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

const regionMetadata = "// one:managed-v1 "

var regionMarker = regexp.MustCompile(`(?m)^[ \t]*// one:(begin|end|insert) ([^\s]+)[ \t]*\r?$`)

type region struct {
	span
	body []byte
}
type regions struct {
	entries  map[string]region
	anchors  map[string]int
	state    map[string]string
	metadata *span
}

// Regions updates marked Pkl entries, leaving all text outside them untouched.
// The footer retains the previous generated hashes, including deleted entries,
// so user edits and deletions remain effective until defaults also change.
func Regions(before, desired []byte) ([]byte, error) {
	next, err := parseRegions(desired)
	if err != nil {
		return nil, err
	}
	old, err := parseRegions(before)
	if err != nil {
		return nil, err
	}
	if len(before) > 0 && old.metadata == nil {
		if len(old.entries) > 0 {
			return nil, i18n.Errorf("config.metadata_invalid")
		}
		// An existing unmarked configuration is wholly user-owned.
		return before, nil
	}
	if len(before) == 0 {
		for id, entry := range next.entries {
			next.state[id] = fingerprint(string(entry.body), true)
		}
		state, _ := json.Marshal(next.state)
		return append(bytes.Clone(desired), []byte("\n"+regionMetadata+string(state)+"\n")...), nil
	}
	raw := splice(before, old.metadata.start, old.metadata.end, nil)
	original := bytes.Clone(raw)
	stateBefore, _ := json.Marshal(old.state)
	keys := map[string]bool{}
	for id := range next.entries {
		keys[id] = true
	}
	for id := range old.state {
		keys[id] = true
	}
	sorted := make([]string, 0, len(keys))
	for id := range keys {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	for _, id := range sorted {
		baseline, owned := old.state[id]
		current, present := old.entries[id]
		want, exists := next.entries[id]
		currentHash, nextHash := fingerprint(string(current.body), present), fingerprint(string(want.body), exists)
		if owned && currentHash != baseline {
			if nextHash != baseline && currentHash != nextHash {
				return nil, i18n.Errorf("config.entry_conflict", id)
			}
			if currentHash == nextHash {
				if exists {
					old.state[id] = nextHash
				} else {
					delete(old.state, id)
				}
			}
			continue
		}
		if !owned && present {
			continue
		}
		if currentHash != nextHash {
			doc, err := parseRegions(raw)
			if err != nil {
				return nil, err
			}
			if entry, ok := doc.entries[id]; ok {
				var replacement []byte
				if exists {
					replacement = desired[want.start:want.end]
				}
				raw = splice(raw, entry.start, entry.end, replacement)
			} else if exists {
				group, _, _ := strings.Cut(id, "/")
				position, ok := doc.anchors[group]
				if !ok {
					return nil, i18n.Errorf("config.entry_conflict", id)
				}
				// Do not introduce duplicate keys if a user has already added this entry.
				_, key, _ := strings.Cut(id, "/")
				keyJSON, _ := json.Marshal(key)
				if group == "steps" || group == "hooks" {
					pattern := regexp.MustCompile(`(?m)^\s*\[\s*` + regexp.QuoteMeta(string(keyJSON)) + `\s*\]\s*(?:=|\{)`)
					if pattern.Match(raw) {
						return nil, i18n.Errorf("config.entry_conflict", id)
					}
				}
				raw = splice(raw, position, position, desired[want.start:want.end])
			}
		}
		if exists {
			old.state[id] = nextHash
		} else {
			delete(old.state, id)
		}
	}
	stateAfter, _ := json.Marshal(old.state)
	if bytes.Equal(original, raw) && bytes.Equal(stateBefore, stateAfter) {
		return before, nil
	}
	raw = bytes.TrimRight(raw, "\r\n")
	return append(raw, []byte("\n\n"+regionMetadata+string(stateAfter)+"\n")...), nil
}

func parseRegions(raw []byte) (regions, error) {
	doc := regions{entries: map[string]region{}, anchors: map[string]int{}, state: map[string]string{}}
	active := ""
	start, bodyStart := 0, 0
	for _, match := range regionMarker.FindAllSubmatchIndex(raw, -1) {
		kind, id := string(raw[match[2]:match[3]]), string(raw[match[4]:match[5]])
		end := match[1]
		if end < len(raw) && raw[end] == '\n' {
			end++
		}
		switch kind {
		case "begin":
			if active != "" {
				return doc, i18n.Errorf("config.metadata_invalid")
			}
			if _, exists := doc.entries[id]; exists {
				return doc, i18n.Errorf("config.metadata_invalid")
			}
			active, start, bodyStart = id, match[0], end
		case "end":
			if id != active {
				return doc, i18n.Errorf("config.metadata_invalid")
			}
			doc.entries[id] = region{span{start, end}, raw[bodyStart:match[0]]}
			active = ""
		case "insert":
			if active != "" {
				return doc, i18n.Errorf("config.metadata_invalid")
			}
			if _, exists := doc.anchors[id]; exists {
				return doc, i18n.Errorf("config.metadata_invalid")
			}
			doc.anchors[id] = match[0]
		}
	}
	if active != "" {
		return doc, i18n.Errorf("config.metadata_invalid")
	}
	offset := 0
	for _, line := range bytes.SplitAfter(raw, []byte("\n")) {
		if bytes.HasPrefix(line, []byte(regionMetadata)) {
			if doc.metadata != nil {
				return doc, i18n.Errorf("config.metadata_invalid")
			}
			end := offset + len(bytes.TrimRight(line, "\r\n"))
			doc.metadata = &span{offset, end}
			if err := json.Unmarshal(raw[offset+len(regionMetadata):end], &doc.state); err != nil || doc.state == nil {
				return doc, i18n.Errorf("config.metadata_invalid")
			}
		}
		offset += len(line)
	}
	return doc, nil
}
