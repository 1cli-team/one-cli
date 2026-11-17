package terminalui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestNarrowOutputPreservesChineseAndLongValues(t *testing.T) {
	var buf bytes.Buffer
	ui := New(&buf)
	ui.width = 28
	path := "/workspace/项目/很长的项目目录/packages/example"
	ui.Fields(Field{Label: "环境变量来源", Value: path})
	ui.Error("无法创建工作区：目录已存在。\n请选择其他目录并重试。")
	for _, line := range strings.Split(buf.String(), "\n") {
		if width := ansi.StringWidth(line); width > ui.width {
			t.Errorf("line is %d columns, want at most %d: %q", width, ui.width, line)
		}
	}
	compact := strings.Join(strings.Fields(buf.String()), "")
	if !strings.Contains(compact, path) || !strings.Contains(compact, "请选择其他目录并重试。") {
		t.Fatalf("wrapped output lost content: %q", buf.String())
	}
}

func TestRedirectedOutputHasNoEscapesAndKeepsCommandsCopyable(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	var buf bytes.Buffer
	ui := New(&buf)
	ui.width = 24
	command := "one run build -p very-long-project-name"
	ui.Header("one add")
	ui.Success("Project added: example")
	ui.Warning("Check the project settings")
	ui.Command(command)
	if strings.Contains(buf.String(), "\x1b") {
		t.Fatalf("redirected output contains terminal escapes: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "    "+command+"\n") {
		t.Fatalf("command was wrapped or changed: %q", buf.String())
	}
}
