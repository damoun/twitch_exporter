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
		var total int
		cursor := ""

		for {
			bansResp, err := c.client.GetBannedUsers(&helix.BannedUsersParams{
				BroadcasterID: user.ID,
				After:         cursor,
			})

			if err != nil {
				c.logger.Error("Failed to collect banned users from Twitch helix API", "err", err)
				return err
			}

			if bansResp.StatusCode != 200 {
				c.logger.Error("Failed to collect banned users from Twitch helix API", "err", bansResp.ErrorMessage)
				return errors.New(bansResp.ErrorMessage)
			}

			total += len(bansResp.Data.Bans)

			if bansResp.Data.Pagination.Cursor == "" {
				break
			}
			cursor = bansResp.Data.Pagination.Cursor
		}

		ch <- c.channelBannedUsersTotal.mustNewConstMetric(float64(total), user.DisplayName)
	}

	return nil
}
