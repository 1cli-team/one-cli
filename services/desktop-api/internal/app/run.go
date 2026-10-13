// Package desktop owns the private native-host entrypoint, before ordinary CLI parsing.
package app

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	serve "github.com/torchstellar-team/one-cli/services/desktop-api/internal/http"
)

func Run(identity desktopcontrol.Identity, compose func() serve.MuxOpts) error {
	_ = i18n.Init(i18n.Resolve("auto"))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	frames := make(chan []byte)
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 4096), 16<<10)
		for scanner.Scan() {
			frame := append([]byte(nil), scanner.Bytes()...)
			select {
			case frames <- frame:
			case <-ctx.Done():
				return
			}
		}
	}()
	var boot desktopcontrol.Bootstrap
	select {
	case frame, ok := <-frames:
		if !ok || json.Unmarshal(frame, &boot) != nil {
			return i18n.Errorf("desktop.handshake_failed")
		}
	case <-time.After(10 * time.Second):
		return i18n.Errorf("desktop.handshake_timeout")
	case <-ctx.Done():
		return ctx.Err()
	}
	token, err := hex.DecodeString(boot.Token)
	if err != nil || len(token) != 32 || len(boot.Generation) < 16 || len(boot.Generation) > 64 || boot.Protocol != desktopcontrol.Protocol || boot.Type != "bootstrap" {
		return i18n.Errorf("desktop.handshake_failed")
	}
	origin := "tauri://localhost"
	if runtime.GOOS == "windows" {
		origin = "http://tauri.localhost"
	}
	if boot.Origin != origin && !(identity.Channel != "release" && boot.Origin == "http://127.0.0.1:5173") {
		return i18n.Errorf("desktop.origin_rejected")
	}
	session := &desktopcontrol.Session{Bootstrap: boot, Identity: identity}
	var outputMu sync.Mutex
	send := func(v any) { outputMu.Lock(); defer outputMu.Unlock(); _ = json.NewEncoder(os.Stdout).Encode(v) }
	go func() {
		defer cancel() // EOF is host death, never keep a detached backend alive.
		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-frames:
				if !ok {
					return
				}
				var command struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(frame, &command) != nil {
					return
				}
				switch command.Type {
				case "shutdown":
					session.Quiesce()
					return
				case "quiesce":
					send(map[string]any{"type": "quiesced", "active": session.Quiesce()})
				case "resume":
					session.Resume()
				default:
					return
				}
			}
		}
	}()
	paths, err := bootstrap.InitializeStorage()
	if err != nil {
		send(map[string]string{"type": "failed", "error": err.Error()})
		return err
	}
	prefs, err := preferences.Load()
	if err != nil {
		return err
	}
	if err = i18n.Init(i18n.Resolve(prefs.Locale)); err != nil {
		return err
	}
	opts := compose()
	opts.Session = session
	return serve.Run(ctx, opts, func(port int) {
		send(struct {
			Type        string `json:"type"`
			Generation  string `json:"generation"`
			Port        int    `json:"port"`
			StorageRoot string `json:"storageRoot"`
			desktopcontrol.Identity
		}{"ready", boot.Generation, port, paths.Root, identity})
	})
}
