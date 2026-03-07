# Twitch Exporter

Export [Twitch](https://dev.twitch.tv/docs/api/reference) metrics to [Prometheus](https://github.com/prometheus/prometheus).

## Collectors

Each collector can be toggled with `--[no-]collector.<name>` flags.

| Collector | Default | Auth | Metrics |
|---|---|---|---|
| `channel_up` | enabled | app | `twitch_channel_up` (username, game) |
| `channel_viewers_total` | enabled | app | `twitch_channel_viewers_total` (username, game) |
| `channel_followers_total` | enabled | app | `twitch_channel_followers_total` (username) |
| `channel_clips_total` | enabled | app | `twitch_channel_clips_total` (username) |
| `channel_info` | enabled | app | `twitch_channel_info` (username, game, title, language), `twitch_channel_delay_seconds` (username) |
| `channel_emotes_total` | enabled | app | `twitch_channel_emotes_total` (username) |
| `channel_chat_settings` | enabled | app | `twitch_channel_chat_emote_only`, `_followers_only`, `_subscriber_only`, `_slow_mode`, `_slow_mode_wait_seconds` (username) |
| `channel_subscribers_total` | disabled | user | `twitch_channel_subscribers_total` (username, tier, gifted) |
| `channel_bits_leaderboard` | disabled | user | `twitch_channel_bits_leaderboard` (username, user_name, user_id, rank) |
| `channel_chatters_total` | disabled | user | `twitch_channel_chatters_total` (username) |
| `channel_goals` | disabled | user | `twitch_channel_goal_current`, `_goal_target` (username, type) |
| `channel_vips_total` | disabled | user | `twitch_channel_vips_total` (username) |
| `channel_banned_users_total` | disabled | user | `twitch_channel_banned_users_total` (username) |
| `channel_charity` | disabled | user | `twitch_channel_charity_current_amount`, `_charity_target_amount` (username, currency) |
| `channel_moderators_total` | disabled | user | `twitch_channel_moderators_total` (username) |
| `channel_chat_messages_total` | disabled | user + EventSub | `twitch_channel_chat_messages_total` (username, chatter_username) |

## Flags

```bash
./twitch_exporter --help
```

* __`twitch.channel`:__ Name of a Twitch channel to request metrics.
* __`twitch.client-id`:__ Client ID for the Twitch Helix API.
* __`twitch.client-secret`:__ Client Secret for the Twitch Helix API.
* __`twitch.access-token`:__ Access Token for the Twitch Helix API.
* __`twitch.access-token-file`:__ File containing the Access Token (alternative to `twitch.access-token`).
* __`twitch.refresh-token`:__ Refresh Token for the Twitch Helix API.
* __`twitch.refresh-token-file`:__ File containing the Refresh Token (alternative to `twitch.refresh-token`).
* __`log.format`:__ Output format of log messages. One of: `logfmt`, `json`.
* __`log.level`:__ Logging level. One of: `debug`, `info`, `warn`, `error`. Default: `info`.
* __`version`:__ Show application version.
* __`web.listen-address`:__ Addresses on which to expose metrics and web interface. Repeatable for multiple addresses.
* __`web.telemetry-path`:__ Path under which to expose metrics.
* __`web.config.file`:__ Path to configuration file that can enable TLS or authentication.
* __`eventsub.enabled`:__ Enable eventsub endpoint (default: false).
* __`eventsub.webhook-url`:__ The url your collector will be expected to be hosted at, eg: http://example.svc/eventsub (Must end with `/eventsub`).
* __`eventsub.webhook-secret`:__ Secure 1-100 character secret for your eventsub validation.

## Getting an Access Token

Some metrics require a user access token with specific scopes. You can use the [Token Helper](https://damoun.github.io/twitch_exporter/) to generate one through the OAuth flow, or use `twitch-cli` as described in the [Development & Testing](#development--testing) section.

## EventSub

EventSub metrics are disabled by default because they require a publicly accessible endpoint and additional permissions.

If you wish to use eventsub based metrics then you should deploy an instance of the exporter just for the user that needs
the eventsub metrics, such as your own channel, and just collect the privileged metrics using that exporter.

### Setting up EventSub metrics

You can read more about the process [here](https://dev.twitch.tv/docs/chat/authenticating/)

1. Install the twitch-cli
1. Ensure your twitch app has localhost:3000 added as a redirect uri
1. `twitch token -u -s 'channel:read:subscriptions bits:read moderator:read:chatters moderation:read channel:read:goals channel:read:charity channel:bot user:read:chat user:bot'`
1. Start the collector with `client-id`, `client-secret`, `access-token`, and `refresh-token` defined

```
./twitch_exporter \
  --twitch.client-id xxx \
  --twitch.client-secret xxx \
  --twitch.access-token xxx \
  --twitch.refresh-token xxx \
  --twitch.channel surdaft \
  --eventsub.enabled \
  --eventsub.webhook-url 'https://xxx/eventsub' \
  --eventsub.webhook-secret xxxx \
  --collector.channel_chat_messages_total \
  --collector.channel_subscribers_total \
  --no-collector.channel_followers_total \
  --no-collector.channel_up \
  --no-collector.channel_viewers_total
```

## Development & Testing

### Prerequisites

- Go 1.25+
- A [Twitch Developer](https://dev.twitch.tv/) account
- [twitch-cli](https://dev.twitch.tv/docs/cli/) installed

### Create a Twitch Application

1. Go to https://dev.twitch.tv/console/apps
2. Register a new app, set OAuth redirect to `http://localhost:3000`
3. Note the Client ID and Client Secret

### Build

```bash
make build
```

### Run with app token (basic collectors)

```bash
./twitch_exporter \
  --twitch.client-id <client-id> \
  --twitch.client-secret <client-secret> \
  --twitch.channel <channel>
```

### Configure twitch-cli

Required before generating user tokens:

```bash
twitch configure
```

Enter Client ID and Client Secret when prompted.

### Get a user token (for privileged collectors)

```bash
twitch token -u -s 'channel:read:subscriptions bits:read moderator:read:chatters moderation:read channel:read:goals channel:read:charity channel:bot user:read:chat user:bot'
```

### Run with user token

```bash
./twitch_exporter \
  --twitch.client-id <client-id> \
  --twitch.client-secret <client-secret> \
  --twitch.access-token <access-token> \
  --twitch.refresh-token <refresh-token> \
  --twitch.channel <channel> \
  --collector.channel_subscribers_total
```

### Verify metrics

```bash
curl http://localhost:9184/metrics
```

### Run tests

```bash
make test
```

### Testing with twitch-cli mock API

Start mock server:

```bash
twitch mock-api start
```

Useful for development without hitting Twitch rate limits.

## Using Docker

You can deploy this exporter using the `ghcr.io/damoun/twitch-exporter` Docker image.

For example:

```bash
docker pull ghcr.io/damoun/twitch-exporter

docker run -d -p 9184:9184 \
        ghcr.io/damoun/twitch-exporter \
        --twitch.client-id <client-id> \
        --twitch.client-secret <client-secret> \
        --twitch.channel dam0un
```

## Using Helm

[Helm](https://helm.sh) must be installed to use the charts.  Please refer to
Helm's [documentation](https://helm.sh/docs) to get started.

Once Helm has been set up correctly, add the repo as follows:

    helm repo add twitch-exporter https://damoun.github.io/twitch-exporter

If you had already added this repo earlier, run `helm repo update` to retrieve
the latest versions of the packages.  You can then run `helm search repo
twitch-exporter` to see the charts.

To install the twitch-exporter chart:

    helm install my-twitch-exporter twitch-exporter/twitch-exporter

To uninstall the chart:

    helm delete my-twitch-exporter
