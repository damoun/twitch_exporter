// Copyright 2020 Damien PLÉNARD.
// Licensed under the MIT License

package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/damoun/twitch_exporter/collector"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

var (
	metricsPath = kingpin.Flag("web.telemetry-path",
		"Path under which to expose metrics.").
		Default("/metrics").String()
	twitchClientID = kingpin.Flag("twitch.client-id",
		"Client ID for the Twitch Helix API.").Required().String()
	twitchChannel = Channels(kingpin.Flag("twitch.channel",
		"Name of a Twitch Channel to request metrics."))
	twitchAccessToken = kingpin.Flag("twitch.access-token",
		"Access Token for the Twitch Helix API.").Required().String()
)

type promHTTPLogger struct {
	logger *slog.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	l.logger.Error("msg", fmt.Sprint(v...))
}

// Channels creates a collection of Channels from a kingpin command line argument.
func Channels(s kingpin.Settings) (target *collector.ChannelNames) {
	target = &collector.ChannelNames{}
	s.SetValue(target)
	return target
}

func init() {
	prometheus.MustRegister(versioncollector.NewCollector("twitch_exporter"))
}

func main() {
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	var webConfig = webflag.AddFlags(kingpin.CommandLine, ":9184")
	kingpin.Version(version.Print("twitch_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)
	logger.Info("msg", "Starting twitch_exporter", "version", version.Info())
	logger.Info("build_context", version.BuildContext())

	client, err := helix.NewClient(&helix.Options{
		ClientID:        *twitchClientID,
		UserAccessToken: *twitchAccessToken,
	})

	if err != nil {
		logger.Error("msg", "could not initialise twitch client", "err", err)
		return
	}

	exporter, err := collector.NewExporter(logger, client, *twitchChannel)
	if err != nil {
		logger.Error("msg", "Error creating the exporter", "err", err)
		os.Exit(1)
	}

	r := prometheus.NewRegistry()
	r.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.HandlerFor(r, promhttp.HandlerOpts{
		ErrorLog:      promHTTPLogger{logger: logger},
		ErrorHandling: promhttp.ContinueOnError,
	}))

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
		logger.Error("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
