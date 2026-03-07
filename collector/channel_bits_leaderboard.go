package collector

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelBitsLeaderboardCollector struct {
	logger *slog.Logger
	client *helix.Client

	channelBitsLeaderboard typedDesc
}

func init() {
	registerCollector("channel_bits_leaderboard", defaultDisabled, NewChannelBitsLeaderboardCollector)
}

func NewChannelBitsLeaderboardCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, _ ChannelNames) (Collector, error) {
	c := channelBitsLeaderboardCollector{
		logger: logger,
		client: client,

		channelBitsLeaderboard: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_bits_leaderboard"),
			"The bits leaderboard score for users on a channel.",
			[]string{"username", "user_name", "user_id", "rank"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelBitsLeaderboardCollector) Update(ch chan<- prometheus.Metric) error {
	// GetUsers with nil logins returns the authenticated user
	authUsers, err := getUsers(c.client, c.logger, nil)
	if err != nil {
		return err
	}

	if len(authUsers) == 0 {
		return ErrNoData
	}

	username := authUsers[0].DisplayName

	// GetBitsLeaderboard returns the leaderboard for the authenticated broadcaster
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
			username,
			entry.UserName,
			entry.UserID,
			strconv.Itoa(entry.Rank),
		)
	}

	return nil
}
