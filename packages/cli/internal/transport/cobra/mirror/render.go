package mirrorcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"golang.org/x/term"
)

func (r *result) RenderTTY(w io.Writer) {
	layout := mirrorLayout{writer: w, width: 100}
	if terminal, ok := w.(interface{ Fd() uintptr }); ok {
		if width, _, err := term.GetSize(int(terminal.Fd())); err == nil && width > 1 {
			layout.width = width - 1
		}
	}
	if r.Action == "use" || r.Action == "reset" {
		layout.renderChange(r)
		return
	}
	for index, group := range groups(r.Group) {
		if index > 0 {
			fmt.Fprintln(w)
		}
		title := i18n.T("mirror.group." + group)
		if value, ok := r.Groups[group]; ok {
			title += " · " + presetLabel(value)
		}
		layout.line(title, 0)
		for _, preset := range r.Presets[group] {
			fmt.Fprintln(w)
			layout.line(preset.ID+" · "+presetLabel(preset.ID), 2)
			for _, key := range sourceKeys(group) {
				layout.source(i18n.T("mirror.target."+key), preset.Sources[key], "")
			}
		}
		for _, entry := range r.Entries {
			if entry.Group != group {
				continue
			}
			preserved := ""
			if entry.Status == "preserved" {
				preserved = entry.Source
			}
			layout.source(i18n.T("mirror.target."+entry.Target), entry.URL, preserved)
		}
		for _, check := range r.Checks {
			if check.Group != group {
				continue
			}
			fmt.Fprintln(w)
			layout.line(fmt.Sprintf("%s · %s: %s (%d ms)", i18n.T("mirror.target."+check.Target), i18n.T("mirror.kind."+check.Kind), i18n.T("mirror.check."+check.Status), check.Milliseconds), 2)
			if check.Status != "ok" {
				layout.line(check.URL, 4)
				layout.line(check.Detail, 4)
			}
		}
	}
	if r.Action == "summary" || r.Action == "test" {
		return
	}
	fmt.Fprintln(w)
	layout.line(i18n.T("mirror.scope.short"), 0)
	if r.ConfigPath != "" {
		layout.line(i18n.Tf("mirror.path", r.ConfigPath), 0)
	}
}

type mirrorLayout struct {
	writer io.Writer
	width  int
}

func (l mirrorLayout) renderChange(r *result) {
	group := ""
	if r.Group != "" {
		group = i18n.T("mirror.group." + r.Group)
	}
	key := "mirror.applied"
	if r.DryRun {
		key = "mirror.preview"
	}
	message := ""
	if r.Action == "reset" {
		key = "mirror.reset.applied"
		if r.DryRun {
			key = "mirror.reset.preview"
		}
		if r.Group == "" {
			message = i18n.T(key)
		} else {
			message = i18n.Tf(key+"_group", group)
		}
	} else {
		message = i18n.Tf(key, group, presetLabel(r.Preset))
	}
	marker := "✓ "
	if r.DryRun {
		marker = ""
	}
	l.line(marker+message, 0)
	if !r.DryRun {
		l.line(i18n.T("mirror.scope.short"), 2)
	}
	command := "one mirror"
	if r.Group != "" {
		command += " " + r.Group
	}
	command += " status"
	l.line(i18n.Tf("mirror.details", command), 2)
}

func (l mirrorLayout) source(name, url, preserved string) {
	fmt.Fprintln(l.writer)
	l.line(name, 2)
	l.line(url, 4)
	if preserved != "" {
		l.line(i18n.Tf("mirror.preserved", preserved), 4)
	}
}

func (l mirrorLayout) line(text string, indent int) {
	if text == "" {
		return
	}
	indent = min(indent, max(0, l.width-1))
	for _, line := range strings.Split(ansi.Wrap(text, max(1, l.width-indent), ""), "\n") {
		fmt.Fprintln(l.writer, strings.Repeat(" ", indent)+line)
	}
}
