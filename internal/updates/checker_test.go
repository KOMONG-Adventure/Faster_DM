package updates

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		name, body, current, want string
		code                      int
	}{
		{"newer", `{"tag_name":"v0.2.0"}`, "0.1.0", "available", 200},
		{"numeric order", `{"tag_name":"v0.10.0"}`, "0.9.0", "available", 200},
		{"same", `{"tag_name":"v0.1.0"}`, "0.1.0", "current", 200},
		{"older", `{"tag_name":"v0.1.0"}`, "1.0.0", "current", 200},
		{"absent", `{}`, "0.1.0", "unavailable", 404},
		{"limited", `{}`, "0.1.0", "error", 403},
		{"invalid JSON", `<html>`, "0.1.0", "error", 200},
		{"invalid tag", `{"tag_name":"latest"}`, "0.1.0", "error", 200},
		{"prerelease", `{"tag_name":"v2.0.0","prerelease":true}`, "0.1.0", "unavailable", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") == "" {
					t.Error("user agent алга")
				}
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer s.Close()
			got := check(context.Background(), s.Client(), s.URL, tc.current)
			if got.Status != tc.want || got.CheckedAt == "" {
				t.Fatal(got)
			}
		})
	}
}
func TestNetworkFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := check(ctx, http.DefaultClient, LatestAPI, "0.1.0"); r.Status != "error" {
		t.Fatal(r)
	}
}
