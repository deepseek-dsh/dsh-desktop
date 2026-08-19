package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPluginChangeNeedsRestart(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		installed   string
		wantRestart bool
		wantBusy    bool
	}{
		{
			name:        "restart activation",
			status:      `{"installing":false}`,
			installed:   `{"activation":{"plugin":{"state":"restart"}}}`,
			wantRestart: true,
		},
		{
			name:      "live activation",
			status:    `{"installing":false}`,
			installed: `{"activation":{"plugin":{"state":"live"}}}`,
		},
		{
			name:        "client plugin pending next boot",
			status:      `{"installing":false}`,
			installed:   `{"activation":{"plugin":{"state":"inert"}}}`,
			wantRestart: true,
		},
		{
			name:     "installation busy",
			status:   `{"installing":true}`,
			wantBusy: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/dsh-market/status":
					_, _ = w.Write([]byte(test.status))
				case "/dsh-market/installed":
					_, _ = w.Write([]byte(test.installed))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			a := &App{}
			gotRestart, gotBusy := a.pluginChangeNeedsRestart(server.URL)
			if gotRestart != test.wantRestart || gotBusy != test.wantBusy {
				t.Fatalf("得到 (restart=%v,busy=%v), 期望 (restart=%v,busy=%v)",
					gotRestart, gotBusy, test.wantRestart, test.wantBusy)
			}
		})
	}
}
