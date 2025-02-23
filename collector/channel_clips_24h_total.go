package collector

import (
	"errors"
	"log/slog"
	"time"

	"github.com/damoun/twitch_exporter/config"
	"github.com/damoun/twitch_exporter/twitch"

	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelClips24HTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames *config.ChannelNames

	metric typedDesc
}

func init() {
	registerCollector("channel_clips_24h_total", defaultEnabled, NewChannelClips24HTotalCollector)
}

func NewChannelClips24HTotalCollector(logger *slog.Logger, client *helix.Client, cfg *config.Config) (Collector, error) {
	c := channelClips24HTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: cfg.Twitch.Channels,

		metric: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_clips_total"),
			"Total number of clips created within the last 24h for a channel",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelClips24HTotalCollector) Update(ch chan<- prometheus.Metric) error {
	if len(*c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := twitch.GetUsersByUsername(c.logger, c.client, *c.channelNames)
	if err != nil {
		err = errors.Join(errors.New("failed to get user by username for channel_clips_total"), err)
		return err
	}

	for _, user := range *users {
		clipsCount, err := c.getClipsCount(user.ID, "", 0)
		if err != nil {
			c.logger.Error("could not get clips count", "err", err.Error(), "user", user.DisplayName)
			continue
		}

		ch <- c.metric.mustNewConstMetric(float64(clipsCount), user.DisplayName)
	}

	return nil
}

func (c channelClips24HTotalCollector) getClipsCount(id string, cursor string, count int) (int, error) {
	// we specify the last 24h to avoid an API limitation
	start := time.Now().Add(-24 * time.Hour).Truncate(24 * time.Hour)
	clipsResp, err := c.client.GetClips(&helix.ClipsParams{
		BroadcasterID: id,
		First:         100,
		After:         cursor,
		StartedAt:     helix.Time{Time: start},
	})

	if err != nil {
		c.logger.Error("failed to collect users stats from Twitch helix API", "err", err.Error())
		return 0, err
	}

	if clipsResp.StatusCode != 200 {
		c.logger.Error("failed to collect users stats from Twitch helix API", "err", clipsResp.ErrorMessage)
		return 0, err
	}

	count += len(clipsResp.Data.Clips)

	if clipsResp.Data.Pagination.Cursor != "" {
		return c.getClipsCount(id, clipsResp.Data.Pagination.Cursor, count)
	}

	return count, nil
}
