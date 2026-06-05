package mobile

import "testing"

func TestIsSyncTransient(t *testing.T) {
	cases := []struct {
		name      string
		errMsg    string
		wantRetry bool
	}{
		// 4xx — permanent, do NOT retry
		{"401 unauthorized", "/sync/v1/wordbank/pull: HTTP 401: Unauthorized", false},
		{"403 forbidden", "/sync/v1/history/pull: HTTP 403: Forbidden", false},
		{"404 not found", "/sync/v1/wordbank/push: HTTP 404: Not Found", false},
		{"400 bad request", "/sync/v1/wordbank/pull: HTTP 400: bad since timestamp", false},

		// 5xx — transient, retry
		{"500 server error", "/sync/v1/wordbank/push: HTTP 500: internal server error", true},
		{"503 unavailable", "/sync/v1/history/pull: HTTP 503: Service Unavailable", true},

		// Network-level errors — no HTTP code, transient
		{"connection refused", "Post \"http://...\": dial tcp: connect: connection refused", true},
		{"timeout", "context deadline exceeded", true},
		{"empty", "", true},

		// Unknown / unrecognised format — default to transient
		{"no code parseable", "some unexpected error: HTTP xyz: broken", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSyncTransient(tc.errMsg)
			if got != tc.wantRetry {
				t.Errorf("IsSyncTransient(%q) = %v, want %v", tc.errMsg, got, tc.wantRetry)
			}
		})
	}
}
