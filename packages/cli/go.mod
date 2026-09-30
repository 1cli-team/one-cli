module github.com/torchstellar-team/one-cli/packages/cli

go 1.26.0

require (
	charm.land/bubbles/v2 v2.2.1
	charm.land/bubbletea/v2 v2.0.10
	charm.land/huh/v2 v2.0.3
	charm.land/lipgloss/v2 v2.0.6
	github.com/charmbracelet/x/ansi v0.11.8
	github.com/creack/pty v1.1.24
	github.com/go-resty/resty/v2 v2.17.2
	github.com/gofrs/flock v0.13.1
	github.com/infisical/go-sdk v0.8.0
	github.com/pelletier/go-toml/v2 v2.4.3
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/torchstellar-team/one-cli/packages/kernel v0.0.0
	github.com/zalando/go-keyring v0.2.8
	golang.org/x/mod v0.41.0
	golang.org/x/sys v0.48.0
	golang.org/x/term v0.46.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/torchstellar-team/one-cli/packages/kernel => ../kernel

require (
	cloud.google.com/go/auth v0.24.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.3.0 // indirect
	cloud.google.com/go/compute/metadata v0.10.0 // indirect
	cloud.google.com/go/iam v1.14.0 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aws/aws-sdk-go-v2 v1.47.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.33.6 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.6 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.1 // indirect
	github.com/aws/smithy-go v1.28.2 // indirect
	github.com/catppuccin/go v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260922123528-4e49372c11f9
	github.com/charmbracelet/x/exp/ordered v0.1.0 // indirect
	github.com/charmbracelet/x/exp/strings v0.1.0 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/google/s2a-go v0.1.10 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.22 // indirect
	github.com/googleapis/gax-go/v2 v2.26.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-runewidth v0.0.30 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/oracle/oci-go-sdk/v65 v65.126.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/sony/gobreaker/v2 v2.4.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.71.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	google.golang.org/api v0.299.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260921155816-b14227669459 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260921155816-b14227669459 // indirect
	google.golang.org/grpc v1.84.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
