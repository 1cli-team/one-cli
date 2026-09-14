package miserelease

import (
	"fmt"
)

// Pin the release and both the archive and extracted binary digests. These
// values come from the official v2026.9.7 SHASUMS256.txt; no runtime "latest"
// resolution or unverified installer script is involved.
const Version = "2026.9.7"
const BaseURL = "https://github.com/jdx/mise/releases/download/v" + Version

type Asset struct {
	Platform      string
	Format        string
	ArchiveSHA256 string
	BinarySHA256  string
}

func ForPlatform(goos, goarch string) (Asset, error) {
	var a Asset
	switch goos + "/" + goarch {
	case "linux/amd64":
		a = Asset{"linux-x64-musl", "tar.gz", "2b95652a7e946be3fc72b729d3e415813eba74c49eb3a13b96d16b213aaf6c60", "67265e2efa2874bb956e3348b8c2b3eabf9654d80770f92b9b334d059f583962"}
	case "linux/arm64":
		a = Asset{"linux-arm64-musl", "tar.gz", "85cce4289be0f931609ba2ac179b84875198e948ccb8557a8e842aa15ee117b6", "0e99838cae628b428f84236db29dc6a80edfd90e2dedf81a84d367214300f885"}
	case "darwin/amd64":
		a = Asset{"macos-x64", "tar.gz", "98d6fa19fbf6022558ffbbf259800fc7f3dd696fdc950b922d7f3f53e75ef36b", "06f763e37615966f0d660f0fd7d4d23568d1b63d5bdb65e2f7a98af2fdc36075"}
	case "darwin/arm64":
		a = Asset{"macos-arm64", "tar.gz", "f6810aa1609a475ce7f3fdc83eb1e090ace6231d3b1b5aaead944663c4b4b7f1", "3c3f377e7123a466274a20f01502ddd8c58f76028907f471c9bc42fbf83846e1"}
	case "windows/amd64":
		a = Asset{"windows-x64", "zip", "caa1ca158f04d91f42dc2cd99bb1f69f6b5bfa0d0772d485150120aad9778685", "80552c6a4a03849707cb6154186a22795516388d06bf0760b4418729a42efe43"}
	default:
		return a, fmt.Errorf("bundled mise is unsupported on %s/%s; set ONE_MISE_BINARY to a compatible executable", goos, goarch)
	}
	return a, nil
}

func (a Asset) Filename() string {
	return "mise-v" + Version + "-" + a.Platform + "." + a.Format
}

func (a Asset) BinaryName() string {
	if a.Format == "zip" {
		return "mise.exe"
	}
	return "mise"
}
