package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelClipsTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelClips typedDesc
}

func init() {
	registerCollector("channel_clips_total", defaultEnabled, NewChannelClipsTotalCollector)
}

func NewChannelClipsTotalCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelClipsTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelClips: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_clips_total"),
			"The number of clips of a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelClipsTotalCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := getUsers(c.client, c.logger, c.channelNames)
	if err != nil {
		return err
	}

	for _, user := range users {
		total, err := c.countClips(user.ID)
		if err != nil {
			c.logger.Error("Failed to collect clips stats from Twitch helix API", "err", err)
			return err
		}

		ch <- c.channelClips.mustNewConstMetric(float64(total), user.DisplayName)
	}

	return nil
}

func (c channelClipsTotalCollector) countClips(broadcasterID string) (int, error) {
	total := 0
	cursor := ""

	for {
		clipsResp, err := c.client.GetClips(&helix.ClipsParams{
			BroadcasterID: broadcasterID,
			First:         100,
			After:         cursor,
		})

		if err != nil {
			return 0, err
		}

		if clipsResp.StatusCode != 200 {
			return 0, errors.New(clipsResp.ErrorMessage)
		}

		total += len(clipsResp.Data.Clips)

		if clipsResp.Data.Pagination.Cursor == "" {
			break
		}
		cursor = clipsResp.Data.Pagination.Cursor
	}

	return total, nil
}
