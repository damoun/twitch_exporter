package collector

import (
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelViewersTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelViewersTotal typedDesc
}

func init() {
	registerCollector("channel_viewers_total", defaultEnabled, AuthApp, NewChannelViewersTotalCollector)
}

func NewChannelViewersTotalCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelViewersTotalCollector{
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

func (c channelViewersTotalCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	// GetStreams returns only channels that are currently live, batched into
	// grouped requests of at most maxHelixIDsPerRequest logins.
	for _, chunk := range chunkStrings(c.channelNames, maxHelixIDsPerRequest) {
		streamsResp, err := c.client.GetStreams(&helix.StreamsParams{
			UserLogins: chunk,
			First:      len(chunk),
		})

		if err != nil {
			c.logger.Error("could not get streams", "err", err)
			return err
		}

		for _, s := range streamsResp.Data.Streams {
			ch <- c.channelViewersTotal.mustNewConstMetric(float64(s.ViewerCount), s.UserLogin, s.GameName)
		}
	}

	return nil
}
