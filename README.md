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

## Useful Queries

TODO

## Multi-exporter mode

This module supports multi-exporter mode, which allows you to use `/probe` to query for metrics of a channel. An example
of how to use this:

**[Prometheus](https://prometheus.io/docs/guides/multi-target-exporter/#querying-multi-target-exporters-with-prometheus):**
```yaml
scrape_configs:
  - job_name: 'twitch_exporter'
    // we aren't using /metrics this time, we are using /probe and sending a
    // list of targets
    metrics_path: /probe

    static_configs:
        // some targets, unlike alloy prometheus assumes that these will be
        // __address__'s and will sort that for us
      - targets:
        - TimTheTatMan
        - JordOfTheNorth
        - LIRIK
        - SurDaft
        - Dam0un

    // format the config from a human readable config to one suitable for multi-
    // -exporter usage
    relabel_configs:
        // relabel the target from being the __address__ to instead be a param
        // ?target=xxx
      - source_labels: [__address__]
        target_label: __param_target

        // also set a label of instance=, to be in line with other multi-exporters
      - source_labels: [__param_target]
        target_label: instance

        // now define the real location to probe, which is the IP and port of your
        // exporter
      - target_label: __address_
        replacement: 127.0.0.1:9184
```

**[Alloy](https://grafana.com/docs/alloy/latest/):**
```hcl
// export metrics regarding stream viewers and current game
// @see https://github.com/damoun/twitch_exporter
prometheus.scrape "twitch_exporter" {
        targets    = discovery.relabel.twitch_exporter_relabel_config.output
        forward_to = [prometheus.remote_write.grafanacloud.receiver]

        // we aren't using /metrics this time, we are using /probe and sending a list
        // of targets
        metrics_path = "/probe"

        // grafana cloud recommends an ingest rate of 1 metric-per-minute
        // @see https://grafana.com/docs/grafana-cloud/cost-management-and-billing/reduce-costs/metrics-costs/adjust-data-points-per-minute/
        scrape_interval = "60s"
}

// format the config from a human readable config to one suitable for multi-
// -exporter usage
discovery.relabel "twitch_exporter_relabel_config" {
        // some targets, must be a map but name can really be anything, just make
        // sure to update the remap from below, to __param_target
        targets = [
                {name = "TimTheTatMan"},
                {name = "JordOfTheNorth"},
                {name = "LIRIK"},
                {name = "SurDaft"},
                {name = "Dam0un"},
        ]

        // relabel the target to one that is less human-friendly and suitable
        // for the probe, this converts name=xxx to be used as as query arg
        // ?target=xxx
        rule {
                source_labels = ["name"]
                target_label  = "__param_target"
        }

        // also set a label of instance=, to be in line with other multi-exporters
        rule {
                source_labels = ["__param_target"]
                target_label  = "instance"
        }

        // now define the real location to probe, which is the IP and port of your
        // exporter
        rule {
                target_label = "__address__"
                replacement  = "127.0.0.1:9184"
        }
}
```

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
