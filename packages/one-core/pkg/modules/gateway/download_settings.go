package gateway

import (
	"context"
	"net/http"
)

type DownloadSettings struct {
	ChinaMirror bool `json:"chinaMirror"`
}

type DownloadSettingsInput struct {
	ChinaMirror *bool `json:"chinaMirror"`
}

func (c *Client) DownloadSettings(ctx context.Context) (DownloadSettings, error) {
	var result DownloadSettings
	err := c.do(ctx, http.MethodGet, &result, "settings", "downloads")
	return result, err
}

func (c *Client) SetDownloadSettings(ctx context.Context, input DownloadSettingsInput) (DownloadSettings, error) {
	var result DownloadSettings
	err := c.request(ctx, http.MethodPut, input, &result, "settings", "downloads")
	return result, err
}
