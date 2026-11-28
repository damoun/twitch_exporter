package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelChattersCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelChatters typedDesc
}

func init() {
	registerCollector("channel_chatters_total", defaultDisabled, NewChannelChattersCollector)
}

func NewChannelChattersCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelChattersCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelChatters: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_chatters"),
			"The number of users in a channel's chat.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelChattersCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	// Get authenticated user ID to use as moderator ID
	authUsers, err := getUsers(c.client, c.logger, nil)
	if err != nil {
		return err
	}

	if len(authUsers) == 0 {
		return ErrNoData
	}

	moderatorID := authUsers[0].ID

	users, err := getUsers(c.client, c.logger, c.channelNames)
	if err != nil {
		return err
	}

	for _, user := range users {
		chattersResp, err := c.client.GetChannelChatChatters(&helix.GetChatChattersParams{
			BroadcasterID: user.ID,
			ModeratorID:   moderatorID,
		})

		if err != nil {
			c.logger.Error("Failed to collect chatters from Twitch helix API", "err", err)
			return err
		}

		if chattersResp.StatusCode != 200 {
			c.logger.Error("Failed to collect chatters from Twitch helix API", "err", chattersResp.ErrorMessage)
			return errors.New(chattersResp.ErrorMessage)
		}

		ch <- c.channelChatters.mustNewConstMetric(float64(chattersResp.Data.Total), user.DisplayName)
	}

	return nil
}
