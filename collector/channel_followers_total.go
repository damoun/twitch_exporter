package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/config"
	"github.com/damoun/twitch_exporter/twitch"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelFollowersTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames *config.ChannelNames

	channelFollowers typedDesc
}

func init() {
	registerCollector("channel_followers_total", defaultEnabled, NewChannelFollowersTotalCollector)
}

func NewChannelFollowersTotalCollector(logger *slog.Logger, client *helix.Client, cfg *config.Config) (Collector, error) {
	c := channelFollowersTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: cfg.Twitch.Channels,

		channelFollowers: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_followers_total"),
			"The number of followers of a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelFollowersTotalCollector) Update(ch chan<- prometheus.Metric) error {
	if len(*c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := twitch.GetUsersByUsername(c.logger, c.client, *c.channelNames)
	if err != nil {
		err = errors.Join(errors.New("failed to get user by username for channel_followers_total"), err)
		return err
	}

	for _, user := range *users {
		usersFollowsResp, err := c.client.GetChannelFollows(&helix.GetChannelFollowsParams{
			BroadcasterID: user.ID,
		})

		if err != nil {
			c.logger.Error("Failed to collect follower stats from Twitch helix API", "err", err.Error(), "user", user.DisplayName)
			return errors.Join(errors.New("failed to collect follower stats from Twitch helix API"), err)
		}

		if usersFollowsResp.StatusCode != 200 {
			c.logger.Error("Failed to collect follower stats from Twitch helix API", "err", usersFollowsResp.ErrorMessage, "user", user.DisplayName)
			return errors.Join(errors.New("failed to collect follower stats from Twitch helix API"), errors.New(usersFollowsResp.ErrorMessage))
		}

		ch <- c.channelFollowers.mustNewConstMetric(float64(usersFollowsResp.Data.Total), user.DisplayName)
	}

	return nil
}
