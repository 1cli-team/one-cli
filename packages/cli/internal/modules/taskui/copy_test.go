package taskui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"errors"
	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"strings"
	"testing"
)

func TestCopyLogicalRangesKeepUnicodeBlankLinesAndNoSoftBreaks(t *testing.T) {
	source := []string{"\x1b[32m中文🧑‍💻é URL\x1b[0m", "", "  indented"}
	if got := textRange(source, cursor{}, cursor{2, 10}); got != "中文🧑‍💻é URL\n\n  indented" {
		t.Fatalf("lost Unicode or indentation: %q", got)
	}
	frame := &copyFrame{lines: source, selected: true, anchor: cursor{0, 4}, head: cursor{0, 5}}
	a, b := frame.selection()
	if got := textRange(source, a, b); got != "🧑‍💻" {
		t.Fatalf("split emoji: %q", got)
	}
	frame.anchor, frame.head = frame.head, frame.anchor
	a, b = frame.selection()
	if got := textRange(source, a, b); got != "🧑‍💻" {
		t.Fatal("reverse drag changed text", got)
	}
	logs := testStore(t)
	text := strings.Repeat("中文é🧑‍💻", 30)
	writeLog(t, newLogWriter(logs), text+"\n")
	m := newModel(logs, Graph{})
	m.width = 45
	m.height = 100
	m.freeze()
	m.copy.selected = true
	m.copy.anchor = cursor{}
	m.copy.head = cursor{0, ansi.StringWidth(text)}
	if got := m.selectedText(); got != text || strings.Contains(got, "\n") {
		t.Fatal("soft wraps became real newlines", got)
	}
	before := m.selectedText()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(model)
	if m.selectedText() != before {
		t.Fatal("resize changed logical selection")
	}
}
func TestDragSelectionFreezesProgressAndResumeReadsNewLogs(t *testing.T) {
	logs := testStore(t)
	writer := newLogWriter(logs)
	writeLog(t, writer, "before\nprogress 中文")
	m := newModel(logs, Graph{})
	left, _, _ := m.layout()
	y := len(m.headerLines())
	updated, _ := m.Update(tea.MouseClickMsg{X: left + 2, Y: y, Button: tea.MouseLeft})
	m = updated.(model)
	updated, _ = m.Update(tea.MouseMotionMsg{X: left + 7, Y: y, Button: tea.MouseLeft})
	m = updated.(model)
	updated, _ = m.Update(tea.MouseReleaseMsg{X: left + 7, Y: y, Button: tea.MouseLeft})
	m = updated.(model)
	if got := m.selectedText(); got != "before" {
		t.Fatal("mouse coordinates copied decorations", got)
	}
	frozen := plainPage(m)
	writeLog(t, writer, "\rchanged\nnew output\n")
	updated, _ = m.Update(tickMsg{})
	m = updated.(model)
	if plainPage(m) != frozen || m.selectedText() != "before" {
		t.Fatal("new output changed selected snapshot")
	}
	m = key(m, 'c', "c")
	if view := m.View(); view.MouseMode != tea.MouseModeNone || strings.Contains(view.Content, "new output") || strings.Contains(view.Content, "│ ") {
		t.Fatal("native copy not a fixed full-width log view")
	}
	m = key(m, tea.KeyEscape, "")
	if m.View().MouseMode != tea.MouseModeCellMotion || !strings.Contains(plainPage(m), "new output") {
		t.Fatal("resume did not restore live logs and mouse")
	}
}
func TestCopyMenuHistorySearchAndFailure(t *testing.T) {
	logs := testStore(t, "//:api")
	writer := newLogWriter(logs)
	writeLog(t, writer, "[//:api\t] old 中文🧑‍💻\n[//:api\t] unmatched\n[//:api\t] latest\n")
	m := newModel(logs, Graph{Tasks: []Task{{Name: "//:api"}}})
	m.selected = 1
	var copied string
	m.clipboard = func(_ context.Context, text string) (bool, error) { copied = text; return false, nil }
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	m = updated.(model)
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	m = updated.(model)
	updated, _ = m.Update(cmd())
	m = updated.(model)
	if copied != "//:api" || m.copyNote != i18n.T("tasks.ui.copied") {
		t.Fatal("full name copy failed")
	}
	m.query = "old"
	m.matches = []int{0}
	cmd = m.historyCopyCmd()
	writeLog(t, writer, "[//:api\t] old after request\n")
	cmd()
	if !strings.Contains(copied, "old 中文🧑‍💻") || strings.Contains(copied, "unmatched") || strings.Contains(copied, "after request") || strings.Contains(copied, "\x1b") {
		t.Fatal("history snapshot/search leaked records", copied)
	}
	m.query = ""
	m.freeze()
	m.copy.selected = true
	m.copy.anchor = cursor{}
	m.copy.head = cursor{0, 3}
	selection := m.selectedText()
	m.clipboard = func(context.Context, string) (bool, error) { return false, errors.New("clipboard unavailable") }
	cmd = m.clipboardCmd(selection)
	updated, _ = m.Update(cmd())
	m = updated.(model)
	if m.selectedText() != selection || !strings.Contains(m.copyNote, "clipboard unavailable") {
		t.Fatal("copy failure lost selection or message")
	}
	m.clipboard = func(context.Context, string) (bool, error) { return true, nil }
	updated, cmd = m.Update(m.clipboardCmd(selection)())
	m = updated.(model)
	if cmd == nil || m.copyNote != i18n.T("tasks.ui.copy_requested") {
		t.Fatal("OSC request incorrectly reported success")
	}
	m.search = true
	m.editQuery = ""
	m = key(m, 'y', "y")
	m = key(m, 'c', "c")
	if m.editQuery != "yc" {
		t.Fatal("copy intercepted search input")
	}
}
func TestCopySameTaskAndFullHistoryScope(t *testing.T) {
	graph := sharedGraph()
	logs := testStore(t, graph.names()...)
	writer := newLogWriter(logs)
	for range 100 {
		writeLog(t, writer, "[//:prepare\t] shared\n[//:web:dev\t] other\n")
	}
	m := newModel(logs, graph)
	m.selected = m.tree.primary["//:prepare"]
	m.freeze()
	frame := m.copy
	for id, node := range m.tree.nodes {
		if node.reference && node.task == m.task() {
			m.selectTask(id)
		}
	}
	if m.copy != frame {
		t.Fatal("same task reference cleared selection")
	}
	var text string
	m.clipboard = func(_ context.Context, value string) (bool, error) { text = value; return false, nil }
	m.historyCopyCmd()()
	if strings.Count(text, "shared") != 100 || strings.Contains(text, "other") {
		t.Fatal("history truncated or mixed tasks")
	}
	m.selectTask(m.tree.primary["//:web:dev"])
	if m.copy != nil {
		t.Fatal("selection leaked to another task")
	}
}
func TestRemoteClipboardLimitAndContext(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "remote")
	remote, err := systemClipboard(context.Background(), "safe 中文")
	if !remote || err != nil {
		t.Fatal("remote used desktop clipboard", err)
	}
	_, err = systemClipboard(context.Background(), strings.Repeat("x", 100*1024+1))
	if err == nil {
		t.Fatal("large OSC request was silently truncated")
	}
}

func TestFrozenCopyLayoutsFitTinyTerminalsAndBothLocales(t *testing.T) {
	old := i18n.Active()
	defer i18n.Init(old)
	for _, locale := range []string{"en-US", "zh-CN"} {
		i18n.Init(locale)
		logs := testStore(t)
		writeLog(t, newLogWriter(logs), "中文🧑‍💻 Unicode long line........................................................\n")
		for _, native := range []bool{false, true} {
			m := newModel(logs, Graph{})
			m.freeze()
			m.copy.native = native
			for _, size := range [][2]int{{1, 1}, {1, 12}, {2, 2}, {2, 10}, {38, 8}, {80, 24}, {140, 30}} {
				updated, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				m = updated.(model)
				lines := strings.Split(m.View().Content, "\n")
				if len(lines) > size[1] {
					t.Fatal("copy layout height overflow", size)
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("copy layout width overflow", size, line)
					}
				}
			}
		}
	}
}

func TestDragSelectsLastDisplayedFrameDespiteFastNewLogs(t *testing.T) {
	logs := testStore(t)
	writer := newLogWriter(logs)
	writeLog(t, writer, "VISIBLE_BEFORE 中文\n")
	m := newModel(logs, Graph{})
	view := m.View()
	if !strings.Contains(view.Content, "VISIBLE_BEFORE") {
		t.Fatal("old frame not rendered")
	}
	for range 1000 {
		writeLog(t, writer, "NEW_UNDISPLAYED\n")
	}
	left, _, _ := m.layout()
	y := len(m.headerLines())
	m.startSelection(left+2, y)
	m.copy.head = m.mouseCursor(left+16, y)
	if text := m.selectedText(); text != "VISIBLE_BEFORE " {
		t.Fatalf("selected new unseen logs: %q", text)
	}
	if page := plainPage(m); strings.Contains(page, "NEW_UNDISPLAYED") {
		t.Fatal("frozen view changed at click")
	}
	m.resumeCopy(true)
	if page := plainPage(m); !strings.Contains(page, "NEW_UNDISPLAYED") {
		t.Fatal("follow did not restore live output")
	}
}
