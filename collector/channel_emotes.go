package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelEmotesCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelEmotes typedDesc
}

func init() {
	registerCollector("channel_emotes_total", defaultEnabled, NewChannelEmotesTotalCollector)
}

func NewChannelEmotesTotalCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelEmotesCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelEmotes: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_emotes"),
			"The number of custom emotes of a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelEmotesCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := getUsers(c.client, c.logger, c.channelNames)
	if err != nil {
		return err
	}

	for _, user := range users {
		emotesResp, err := c.client.GetChannelEmotes(&helix.GetChannelEmotesParams{
			BroadcasterID: user.ID,
		})

		if err != nil {
			c.logger.Error("Failed to collect emotes from Twitch helix API", "err", err)
			return err
		}

		if emotesResp.StatusCode != 200 {
			c.logger.Error("Failed to collect emotes from Twitch helix API", "err", emotesResp.ErrorMessage)
			return errors.New(emotesResp.ErrorMessage)
		}

		ch <- c.channelEmotes.mustNewConstMetric(float64(len(emotesResp.Data.Emotes)), user.DisplayName)
	}

	return nil
}
