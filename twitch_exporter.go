// Copyright 2020 Damien PLÉNARD.
// Licensed under the MIT License

package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/damoun/twitch_exporter/collector"
	"github.com/damoun/twitch_exporter/internal/eventsub"
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

	// twitch app access token config
	twitchClientID = kingpin.Flag("twitch.client-id",
		"Client ID for the Twitch Helix API.").Required().String()
	twitchClientSecret = kingpin.Flag("twitch.client-secret",
		"Client Secret for the Twitch Helix API.").String()

	// twitch client access token config
	twitchAccessToken = kingpin.Flag("twitch.access-token",
		"Access Token for the Twitch Helix API.").String()
	twitchRefreshToken = kingpin.Flag("twitch.refresh-token",
		"Refresh Token for the Twitch Helix API.").String()
	eventSubEnabled = kingpin.Flag("eventsub.enabled",
		"Enable the Twitch Eventsub API.").Default("false").Bool()
	eventSubWebhookURL = kingpin.Flag("eventsub.webhook-url",
		"Webhook URL for the Twitch Eventsub API.").Default("").String()
	eventSubWebhookSecret = kingpin.Flag("eventsub.webhook-secret",
		"Webhook Secret for the Twitch Eventsub API.").Default("").String()

	// collector configs
	// the twitch channel is a global config for all collectors, and is
	// defined at the root level. Individual collectors may have their own
	// configurations, which are defined within the collector itself.
	twitchChannel = Channels(kingpin.Flag("twitch.channel",
		"Name of a Twitch Channel to request metrics."))
)

type promHTTPLogger struct {
	logger *slog.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	l.logger.Error(fmt.Sprint(v...))
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

	var webConfig = webflag.AddFlags(kingpin.CommandLine, "0.0.0.0:9184")
	kingpin.Version(version.Print("twitch_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)
	logger.Info("Starting twitch_exporter", "version", version.Info())
	logger.Info("", "build_context", version.BuildContext())

	var client *helix.Client
	var err error

	clientType := "app"

	if *twitchClientID == "" || *twitchClientSecret == "" {
		logger.Error("Error creating the client", "err", "client ID and secret are required")
		os.Exit(1)
	}

	if *twitchAccessToken != "" && *twitchRefreshToken != "" {
		clientType = "user"
	}

	logger.Info("client type determined", "clientType", clientType)

	switch clientType {
	case "app":
		client, err = newClientWithSecret(logger)
		if err != nil {
			logger.Error("Error creating the client", "err", err)
			os.Exit(1)
		}
	case "user":
		client, err = newClientWithUserAccessToken(logger)
		if err != nil {
			logger.Error("Error creating the client", "err", err)
			os.Exit(1)
		}
	}

	var eventsubClient *eventsub.Client

	if *eventSubEnabled {
		logger.Info("eventsub endpoint enabled", "endpoint", "/eventsub")

		var appClient *helix.Client

		// eventsub requires an app client to create webhooks, but we may have created a user client
		// beforehand for subscription metrics, so just check and create the app client if needed
		if clientType == "user" {
			appClient, err = newClientWithSecret(logger)
			if err != nil {
				logger.Error("Error creating the client", "err", err)
				os.Exit(1)
			}
		} else {
			appClient = client
		}

		if *eventSubWebhookURL == "" || *eventSubWebhookSecret == "" {
			logger.Error("Error creating the eventsub client", "err", "webhook URL and secret are required")
			os.Exit(1)
		}

		eventsubClient, err = eventsub.New(
			*twitchClientID,
			*twitchClientSecret,
			*eventSubWebhookURL,
			*eventSubWebhookSecret,
			logger,
			appClient,
		)

		if err != nil {
			logger.Error("Error creating the eventsub client", "err", err)
			os.Exit(1)
		}

		// expose the eventsub endpoint
		http.HandleFunc("/eventsub", eventsubClient.Handler())
	}

	exporter, err := collector.NewExporter(logger, client, eventsubClient, *twitchChannel)
	if err != nil {
		logger.Error("Error creating the exporter", "err", err)
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
		logger.Error("Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}

func refreshAppAccessToken(logger *slog.Logger, client *helix.Client) {
	appAccessToken, err := client.RequestAppAccessToken([]string{})
	if err != nil {
		logger.Error("Error getting app access token", "err", err)
		return
	}

	if appAccessToken.ErrorStatus != 0 {
		logger.Error("Error getting app access token", "err", appAccessToken.ErrorMessage)
		return
	}

	client.SetAppAccessToken(appAccessToken.Data.AccessToken)
}

func refreshUserAccessToken(logger *slog.Logger, client *helix.Client) {
	userAccessToken, err := client.RefreshUserAccessToken(client.GetRefreshToken())
	if err != nil {
		logger.Error("Error getting user access token", "err", err)
		return
	}

	if userAccessToken.ErrorStatus != 0 {
		logger.Error("Error getting user access token", "err", userAccessToken.ErrorMessage)
		return
	}

	client.SetUserAccessToken(userAccessToken.Data.AccessToken)
}

// newClientWithSecret creates a new Twitch client with the use of an app access
// token.
func newClientWithSecret(logger *slog.Logger) (*helix.Client, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:     *twitchClientID,
		ClientSecret: *twitchClientSecret,
	})

	if err != nil {
		logger.Error("could not initialise twitch client", "err", err)
		return nil, err
	}

	refreshAppAccessToken(logger, client)

	// now set a ticker for ensuring the access token is refreshed, app access
	// tokens do not return a refresh token, so we need to refresh them every
	// 24 hours.
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	go func(logger *slog.Logger, client *helix.Client) {
		for range ticker.C {
			refreshAppAccessToken(logger, client)
		}
	}(logger, client)

	return client, nil
}

// newClientWithUserAccessToken creates a new Twitch client with a user access token.
// this is required for private data, such as subscriber counts.
func newClientWithUserAccessToken(logger *slog.Logger) (*helix.Client, error) {
	// providing a refresh token allows the helix client to refresh the access
	// token when it expires. this is done automatically when using the helix
	// client.
	client, err := helix.NewClient(&helix.Options{
		ClientID:        *twitchClientID,
		ClientSecret:    *twitchClientSecret,
		UserAccessToken: *twitchAccessToken,
		RefreshToken:    *twitchRefreshToken,
	})

	if err != nil {
		logger.Error("Error creating the client", "err", err)
		return nil, err
	}

	// it may be redundant to refresh the access token here, but it's done
	// anyway to ensure the access token is always valid, in case the parameters
	// are outdated
	refreshUserAccessToken(logger, client)

	// now set a ticker for ensuring the access token is refreshed, app access
	// tokens do not return a refresh token, so we need to refresh them every
	// 24 hours.
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	go func(logger *slog.Logger, client *helix.Client) {
		for range ticker.C {
			refreshUserAccessToken(logger, client)
		}
	}(logger, client)

	return client, nil
}
