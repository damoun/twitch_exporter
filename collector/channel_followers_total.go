package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelFollowersTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelFollowers typedDesc
}

func init() {
	registerCollector("channel_followers_total", defaultEnabled, NewChannelFollowersTotalCollector)
}

func NewChannelFollowersTotalCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelFollowersTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelFollowers: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_followers_total"),
			"The number of followers of a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelFollowersTotalCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

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

	// todo: we can avoid this with a shared cache of username to userID that has a short TTL
	for _, user := range usersResp.Data.Users {
		usersFollowsResp, err := c.client.GetChannelFollows(&helix.GetChannelFollowsParams{
			BroadcasterID: user.ID,
		})

		if err != nil {
			c.logger.Error("Failed to collect follower stats from Twitch helix API", "err", err)
			return err
		}

		if usersFollowsResp.StatusCode != 200 {
			c.logger.Error("Failed to collect follower stats from Twitch helix API", "err", usersFollowsResp.ErrorMessage)
			return errors.New(usersFollowsResp.ErrorMessage)
		}

		ch <- c.channelFollowers.mustNewConstMetric(float64(usersFollowsResp.Data.Total), user.DisplayName)
	}

	return nil
}
