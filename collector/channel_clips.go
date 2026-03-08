package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelClipsCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelClips typedDesc
}

func init() {
	registerCollector("channel_clips_total", defaultEnabled, NewChannelClipsTotalCollector)
}

func NewChannelClipsTotalCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelClipsCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelClips: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_clips"),
			"The number of clips of a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelClipsCollector) Update(ch chan<- prometheus.Metric) error {
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

func (c channelClipsCollector) countClips(broadcasterID string) (int, error) {
	return countPaginated(func(cursor string) (int, string, error) {
		resp, err := c.client.GetClips(&helix.ClipsParams{
			BroadcasterID: broadcasterID,
			First:         100,
			After:         cursor,
		})
		if err != nil {
			return 0, "", err
		}
		if resp.StatusCode != 200 {
			return 0, "", errors.New(resp.ErrorMessage)
		}
		return len(resp.Data.Clips), resp.Data.Pagination.Cursor, nil
	})
}
