package collector

import (
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelViewersCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelViewers typedDesc
}

func init() {
	registerCollector("channel_viewers_total", defaultEnabled, NewChannelViewersTotalCollector)
}

func NewChannelViewersTotalCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelViewersCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelViewers: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_viewers"),
			"How many viewers on this live channel. If stream is offline then this is absent.",
			[]string{"username", "game"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelViewersCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	streamsResp, err := c.client.GetStreams(&helix.StreamsParams{
		UserLogins: c.channelNames,
		First:      len(c.channelNames),
	})

	if err != nil {
		c.logger.Error("could not get streams", "err", err)
		return err
	}

	for _, s := range streamsResp.Data.Streams {
		ch <- c.channelViewers.mustNewConstMetric(float64(s.ViewerCount), s.UserName, s.GameName)
	}

	return nil
}
