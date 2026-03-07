package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type ChannelChattersCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelChattersTotal typedDesc
}

func init() {
	registerCollector("channel_chatters_total", defaultDisabled, NewChannelChattersCollector)
}

func NewChannelChattersCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := ChannelChattersCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelChattersTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_chatters_total"),
			"The number of users in a channel's chat.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c ChannelChattersCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	// Get authenticated user ID to use as moderator ID
	authUserResp, err := c.client.GetUsers(&helix.UsersParams{})
	if err != nil {
		c.logger.Error("Failed to collect authenticated user from Twitch helix API", "err", err)
		return err
	}

	if authUserResp.StatusCode != 200 {
		c.logger.Error("Failed to collect authenticated user from Twitch helix API", "err", authUserResp.ErrorMessage)
		return errors.New(authUserResp.ErrorMessage)
	}

	if len(authUserResp.Data.Users) == 0 {
		return ErrNoData
	}

	moderatorID := authUserResp.Data.Users[0].ID

	usersResp, err := c.client.GetUsers(&helix.UsersParams{
		Logins: c.channelNames,
	})

	if err != nil {
		c.logger.Error("Failed to collect users stats from Twitch helix API", "err", err)
		return err
	}

	if usersResp.StatusCode != 200 {
		c.logger.Error("Failed to collect users stats from Twitch helix API", "err", usersResp.ErrorMessage)
		return errors.New(usersResp.ErrorMessage)
	}

	for _, user := range usersResp.Data.Users {
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

		ch <- c.channelChattersTotal.mustNewConstMetric(float64(chattersResp.Data.Total), user.DisplayName)
	}

	return nil
}
