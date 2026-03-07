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

	users, err := getUsers(c.client, c.logger, c.channelNames)
	if err != nil {
		return err
	}

	for _, user := range users {
		total, err := countPaginated(func(cursor string) (int, string, error) {
			resp, err := c.client.GetModerators(&helix.GetModeratorsParams{
				BroadcasterID: user.ID,
				First:         100,
				After:         cursor,
			})
			if err != nil {
				c.logger.Error("Failed to collect moderators from Twitch helix API", "err", err)
				return 0, "", err
			}
			if resp.StatusCode != 200 {
				c.logger.Error("Failed to collect moderators from Twitch helix API", "err", resp.ErrorMessage)
				return 0, "", errors.New(resp.ErrorMessage)
			}
			return len(resp.Data.Moderators), resp.Data.Pagination.Cursor, nil
		})
		if err != nil {
			return err
		}

		ch <- c.channelModeratorsTotal.mustNewConstMetric(float64(total), user.DisplayName)
	}

	return nil
}
