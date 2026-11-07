package network

import (
	"strings"
	"testing"
)

func TestHTTPErrorRemoteMessage(t *testing.T) {
	tests := []struct {
		name        string
		err         *HTTPError
		wantContain string
		wantOmit    string
	}{
		{
			name: "google api error object",
			err: &HTTPError{
				StatusCode: 403,
				Status:     "403 Forbidden",
				Body:       []byte(`{"error":{"code":403,"message":"Missing or insufficient permissions.","status":"PERMISSION_DENIED"}}`),
			},
			wantContain: "Missing or insufficient permissions.",
		},
		{
			name: "oauth error string",
			err: &HTTPError{
				StatusCode: 400,
				Status:     "400 Bad Request",
				Body:       []byte(`{"error":"invalid_grant","error_description":"Invalid JWT signature."}`),
			},
			wantContain: "invalid_grant: Invalid JWT signature.",
		},
		{
			name: "non json body is not echoed",
			err: &HTTPError{
				StatusCode: 500,
				Status:     "500 Internal Server Error",
				Body:       []byte("secret raw body"),
			},
			wantContain: "status 500",
			wantOmit:    "secret raw body",
		},
		{
			name: "status only",
			err: &HTTPError{
				StatusCode: 404,
			},
			wantContain: "status 404",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			if !strings.Contains(msg, tc.wantContain) {
				t.Fatalf("expected %q in %q", tc.wantContain, msg)
			}
			if tc.wantOmit != "" && strings.Contains(msg, tc.wantOmit) {
				t.Fatalf("did not expect %q in %q", tc.wantOmit, msg)
			}
		})
	}
}
