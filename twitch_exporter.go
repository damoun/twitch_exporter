// Copyright 2019 Damien PLÉNARD.
// Licensed under the MIT License

package main

import (
	"fmt"
	"net/http"

	"github.com/nicklaw5/helix"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddress = kingpin.Flag("web.listen-address",
		"Address to listen on for web interface and telemetry.").
		Default(":9184").String()
	metricsPath = kingpin.Flag("web.telemetry-path",
		"Path under which to expose metrics.").
		Default("/metrics").String()
	twitchClientID = kingpin.Flag("twitch.client-id",
		"Client ID for the Twitch Helix API.").Required().String()
	twitchChannel = Channels(kingpin.Flag("twitch.channel",
		"Name of a Twitch Channel to request metrics."))
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
)

// Exporter collects Twitch metrics from the helix API and exports them using
// the Prometheus metrics package.
type Exporter struct {
	client *helix.Client
}

// NewExporter returns an initialized Exporter.
func NewExporter() (*Exporter, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID: *twitchClientID,
	})
	if err != nil {
		return nil, err
	}

	return &Exporter{
		client: client,
	}, nil
}

// Describe describes all the metrics ever exported by the Twitch exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- channelUp
	ch <- channelViewers
	ch <- channelFollowers
	ch <- channelViews
}

// Collect fetches the stats from configured Twitch Channels and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	channelsLive := make(map[string]bool)
	streamsResp, err := e.client.GetStreams(&helix.StreamsParams{
		UserLogins: *twitchChannel,
		First:      len(*twitchChannel),
	})
	if err != nil {
		log.Errorf("Failed to collect stats from Twitch helix API: %v", err)
		return
	}

	for _, stream := range streamsResp.Data.Streams {
		gamesResp, err := e.client.GetGames(&helix.GamesParams{
			IDs: []string{stream.GameID},
		})
		var gameName string
		if err != nil {
			log.Errorf("Failed to get Game name: %v", err)
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

	for _, channelName := range *twitchChannel {
		if _, ok := channelsLive[channelName]; !ok {
			ch <- prometheus.MustNewConstMetric(
				channelUp, prometheus.GaugeValue, 0,
				channelName, "",
			)
			channelsLive[channelName] = false
		}
	}

	usersResp, err := e.client.GetUsers(&helix.UsersParams{
		Logins: *twitchChannel,
	})
	if err != nil {
		log.Errorf("Failed to collect stats from Twitch helix API: %v", err)
		return
	}
	for _, user := range usersResp.Data.Users {
		usersFollowsResp, err := e.client.GetUsersFollows(&helix.UsersFollowsParams{
			ToID: user.ID,
		})
		if err != nil {
			log.Errorf("Failed to collect stats from Twitch helix API: %v", err)
			return
		}
		ch <- prometheus.MustNewConstMetric(
			channelFollowers, prometheus.GaugeValue,
			float64(usersFollowsResp.Data.Total), user.Login,
		)
		ch <- prometheus.MustNewConstMetric(
			channelViews, prometheus.GaugeValue,
			float64(user.ViewCount), user.Login,
		)
	}
}

func init() {
	prometheus.MustRegister(version.NewCollector("twitch_exporter"))
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("twitch_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting twitch_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	exporter, err := NewExporter()
	if err != nil {
		log.Fatalln(err)
	}
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.Handler())
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

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
