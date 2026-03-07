package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelBannedUsersTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelBannedUsersTotal typedDesc
}

func init() {
	registerCollector("channel_banned_users_total", defaultDisabled, NewChannelBannedUsersTotalCollector)
}

func NewChannelBannedUsersTotalCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelBannedUsersTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelBannedUsersTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_banned_users_total"),
			"The number of banned users of a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelBannedUsersTotalCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := getUsers(c.client, c.logger, c.channelNames)
	if err != nil {
		return err
	}

	for _, user := range users {
		total, err := countPaginated(func(cursor string) (int, string, error) {
			resp, err := c.client.GetBannedUsers(&helix.BannedUsersParams{
				BroadcasterID: user.ID,
				After:         cursor,
			})
			if err != nil {
				c.logger.Error("Failed to collect banned users from Twitch helix API", "err", err)
				return 0, "", err
			}
			if resp.StatusCode != 200 {
				c.logger.Error("Failed to collect banned users from Twitch helix API", "err", resp.ErrorMessage)
				return 0, "", errors.New(resp.ErrorMessage)
			}
			return len(resp.Data.Bans), resp.Data.Pagination.Cursor, nil
		})
		if err != nil {
			return err
		}

		ch <- c.channelBannedUsersTotal.mustNewConstMetric(float64(total), user.DisplayName)
	}

	return nil
}
