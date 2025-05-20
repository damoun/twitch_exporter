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
| twitch_channel_chat_messages_total | Is the total number of chat messages from a user within a channel. | username, chatter_username |

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
* __`eventsub.enabled`:__ Enable eventsub endpoint (default: false).
* __`eventsub.webhook-url`:__ The url your collector will be expected to be hosted at, eg: http://example.svc/eventsub (Must end with `/eventsub`).
* __`eventsub.secret`:__ Secure 1-100 character secret for your eventsub validation
* __`--[no-]collector.channel_followers_total`:__ Enable the channel_followers_total collector (default: enabled).
* __`--[no-]collector.channel_subscribers_total`:__ Enable the channel_subscribers_total collector (default: disabled*).
* __`--[no-]collector.channel_up`:__ Enable the channel_up collector (default: enabled).
* __`--[no-]collector.channel_viewers_total`:__ Enable the channel_viewers_total collector (default: enabled).
* __`--[no-]collector.channel_chat_messages_total`:__ Enable the channel_chat_messages_total (default: disabled**).

```
* Disabled due to the requirement of a user access token, which must be acquired outside of the collector
** Disabled due to event-sub being disabled by default
```

## Event-sub

Event-sub metrics are disabled by default due to requiring a public endpoint to be exposed and more permissions and setup.
Due to the likeliness that you do not want to expose the service publicly and go through too much effort, it is disabled by
default.

If you wish to use eventsub based metrics then you should deploy an instance of the exporter just for the user that needs
the eventsub metrics, such as your own channel, and just collect the privileged metrics using that exporter.

> Todo: Instead of disabling all other collectors, functionality to set collectors should be implemented

### Setting up eventsub metrics

You can read more about the process [here](https://dev.twitch.tv/docs/chat/authenticating/)

1. Install the twitch-cli
1. Ensure your twitch app has localhost:3000 added as a redirect uri
1. `twitch token -u -s 'channel:bot user:chat:read user:bot channel:read:subscriptions'` (At this point, since you are getting a user token, you may as well get the subscriptions metric too)
1. Start the collector with `client-id`, `client-secret`, `access-token`, and `refresh-token` defined

```
go run twitch_exporter.go \
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

## Using Helmfile
You are able to use a helm chart to manage your exporter, create a file named
`helmfile.yaml` and then add this:

```yaml
repositories:
  - name: twitch-exporter
    url: https://damoun.github.com/twitch_exporter/
  - name: grafana
    url: https://grafana.github.io/helm-charts

releases:
  - name: alloy
    namespace: twitch-exporter
    chart: grafana/alloy
    values:
      - ./alloy.values.yaml

  - name: twitch-exporter
    namespace: twitch-exporter
    chart: twitch-exporter/twitch-exporter
    values:
      - ./twitch-exporter.values.yaml
```

Then create a file called `alloy.values.yaml`:
```yaml
alloy:
  configMap:
    create: true
    content: |-
      prometheus.remote_write "mimir" {
        endpoint {
          # make sure to update this url with your proper push endpoint info,
          # cloud for example required authentication.
          url = "xxx/api/v1/push"
        }
      }

      prometheus.scrape "twitch_exporter_metrics" {
        targets         = [{__address__ = "twitch-exporter.twitch-exporter.svc:9184"}]
        metrics_path    = "/metrics"
        forward_to      = [prometheus.remote_write.mimir.receiver]
        scrape_timeout  = "1m"
        # twitch cache is going to be a pain anyway, so 5m scrape helps with any
        # potential rate limits and works around cache
        scrape_interval = "5m"
      }
```

Create a file named `twitch-exporter.values.yaml`
```yaml
twitch:
  clientId: "muy2fhyb2esa49w3n70fpumxr78ruh"
  accessToken: "dvmkxzay1xi4erxfu0x56h6qsfzukj"
  channels:
    - jordofthenorth
    - timthetatman
    - dam0un
    - surdaft
```

> Note: You can add see more config options in charts/twitch-exporter/values.yaml.
> Ingress is disabled by default, however you can enable it to allow for public
> access to your exporter. Such as if you use a firewall and scrape from another
> device.

[circleci]: https://circleci.com/gh/damoun/twitch_exporter
[hub]: https://hub.docker.com/r/damoun/twitch-exporter/
[goreportcard]: https://goreportcard.com/report/github.com/damoun/twitch_exporter
