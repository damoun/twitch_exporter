// Copyright 2020 Damien PLÉNARD.
// Licensed under the MIT License

package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/damoun/twitch_exporter/collector"
	"github.com/damoun/twitch_exporter/config"
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
	r  = prometheus.NewRegistry()
	sc = config.NewSafeConfig(r)

	configFile  = kingpin.Flag("config.file", "Twitch exporter configuration file.").Default("twitch_exporter.yml").String()
	metricsPath = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
)

type promHTTPLogger struct {
	logger *slog.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	slog.Info(fmt.Sprint(v...))
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

	logger.Info("Starting twitch_exporter", "version", version.Info())
	logger.Info("build_context", "context", version.BuildContext())

	if err := sc.ReloadConfig(*configFile, logger); err != nil {
		logger.Error("Error loading config", "err", err)
		os.Exit(1)
	}

	exporter, err := collector.NewExporter(logger, sc)
	if err != nil {
		logger.Error("Error creating the exporter", "err", err.Error())
		os.Exit(1)
	}

	r.MustRegister(exporter)

	hup := make(chan os.Signal, 1)
	reloadCh := make(chan chan error)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-hup:
				if err := sc.ReloadConfig(*configFile, logger); err != nil {
					logger.Error("Error reloading config", "err", err)
					continue
				}

				exporter.Reload()

				logger.Info("Reloaded config file")
			case rc := <-reloadCh:
				if err := sc.ReloadConfig(*configFile, logger); err != nil {
					logger.Error("Error reloading config", "err", err)
					rc <- err
				} else {
					exporter.Reload()

					logger.Info("Reloaded config file")
					rc <- nil
				}
			}
		}
	}()

	http.HandleFunc("/-/reload",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				fmt.Fprintf(w, "This endpoint requires a POST request.\n")
				return
			}

			rc := make(chan error)
			reloadCh <- rc
			if err := <-rc; err != nil {
				http.Error(w, fmt.Sprintf("failed to reload config: %s", err), http.StatusInternalServerError)
			}
		})

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
	srvc := make(chan struct{})
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := web.ListenAndServe(srv, webConfig, logger); err != nil {
			slog.Error("Error starting HTTP server", "err", err.Error())
			os.Exit(1)
		}
	}()

	for {
		select {
		case <-term:
			logger.Info("Received SIGTERM")
			return
		case <-srvc:
			return
		}
	}
}
