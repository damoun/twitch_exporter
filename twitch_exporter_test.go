package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestParseListParams(t *testing.T) {
	cases := []struct {
		name  string
		query string
		keys  []string
		want  []string
	}{
		{"repeated", "channels=a&channels=b", []string{"channels"}, []string{"a", "b"}},
		{"comma separated", "channels=a,b,c", []string{"channels"}, []string{"a", "b", "c"}},
		{"mixed", "channels=a,b&channels=c", []string{"channels"}, []string{"a", "b", "c"}},
		{"trims and drops empties", "channels=a,, b ,", []string{"channels"}, []string{"a", "b"}},
		{"multiple keys", "collector=x&collectors=y", []string{"collector", "collectors"}, []string{"x", "y"}},
		{"absent", "foo=bar", []string{"channels"}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := url.ParseQuery(tc.query)
			if err != nil {
				t.Fatalf("bad query: %v", err)
			}
			got := parseListParams(q, tc.keys...)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseListParams(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func TestProbeHandlerBadRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := probeHandler(logger, nil)

	cases := []struct {
		name  string
		query string
	}{
		{"missing channels", "collector=channel_up"},
		{"unknown collector", "channels=twitch&collector=nope"},
		{"privileged collector", "channels=twitch&collector=channel_subscribers_total"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/probe?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d (body: %q)", http.StatusBadRequest, rec.Code, rec.Body.String())
			}
		})
	}
}
