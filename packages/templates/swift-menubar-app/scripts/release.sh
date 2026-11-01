#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Import the certificate into your keychain and store a notarytool profile first.
# 请先将证书导入钥匙串，并配置 notarytool 凭据。
: "${SIGNING_IDENTITY:?Set SIGNING_IDENTITY / 请设置 SIGNING_IDENTITY}"
: "${NOTARY_PROFILE:?Set NOTARY_PROFILE / 请设置 NOTARY_PROFILE}"
case "$SIGNING_IDENTITY" in
  "Developer ID Application:"*) ;;
  *) echo "Use a Developer ID Application identity / 请使用 Developer ID Application 证书" >&2; exit 1 ;;
esac

notarize() {
  local artifact="$1" receipt="$2"
  xcrun notarytool submit "$artifact" --keychain-profile "$NOTARY_PROFILE" \
    --wait --timeout 30m --output-format json > "$receipt"
  if [[ "$(plutil -extract status raw -o - "$receipt")" != Accepted ]]; then
    echo "Notarization was not accepted; see $receipt / 公证未通过，请查看 $receipt" >&2
    return 1
  fi
}

bash scripts/build.sh release "$SIGNING_IDENTITY"
app="build/one-template-macos.app"
dmg="build/one-template-macos.dmg"
codesign --verify --deep --strict --verbose=2 "$app"
ditto -c -k --keepParent "$app" build/notarization.zip
notarize build/notarization.zip build/app-notarization.json
xcrun stapler staple "$app"
xcrun stapler validate "$app"
spctl --assess --type execute --verbose=2 "$app"

bash scripts/pack.sh
codesign --force --timestamp --sign "$SIGNING_IDENTITY" "$dmg"
notarize "$dmg" build/dmg-notarization.json
xcrun stapler staple "$dmg"
xcrun stapler validate "$dmg"
codesign --verify --strict "$dmg"
spctl --assess --type open --context context:primary-signature --verbose=2 "$dmg"
echo "$dmg"
