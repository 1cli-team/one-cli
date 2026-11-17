package gateway

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

var ErrInvalidConnection = errors.New("invalid Gateway connection settings")

type Connection struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func ConnectionPath() (string, error) {
	dir, err := userdirs.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gateway.json"), nil
}

func ReadConnection() (Connection, error) {
	var connection Connection
	path, err := ConnectionPath()
	if err != nil {
		return connection, err
	}
	file, err := os.Open(path)
	if err != nil {
		return connection, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return connection, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<10 ||
		(runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return connection, errors.New("Gateway connection requires a private file")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&connection); err != nil {
		return Connection{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return Connection{}, errors.New("unexpected data after Gateway connection")
	}
	return connection, nil
}

func ResolveConnection(input Connection) (Connection, error) {
	input.URL, input.Token = strings.TrimSpace(input.URL), strings.TrimSpace(input.Token)
	if input.Token == "" {
		// Reuse the saved credential only for the exact same endpoint.
		if saved, err := ReadConnection(); err == nil && saved.URL == input.URL {
			input.Token = saved.Token
		}
	}
	if len(input.URL)+len(input.Token) > 8<<10 {
		return Connection{}, ErrInvalidConnection
	}
	if _, err := NewClient(input.URL, input.Token); err != nil {
		return Connection{}, ErrInvalidConnection
	}
	return input, nil
}

func SaveConnection(input Connection) (Connection, error) {
	connection, err := ResolveConnection(input)
	if err != nil || strings.TrimSpace(input.Token) == "" {
		// An unchanged connection needs no write, even if another process is rotating the token.
		return connection, err
	}
	path, err := ConnectionPath()
	if err != nil {
		return Connection{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Connection{}, err
	}
	payload, err := json.MarshalIndent(connection, "", "  ")
	if err != nil {
		return Connection{}, err
	}
	if err := fsutil.WriteAtomic(path, append(payload, '\n'), 0600); err != nil {
		return Connection{}, err
	}
	return connection, nil
}
