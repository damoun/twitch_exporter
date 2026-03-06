# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Twitch Exporter is a Prometheus exporter for Twitch metrics, built following the Prometheus exporter conventions (similar to node_exporter). It exposes metrics about Twitch channels (up/down status, viewer count, followers, subscribers, chat messages, bits leaderboard, clips) on port 9184.

## Build & Test Commands

- **Build**: `make build` (uses [promu](https://github.com/prometheus/promu) under the hood)
- **Test**: `make test` (or `make test-short`)
- **Lint**: `make lint` (uses golangci-lint)
- **Format**: `make format`
- **Cross-build**: `make promu-build` (multi-arch via promu)
- **All checks**: `make` (runs style, lint, build, test)
- **Run single test**: `go test ./collector/ -run TestName`

## Architecture

### Collector Pattern

The project follows the Prometheus collector pattern from `prometheus/node_exporter`:

- **`collector/collector.go`**: Core framework. Defines the `Collector` interface (single `Update(ch)` method), the `Exporter` struct that implements `prometheus.Collector`, and a self-registration system via `registerCollector()`.
- **`collector/channel_*.go`**: Individual metric collectors, each registering themselves via `init()` with `registerCollector()`. Each can be toggled with `--collector.<name>` flags.
- **`internal/eventsub/`**: Twitch EventSub webhook client for real-time events (chat messages). Uses the `twitchwh` library.
- **`twitch_exporter.go`**: Entrypoint. Sets up Twitch API client (app or user token mode), optional EventSub client, and HTTP server.

### Adding a New Collector

1. Create `collector/channel_<metric>.go`
2. Implement the `Collector` interface (just `Update(ch chan<- prometheus.Metric) error`)
3. Call `registerCollector("name", defaultEnabled, NewYourCollector)` in `init()`
4. Factory signature: `func(logger, client, eventsubClient, channelNames) (Collector, error)`

### Auth Modes

- **App token** (default): `--twitch.client-id` + `--twitch.client-secret`
- **User token**: additionally provide `--twitch.access-token` + `--twitch.refresh-token` (needed for subscriber metrics)

Both modes auto-refresh tokens every 24 hours.

### Key Dependencies

- `nicklaw5/helix/v2`: Twitch Helix API client
- `prometheus/client_golang`: Prometheus metrics
- `prometheus/exporter-toolkit`: HTTP server with TLS support
- `alecthomas/kingpin/v2`: CLI flag parsing
- `LinneB/twitchwh`: Twitch EventSub webhook handling

### Existing Collectors

| Collector | File | Default | Auth |
|---|---|---|---|
| `channel_up` | `channel_up.go` | enabled | app token |
| `channel_viewers_total` | `channel_viewers_total.go` | enabled | app token |
| `channel_followers_total` | `channel_followers_total.go` | enabled | app token |
| `channel_subscribers_total` | `channel_subscribers_total.go` | disabled | user token |
| `channel_chat_messages_total` | `channel_chat_messages_total.go` | disabled | user token + EventSub |
| `channel_bits_leaderboard` | `channel_bits_leaderboard.go` | disabled | user token |
| `channel_clips_total` | `channel_clips_total.go` | enabled | app token |

### Deployment

- Docker image built via `Dockerfile` (uses prometheus busybox base)
- Helm chart in `charts/twitch-exporter/`
- Default listen address: `0.0.0.0:9184`
