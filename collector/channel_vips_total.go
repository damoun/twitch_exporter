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
			vipsResp, err := c.client.GetChannelVips(&helix.GetChannelVipsParams{
				BroadcasterID: user.ID,
				First:         100,
				After:         cursor,
			})

			if err != nil {
				c.logger.Error("Failed to collect VIPs from Twitch helix API", "err", err)
				return err
			}

			if vipsResp.StatusCode != 200 {
				c.logger.Error("Failed to collect VIPs from Twitch helix API", "err", vipsResp.ErrorMessage)
				return errors.New(vipsResp.ErrorMessage)
			}

			total += len(vipsResp.Data.ChannelsVips)

			if vipsResp.Data.Pagination.Cursor == "" {
				break
			}
			cursor = vipsResp.Data.Pagination.Cursor
		}

		ch <- c.channelVipsTotal.mustNewConstMetric(float64(total), user.DisplayName)
	}

	return nil
}
