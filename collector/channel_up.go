package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/config"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelUpCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames *config.ChannelNames

	channelUp typedDesc
}

func init() {
	registerCollector("channel_up", defaultEnabled, NewChannelUpCollector)
}

func NewChannelUpCollector(logger *slog.Logger, client *helix.Client, cfg *config.Config) (Collector, error) {
	c := channelUpCollector{
		logger:       logger,
		client:       client,
		channelNames: cfg.Twitch.Channels,

		channelUp: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_up"),
			"Is the channel live.",
			[]string{"username", "game"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelUpCollector) Update(ch chan<- prometheus.Metric) error {
	if len(*c.channelNames) == 0 {
		return ErrNoData
	}

	streamsResp, err := c.client.GetStreams(&helix.StreamsParams{
		UserLogins: *c.channelNames,
		First:      len(*c.channelNames),
	})

	if err != nil {
		c.logger.Error("could not get streams", "err", err.Error())
		return errors.Join(errors.New("could not get streams"), err)
	}

	for _, n := range *c.channelNames {
		state := 0
		game := ""

		for _, s := range streamsResp.Data.Streams {
			if s.UserName == n {
				state = 1
				game = s.GameName
				break
			}
		}

		ch <- c.channelUp.mustNewConstMetric(float64(state), n, game)
	}

	return nil
}
