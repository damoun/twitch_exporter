package collector

import (
	"io"
	"log/slog"
	"slices"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestProbeableCollectors(t *testing.T) {
	got := ProbeableCollectors()

	// Every returned collector must be registered as app-token only.
	for _, name := range got {
		if collectorAuthModes[name] != AuthApp {
			t.Errorf("ProbeableCollectors returned %q which is not AuthApp", name)
		}
	}

	// No user-token collector should ever leak into the probe set.
	for name, mode := range collectorAuthModes {
		if mode == AuthUser && slices.Contains(got, name) {
			t.Errorf("user-token collector %q must not be probeable", name)
		}
	}

	// Result must be sorted for stable output.
	if !slices.IsSorted(got) {
		t.Errorf("ProbeableCollectors result is not sorted: %v", got)
	}

	// Sanity check a couple of known app-token collectors are present.
	for _, want := range []string{"channel_up", "channel_viewers_total"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q in probeable collectors, got %v", want, got)
		}
	}
}

func TestNewProbeExporter(t *testing.T) {
	logger := testLogger()
	channels := ChannelNames{"twitch"}

	t.Run("app-token collector succeeds", func(t *testing.T) {
		exp, err := NewProbeExporter(logger, nil, channels, []string{"channel_up"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(exp.Collectors) != 1 {
			t.Fatalf("expected 1 collector, got %d", len(exp.Collectors))
		}
		if _, ok := exp.Collectors["channel_up"]; !ok {
			t.Errorf("channel_up collector not present")
		}
	})

	t.Run("user-token collector is rejected", func(t *testing.T) {
		_, err := NewProbeExporter(logger, nil, channels, []string{"channel_subscribers_total"})
		if err == nil {
			t.Fatal("expected error for user-token collector, got nil")
		}
	})

	t.Run("unknown collector is rejected", func(t *testing.T) {
		_, err := NewProbeExporter(logger, nil, channels, []string{"does_not_exist"})
		if err == nil {
			t.Fatal("expected error for unknown collector, got nil")
		}
	})

	t.Run("empty request is rejected", func(t *testing.T) {
		_, err := NewProbeExporter(logger, nil, channels, nil)
		if err == nil {
			t.Fatal("expected error for empty request, got nil")
		}
	})

	t.Run("duplicates are de-duplicated", func(t *testing.T) {
		exp, err := NewProbeExporter(logger, nil, channels, []string{"channel_up", "channel_up"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(exp.Collectors) != 1 {
			t.Fatalf("expected 1 collector after de-dup, got %d", len(exp.Collectors))
		}
	})

	t.Run("one bad collector fails the whole probe", func(t *testing.T) {
		_, err := NewProbeExporter(logger, nil, channels, []string{"channel_up", "channel_goals"})
		if err == nil {
			t.Fatal("expected error when a user-token collector is mixed in, got nil")
		}
	})
}
