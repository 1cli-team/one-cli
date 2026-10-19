module github.com/torchstellar-team/one-cli/packages/cli

go 1.26.0

require (
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/torchstellar-team/one-cli/packages/one-client v0.0.0
	github.com/torchstellar-team/one-cli/packages/one-core v0.0.0
)

require (
	charm.land/bubbles/v2 v2.2.1 // indirect
	charm.land/bubbletea/v2 v2.0.10 // indirect
	charm.land/huh/v2 v2.0.3 // indirect
	charm.land/lipgloss/v2 v2.0.6 // indirect
	github.com/charmbracelet/x/ansi v0.11.8 // indirect
	github.com/gofrs/flock v0.13.1 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/zalando/go-keyring v0.2.8 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/term v0.46.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/catppuccin/go v0.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260922123528-4e49372c11f9 // indirect
	github.com/charmbracelet/x/exp/ordered v0.1.0 // indirect
	github.com/charmbracelet/x/exp/strings v0.1.0 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.30 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/torchstellar-team/one-cli/packages/one-core => ../one-core

replace github.com/torchstellar-team/one-cli/packages/one-client => ../one-client
