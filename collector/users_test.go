package collector

import (
	"reflect"
	"testing"
	"time"

	"github.com/nicklaw5/helix/v2"
)

func TestChunkStrings(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		size int
		want [][]string
	}{
		{"empty", nil, 100, nil},
		{"under limit", []string{"a", "b"}, 100, [][]string{{"a", "b"}}},
		{"exact limit", []string{"a", "b"}, 2, [][]string{{"a", "b"}}},
		{"over limit", []string{"a", "b", "c"}, 2, [][]string{{"a", "b"}, {"c"}}},
		{"size one", []string{"a", "b"}, 1, [][]string{{"a"}, {"b"}}},
		{"zero size", []string{"a", "b"}, 0, [][]string{{"a", "b"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := chunkStrings(tc.in, tc.size)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("chunkStrings(%v, %d) = %v, want %v", tc.in, tc.size, got, tc.want)
			}
		})
	}
}

func TestChunkStringsRespectsHelixLimit(t *testing.T) {
	in := make([]string, 250)
	for i := range in {
		in[i] = "channel"
	}
	chunks := chunkStrings(in, maxHelixIDsPerRequest)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for 250 items, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > maxHelixIDsPerRequest {
			t.Errorf("chunk %d has %d items, exceeds limit %d", i, len(c), maxHelixIDsPerRequest)
		}
	}
}

func TestSingleflightKeyIsOrderIndependent(t *testing.T) {
	a := singleflightKey([]string{"Twitch", "shroud", "ninja"})
	b := singleflightKey([]string{"ninja", "TWITCH", "shroud"})
	if a != b {
		t.Errorf("expected identical keys regardless of order/case, got %q and %q", a, b)
	}
}

func TestGetUsersServesFromCacheWithoutAPI(t *testing.T) {
	// Pre-populate the cache, then resolve with a nil client: if every login is
	// a cache hit, getUsers must never touch the (nil) API client.
	storeUsers([]helix.User{
		{ID: "1", Login: "alpha", DisplayName: "Alpha"},
		{ID: "2", Login: "beta", DisplayName: "Beta"},
	})

	got, err := getUsers(nil, testLogger(), []string{"Alpha", "beta"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 users, got %d", len(got))
	}
	// Order must follow the input, and lookups are case-insensitive.
	if got[0].Login != "alpha" || got[1].Login != "beta" {
		t.Errorf("unexpected order/result: %+v", got)
	}
}

func TestGetUsersDeduplicatesLogins(t *testing.T) {
	storeUsers([]helix.User{{ID: "9", Login: "gamma", DisplayName: "Gamma"}})

	got, err := getUsers(nil, testLogger(), []string{"gamma", "Gamma", "gamma"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 de-duplicated user, got %d", len(got))
	}
}

func TestCachedUserExpires(t *testing.T) {
	userCacheMu.Lock()
	userCacheStore["expired"] = userCacheEntry{
		user:      helix.User{Login: "expired"},
		expiresAt: time.Now().Add(-time.Minute),
	}
	userCacheMu.Unlock()

	if _, ok := cachedUser("expired"); ok {
		t.Error("expected expired cache entry to be treated as a miss")
	}
}
