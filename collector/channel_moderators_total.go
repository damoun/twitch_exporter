package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelModeratorsTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelModeratorsTotal typedDesc
}

func init() {
	registerCollector("channel_moderators_total", defaultDisabled, NewChannelModeratorsTotalCollector)
}

func NewChannelModeratorsTotalCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelModeratorsTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelModeratorsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_moderators_total"),
			"The number of moderators of a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelModeratorsTotalCollector) Update(ch chan<- prometheus.Metric) error {
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
			moderatorsResp, err := c.client.GetModerators(&helix.GetModeratorsParams{
				BroadcasterID: user.ID,
				First:         100,
				After:         cursor,
			})

			if err != nil {
				c.logger.Error("Failed to collect moderators from Twitch helix API", "err", err)
				return err
			}

			if moderatorsResp.StatusCode != 200 {
				c.logger.Error("Failed to collect moderators from Twitch helix API", "err", moderatorsResp.ErrorMessage)
				return errors.New(moderatorsResp.ErrorMessage)
			}

			total += len(moderatorsResp.Data.Moderators)

			if moderatorsResp.Data.Pagination.Cursor == "" {
				break
			}
			cursor = moderatorsResp.Data.Pagination.Cursor
		}

		ch <- c.channelModeratorsTotal.mustNewConstMetric(float64(total), user.DisplayName)
	}

	return nil
}
