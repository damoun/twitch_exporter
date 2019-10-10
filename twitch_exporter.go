// Copyright 2019 Damien PLÉNARD.
// Licensed under the MIT License

package main

import (
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
	twitchChannel = kingpin.Flag("twitch.channel",
		"Name of a Twitch Channel to request metrics.").String()
)

const (
	namespace = "twitch"
)

var (
	channelLive = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channel_live"),
		"Is the channel live.",
		[]string{"username", "game"}, nil,
	)
	channelViewers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channel_viewers_total"),
		"How many viewers on this live channel.",
		[]string{"username", "game"}, nil,
	)
)

func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

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
	ch <- channelLive
	ch <- channelViewers
}

// Collect fetches the stats from configured Twitch Channels and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	channelsName := []string{*twitchChannel}
	resp, err := e.client.GetStreams(&helix.StreamsParams{
		UserLogins: channelsName,
		First:      1,
	})
	if err != nil {
		log.Errorf("Failed to collect stats from Twitch helix API: %v", err)
		return
	}

	for _, stream := range resp.Data.Streams {
		resp, err := e.client.GetGames(&helix.GamesParams{
			IDs: []string{stream.GameID},
		})
		var gameName string
		if err != nil {
			log.Errorf("Failed to get Game name: %v", err)
			gameName = ""
		} else {
			gameName = resp.Data.Games[0].Name
		}
		if stream.Type == "live" {
			ch <- prometheus.MustNewConstMetric(
				channelLive, prometheus.GaugeValue, 1,
				stream.UserName, gameName,
			)
		} else {
			ch <- prometheus.MustNewConstMetric(
				channelLive, prometheus.GaugeValue, 0,
				stream.UserName, gameName,
			)
		}
		ch <- prometheus.MustNewConstMetric(
			channelViewers, prometheus.GaugeValue,
			float64(stream.ViewerCount), stream.UserName, gameName,
		)

		for i, channelName := range channelsName {
			if channelName == stream.UserName {
				remove(channelsName, i)
				break
			}
		}
	}

	for _, channelName := range channelsName {
		ch <- prometheus.MustNewConstMetric(
			channelLive, prometheus.GaugeValue, 0,
			channelName, "",
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
