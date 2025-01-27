// Copyright 2020 Damien PLÉNARD.
// Licensed under the MIT License

package main

import (
	"fmt"
	"net/http"
	"os"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	helix "github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

var (
	metricsPath = kingpin.Flag("web.telemetry-path",
		"Path under which to expose metrics.").
		Default("/metrics").String()
	probePath = kingpin.Flag("web.probe-path",
		"Path under which to expose probe metrics.").
		Default("/probe").String()
	twitchClientID = kingpin.Flag("twitch.client-id",
		"Client ID for the Twitch Helix API.").Required().String()
	twitchChannel = Channels(kingpin.Flag("twitch.channel",
		"Name of a Twitch Channel to request metrics. (/probe will not use this value and instead use the ?target= query param)"))
	twitchAccessToken = kingpin.Flag("twitch.access-token",
		"Access Token for the Twitch Helix API.").Required().String()
)

const (
	namespace = "twitch"
)

// ChannelNames represents a list of twitch channels.
type ChannelNames []string

// IsCumulative is required for kingpin interfaces to allow multiple values
func (c ChannelNames) IsCumulative() bool {
	return true
}

// Set sets the value of a ChannelNames
func (c *ChannelNames) Set(v string) error {
	*c = append(*c, v)
	return nil
}

// String returns a string representation of the Channels type.
func (c ChannelNames) String() string {
	return fmt.Sprintf("%v", []string(c))
}

// Channels creates a collection of Channels from a kingpin command line argument.
func Channels(s kingpin.Settings) (target *ChannelNames) {
	target = &ChannelNames{}
	s.SetValue(target)
	return target
}

var (
	channelUp = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channel_up"),
		"Is the channel live.",
		[]string{"username", "game"}, nil,
	)
	channelViewers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channel_viewers_total"),
		"How many viewers on this live channel.",
		[]string{"username", "game"}, nil,
	)
	channelFollowers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channel_followers_total"),
		"The number of followers of a channel.",
		[]string{"username"}, nil,
	)
	channelViews = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channel_views_total"),
		"The number of view of a channel.",
		[]string{"username"}, nil,
	)
	channelSubscribers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channel_subscribers_total"),
		"The number of subscriber of a channel.",
		[]string{"username", "tier", "gifted"}, nil,
	)
)

type promHTTPLogger struct {
	logger log.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	level.Error(l.logger).Log("msg", fmt.Sprint(v...))
}

// Exporter collects Twitch metrics from the helix API and exports them using
// the Prometheus metrics package.
type Exporter struct {
	client *helix.Client
	logger log.Logger

	twitchChannels ChannelNames
}

// NewExporter returns an initialized Exporter.
func NewExporter(logger log.Logger, twitchChannels ChannelNames) (*Exporter, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:        *twitchClientID,
		UserAccessToken: *twitchAccessToken,
	})
	if err != nil {
		return nil, err
	}

	return &Exporter{
		client: client,
		logger: logger,

		twitchChannels: twitchChannels,
	}, nil
}

// Describe describes all the metrics ever exported by the Twitch exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- channelUp
	ch <- channelViewers
	ch <- channelFollowers
	ch <- channelViews
	ch <- channelSubscribers
}

// Collect fetches the stats from configured Twitch Channels and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	channelsLive := make(map[string]bool)
	streamsResp, err := e.client.GetStreams(&helix.StreamsParams{
		UserLogins: e.twitchChannels,
		First:      len(e.twitchChannels),
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to collect stats from Twitch helix API", "err", err)
		return
	}

	if streamsResp.StatusCode != 200 {
		level.Error(e.logger).Log("msg", "Failed to collect stats from Twitch helix API", "err", streamsResp.ErrorMessage)
		return
	}

	for _, stream := range streamsResp.Data.Streams {
		gamesResp, err := e.client.GetGames(&helix.GamesParams{
			IDs: []string{stream.GameID},
		})
		var gameName string
		if err != nil {
			level.Error(e.logger).Log("msg", "Failed to get Game name", "err", err)
			gameName = ""
		} else {
			gameName = gamesResp.Data.Games[0].Name
		}
		channelsLive[stream.UserName] = true
		ch <- prometheus.MustNewConstMetric(
			channelUp, prometheus.GaugeValue, 1,
			stream.UserName, gameName,
		)
		ch <- prometheus.MustNewConstMetric(
			channelViewers, prometheus.GaugeValue,
			float64(stream.ViewerCount), stream.UserName, gameName,
		)
	}

	usersResp, err := e.client.GetUsers(&helix.UsersParams{
		Logins: e.twitchChannels,
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to collect users stats from Twitch helix API", "err", err)
		return
	}

	if usersResp.StatusCode != 200 {
		level.Warn(e.logger).Log("msg", "Failed to collect users stats from Twitch helix API", "err", usersResp.ErrorMessage)
	}

	for _, user := range usersResp.Data.Users {
		usersFollowsResp, err := e.client.GetChannelFollows(&helix.GetChannelFollowsParams{
			BroadcasterID: user.ID,
		})
		if err != nil {
			level.Error(e.logger).Log("msg", "Failed to collect follower stats from Twitch helix API", "err", err)
			return
		}

		if usersFollowsResp.StatusCode != 200 {
			level.Warn(e.logger).Log("msg", "Failed to collect follower stats from Twitch helix API", "err", usersFollowsResp.ErrorMessage)
		}

		if _, ok := channelsLive[user.DisplayName]; !ok {
			ch <- prometheus.MustNewConstMetric(
				channelUp, prometheus.GaugeValue, 0,
				user.DisplayName, "",
			)
		}
		ch <- prometheus.MustNewConstMetric(
			channelFollowers, prometheus.GaugeValue,
			float64(usersFollowsResp.Data.Total), user.DisplayName,
		)
		ch <- prometheus.MustNewConstMetric(
			channelViews, prometheus.GaugeValue,
			float64(user.ViewCount), user.DisplayName,
		)
		subscribtionsResp, err := e.client.GetSubscriptions(&helix.SubscriptionsParams{
			BroadcasterID: user.ID,
		})
		if err != nil {
			level.Error(e.logger).Log("msg", "Failed to collect subscribers stats from Twitch helix API", "err", err)
			return
		}

		if subscribtionsResp.StatusCode != 200 {
			level.Warn(e.logger).Log("msg", "Failed to collect subscirbers stats from Twitch helix API", "err", subscribtionsResp.ErrorMessage)
		}

		subCounter := make(map[string]int)
		giftedSubCounter := make(map[string]int)
		for _, subscription := range subscribtionsResp.Data.Subscriptions {
			if subscription.IsGift {
				if _, ok := giftedSubCounter[subscription.Tier]; !ok {
					giftedSubCounter[subscription.Tier] = 0
				}
				giftedSubCounter[subscription.Tier] = giftedSubCounter[subscription.Tier] + 1
			} else {
				if _, ok := subCounter[subscription.Tier]; !ok {
					subCounter[subscription.Tier] = 0
				}
				subCounter[subscription.Tier] = subCounter[subscription.Tier] + 1
			}
		}
		for tier, counter := range giftedSubCounter {
			ch <- prometheus.MustNewConstMetric(
				channelSubscribers, prometheus.GaugeValue,
				float64(counter), user.DisplayName, tier, "true",
			)
		}
		for tier, counter := range subCounter {
			ch <- prometheus.MustNewConstMetric(
				channelSubscribers, prometheus.GaugeValue,
				float64(counter), user.DisplayName, tier, "false",
			)
		}
	}
}

func init() {
	prometheus.MustRegister(versioncollector.NewCollector("twitch_exporter"))
}

func main() {
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	var webConfig = webflag.AddFlags(kingpin.CommandLine, ":9184")
	kingpin.Version(version.Print("twitch_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(promlogConfig)
	level.Info(logger).Log("msg", "Starting twitch_exporter", "version", version.Info())
	level.Info(logger).Log("build_context", version.BuildContext())

	exporter, err := NewExporter(logger, *twitchChannel)
	if err != nil {
		level.Error(logger).Log("msg", "Error creating the exporter", "err", err)
		os.Exit(1)
	}
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc(*probePath, probeHandler(logger))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
             <head><title>Twitch Exporter</title></head>
             <body>
             <h1>Twitch Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             <h2>Build</h2>
             <pre>` + version.Info() + ` ` + version.BuildContext() + `</pre>
             </body>
             </html>`))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	srv := &http.Server{}
	if err := web.ListenAndServe(srv, webConfig, logger); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}

func probeHandler(logger log.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		probeTarget := ChannelNames{}

		if r.URL.Query().Has("target") {
			probeTarget = append(probeTarget, r.URL.Query().Get("target"))
		}

		exporter, err := NewExporter(logger, probeTarget)
		if err != nil {
			level.Error(logger).Log("msg", "Error creating exporter for probe target", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// we need a fresh prometheus registry, to only export the metrics for the probe
		registry := prometheus.NewRegistry()
		registry.MustRegister(exporter)

		// we now manually execute the handler, as though the `/emtrics` endpoint would
		// but with our configured exporter which contains the probe target
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}
