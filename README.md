# Twitch Exporter

[![CircleCI](https://circleci.com/gh/damoun/twitch_exporter/tree/master.svg?style=shield)][circleci]
[![Docker Pulls](https://img.shields.io/docker/pulls/damoun/twitch-exporter.svg?maxAge=604800)][hub]
[![Go Report Card](https://goreportcard.com/badge/github.com/damoun/twitch_exporter)][goreportcard]

Export [Twitch](https://dev.twitch.tv/docs/api/reference) metrics to [Prometheus](https://github.com/prometheus/prometheus).

To run it:

```bash
make
./twitch_exporter [flags]
```

## Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| twitch_channel_up | Is the twitch channel Online. | username, game |
| twitch_channel_viewers_total | Is the total number of viewers on an online twitch channel. | username, game |
| twitch_channel_views_total | Is the total number of views on a twitch channel. | username |
| twitch_channel_followers_total | Is the total number of follower on a twitch channel. | username |
| twitch_channel_subscribers_total | Is the total number of subscriber on a twitch channel. | username, tier, gifted |

### Flags

```bash
./twitch_exporter --help
```

* __`twitch.channel`:__ The name of a twitch channel.
* __`twitch.client-id`:__ The client ID to request the New Twitch API (helix).
* __`twitch.access-token`:__ The access token to request the New Twitch API (helix).
* __`log.format`:__ Set the log target and format. Example: `logger:syslog?appname=bob&local=7`
    or `logger:stdout?json=true`
* __`log.level`:__ Logging level. `info` by default.
* __`version`:__ Show application version.
* __`web.listen-address`:__ Address to listen on for web interface and telemetry.
* __`web.telemetry-path`:__ Path under which to expose metrics.
* __`--[no-]collector.channel_followers_total`:__ Enable the channel_followers_total collector (default: enabled).
* __`--[no-]collector.channel_subscribers_total`:__ Enable the channel_subscribers_total collector (default: disabled).
* __`--[no-]collector.channel_up`:__ Enable the channel_up collector (default: enabled).
* __`--[no-]collector.channel_viewers_total`:__ Enable the channel_viewers_total collector (default: enabled).

## Useful Queries

TODO

## Using Docker

You can deploy this exporter using the [damoun/twitch-exporter](https://hub.docker.com/r/damoun/twitch-exporter/) Docker image.

For example:

```bash
docker pull damoun/twitch-exporter

docker run -d -p 9184:9184 \
        damoun/twitch-exporter \
        --twitch.client-id <secret> \
        --twitch.access-token <secret> \
        --twitch.channel dam0un
```

[circleci]: https://circleci.com/gh/damoun/twitch_exporter
[hub]: https://hub.docker.com/r/damoun/twitch-exporter/
[goreportcard]: https://goreportcard.com/report/github.com/damoun/twitch_exporter
