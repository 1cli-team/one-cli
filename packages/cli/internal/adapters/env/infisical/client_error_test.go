package infisical

import (
	"errors"
	"fmt"
	"testing"

	sdk "github.com/infisical/go-sdk"
	sdkerrors "github.com/infisical/go-sdk/packages/errors"
)

func TestAPIErrorStatusDoesNotReadPortOrRequestID(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"unauthorized", &sdk.APIError{StatusCode: 401, URL: "http://localhost:50001"}, "INFISICAL_AUTH_FAILED"},
		{"wrapped unauthorized", fmt.Errorf("list: %w", &sdk.APIError{StatusCode: 401}), "INFISICAL_AUTH_FAILED"},
		{"server error with auth-looking request ID", &sdk.APIError{StatusCode: 500, URL: "http://localhost:40123", ReqId: "unauthorized-401"}, "INFISICAL_API_ERROR"},
		{"closed connection on auth-looking port", sdkerrors.NewRequestError("ListSecrets", errors.New("Get http://127.0.0.1:40123/api/v3/secrets/raw: EOF")), "INFISICAL_API_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var coded interface{ ErrorCode() string }
			err := mapAPIError(tc.err)
			if !errors.As(err, &coded) || coded.ErrorCode() != tc.code {
				t.Fatalf("code=%v want=%s", err, tc.code)
			}
		})
	}
}
