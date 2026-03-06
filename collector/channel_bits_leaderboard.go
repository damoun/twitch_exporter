package collector

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type ChannelBitsLeaderboardCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelBitsLeaderboard typedDesc
}

func init() {
	registerCollector("channel_bits_leaderboard", defaultDisabled, NewChannelBitsLeaderboardCollector)
}

func NewChannelBitsLeaderboardCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := ChannelBitsLeaderboardCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelBitsLeaderboard: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_bits_leaderboard"),
			"The bits leaderboard score for users on a channel.",
			[]string{"channel", "user_name", "user_id", "rank"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c ChannelBitsLeaderboardCollector) Update(ch chan<- prometheus.Metric) error {
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

	for _, user := range usersResp.Data.Users {
		bitsResp, err := c.client.GetBitsLeaderboard(&helix.BitsLeaderboardParams{
			Count: 100,
		})

		if err != nil {
			c.logger.Error("Failed to collect bits leaderboard from Twitch helix API", "err", err)
			return err
		}

		if bitsResp.StatusCode != 200 {
			c.logger.Error("Failed to collect bits leaderboard from Twitch helix API", "err", bitsResp.ErrorMessage)
			return errors.New(bitsResp.ErrorMessage)
		}

		for _, entry := range bitsResp.Data.UserBitTotals {
			ch <- c.channelBitsLeaderboard.mustNewConstMetric(
				float64(entry.Score),
				user.DisplayName,
				entry.UserName,
				entry.UserID,
				strconv.Itoa(entry.Rank),
			)
		}
	}

	return nil
}
