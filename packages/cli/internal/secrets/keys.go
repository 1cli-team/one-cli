package secrets

import (
	"regexp"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// secretKeyRE is the POSIX env-var pattern: must start with a letter or
// underscore, then letters / digits / underscore. The whole point of the
// validator is to keep keys safe across both backends:
//
//   - dotenv writes to `.env` files; if the key fails this pattern,
//     `source .env` (bash/zsh) silently drops the line and downstream
//     consumers (overmind, docker-compose env_file, CI runners that
//     read dotenv) end up missing the value.
//   - Infisical accepts arbitrary keys server-side, but a key that
//     can't round-trip through env vars makes the whole "inject as
//     env" path break — and that's the only path `one run` uses today.
//
// Cross-backend portability also matters: a workspace that switches from
// dotenv to Infisical (or back) shouldn't silently drop secrets because
// of naming.
var secretKeyRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// AssertValidKey rejects secret names that don't match the POSIX env-var
// pattern. Returns ENV_INVALID_KEY on bad input; the message includes a
// concrete example so users can fix their input without consulting docs.
func AssertValidKey(s string) error {
	if !secretKeyRE.MatchString(s) {
		return cliErrors.New(cliErrors.ENV_INVALID_KEY,
			"密钥名称非法："+s+"（必须匹配 ^[A-Za-z_][A-Za-z0-9_]*$，例如 DATABASE_URL）。"+
				"如果配置框架（viper / NestJS @nestjs/config 等）需要嵌套路径如 database.url，"+
				"请在 secret 端用 DATABASE_URL，框架运行时会按 a.b.c ↔ A_B_C 自动覆盖。")
	}
	return nil
}
