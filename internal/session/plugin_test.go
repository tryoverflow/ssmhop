package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestEndpoint(t *testing.T) {
	tests := []struct{ region, want string }{
		{"eu-west-1", "https://ssm.eu-west-1.amazonaws.com"},
		{"us-east-1", "https://ssm.us-east-1.amazonaws.com"},
		{"ap-southeast-2", "https://ssm.ap-southeast-2.amazonaws.com"},

		{"cn-north-1", "https://ssm.cn-north-1.amazonaws.com.cn"},
		{"cn-northwest-1", "https://ssm.cn-northwest-1.amazonaws.com.cn"},
	}
	for _, tc := range tests {
		if got := Endpoint(tc.region); got != tc.want {
			t.Errorf("Endpoint(%q) = %q, want %q", tc.region, got, tc.want)
		}
	}
}

func TestSessionJSONKeys(t *testing.T) {
	b, err := json.Marshal(Session{SessionID: "s-1", StreamURL: "wss://example", TokenValue: "tok"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"SessionId", "StreamUrl", "TokenValue"} {
		if _, ok := got[key]; !ok {
			t.Errorf("session JSON is missing key %q: %s", key, b)
		}
	}
}

func TestRunRejectsIncompleteSession(t *testing.T) {
	for _, s := range []Session{
		{SessionID: "s-1"},
		{SessionID: "s-1", StreamURL: "wss://example"},
		{SessionID: "s-1", TokenValue: "tok"},
	} {
		err := Run(context.Background(), s, "eu-west-1", "", struct{}{})
		if err == nil {
			t.Fatalf("Run(%+v) = nil, want an error", s)
		}
		if !strings.Contains(err.Error(), "StreamUrl") {
			t.Errorf("Run(%+v) error = %v, want it to name the missing field", s, err)
		}
	}
}
