package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelVipsTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelVipsTotal typedDesc
}

func init() {
	registerCollector("channel_vips_total", defaultDisabled, NewChannelVipsTotalCollector)
}

func NewChannelVipsTotalCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelVipsTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelVipsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_vips_total"),
			"The number of VIPs of a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelVipsTotalCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := getUsers(c.client, c.logger, c.channelNames)
	if err != nil {
		return err
	}

	for _, user := range users {
		total, err := countPaginated(func(cursor string) (int, string, error) {
			resp, err := c.client.GetChannelVips(&helix.GetChannelVipsParams{
				BroadcasterID: user.ID,
				First:         100,
				After:         cursor,
			})
			if err != nil {
				c.logger.Error("Failed to collect VIPs from Twitch helix API", "err", err)
				return 0, "", err
			}
			if resp.StatusCode != 200 {
				c.logger.Error("Failed to collect VIPs from Twitch helix API", "err", resp.ErrorMessage)
				return 0, "", errors.New(resp.ErrorMessage)
			}
			return len(resp.Data.ChannelsVips), resp.Data.Pagination.Cursor, nil
		})
		if err != nil {
			return err
		}

		ch <- c.channelVipsTotal.mustNewConstMetric(float64(total), user.DisplayName)
	}

	return nil
}
