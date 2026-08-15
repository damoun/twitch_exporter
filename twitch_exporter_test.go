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

func TestFilterOut(t *testing.T) {
	cases := []struct {
		name   string
		list   []string
		remove []string
		want   []string
	}{
		{"nothing removed", []string{"a", "b", "c"}, nil, []string{"a", "b", "c"}},
		{"removes one", []string{"a", "b", "c"}, []string{"b"}, []string{"a", "c"}},
		{"removes several", []string{"a", "b", "c"}, []string{"a", "c"}, []string{"b"}},
		{"removes all", []string{"a", "b"}, []string{"a", "b"}, []string{}},
		{"ignores unknown removals", []string{"a", "b"}, []string{"z"}, []string{"a", "b"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterOut(tc.list, tc.remove)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("filterOut(%v, %v) = %v, want %v", tc.list, tc.remove, got, tc.want)
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
		{"everything excluded", "channels=twitch&collector=channel_up&exclude=channel_up"},
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
