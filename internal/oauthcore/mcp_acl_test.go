package oauthcore

import "testing"

func TestAnonymousMCPPayloadAllowed(t *testing.T) {
	policy := AnonymousMCPPolicy{PublicTools: []string{"list_pages", "get_page"}}

	tests := []struct {
		name          string
		body          string
		wantAllowed   bool
		wantToolsList bool
		wantReason    string
	}{
		{"initialize allowed", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, true, false, ""},
		{"tools list allowed", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, true, true, ""},
		{"public tool allowed", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_page"}}`, true, false, ""},
		{"private tool rejected", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"publish_post"}}`, false, false, "tool_not_public"},
		{"unknown method rejected", `{"jsonrpc":"2.0","id":1,"method":"resources/read"}`, false, false, "method_not_public"},
		{"batch detects tools list", `[{"jsonrpc":"2.0","id":1,"method":"initialize"},{"jsonrpc":"2.0","id":2,"method":"tools/list"}]`, true, true, ""},
		{"empty batch rejected", `[]`, false, false, "empty_batch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAllowed, gotToolsList, gotReason := policy.PayloadAllowed([]byte(tt.body))
			if gotAllowed != tt.wantAllowed || gotToolsList != tt.wantToolsList || gotReason != tt.wantReason {
				t.Fatalf("PayloadAllowed() = (%v, %v, %q), want (%v, %v, %q)",
					gotAllowed, gotToolsList, gotReason,
					tt.wantAllowed, tt.wantToolsList, tt.wantReason)
			}
		})
	}
}
