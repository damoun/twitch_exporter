package collector

import (
	"log/slog"

	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type ChannelViewersTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelViewersTotal typedDesc
}

func init() {
	registerCollector("channel_viewers_total", defaultEnabled, NewChannelViewersTotalCollector)
}

func NewChannelViewersTotalCollector(logger *slog.Logger, client *helix.Client, channelNames ChannelNames) (Collector, error) {
	c := ChannelViewersTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelViewersTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_viewers_total"),
			"How many viewers on this live channel. If stream is offline then this is absent.",
			[]string{"username", "game"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c ChannelViewersTotalCollector) Update(ch chan<- prometheus.Metric) error {
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
		ch <- c.channelViewersTotal.mustNewConstMetric(float64(s.ViewerCount), s.UserName, s.GameName)
	}

	return nil
}
