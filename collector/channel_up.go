package collector

import (
	"log/slog"
	"strings"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelUpCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelUp typedDesc
}

func init() {
	registerCollector("channel_up", defaultEnabled, AuthApp, NewChannelUpCollector)
}

func NewChannelUpCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelUpCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelUp: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_up"),
			"Is the channel live.",
			[]string{"username", "game"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelUpCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	// GetStreams returns only channels that are currently live, batched into
	// grouped requests of at most maxHelixIDsPerRequest logins.
	liveGames := make(map[string]string)
	for _, chunk := range chunkStrings(c.channelNames, maxHelixIDsPerRequest) {
		streamsResp, err := c.client.GetStreams(&helix.StreamsParams{
			UserLogins: chunk,
			First:      len(chunk),
		})

		if err != nil {
			c.logger.Error("could not get streams", "err", err)
			return err
		}

		for _, s := range streamsResp.Data.Streams {
			liveGames[strings.ToLower(s.UserLogin)] = s.GameName
		}
	}

	for _, n := range c.channelNames {
		// Label with the login (lower-cased): stable across display-name case
		// changes and matches the canonical login the API returns.
		login := strings.ToLower(n)

		state := 0
		game := ""

		if g, ok := liveGames[login]; ok {
			state = 1
			game = g
		}

		ch <- c.channelUp.mustNewConstMetric(float64(state), login, game)
	}

	return nil
}
