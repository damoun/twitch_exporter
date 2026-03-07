package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelInfoCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelInfo         typedDesc
	channelDelaySeconds typedDesc
}

func init() {
	registerCollector("channel_info", defaultEnabled, NewChannelInfoCollector)
}

func NewChannelInfoCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelInfoCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelInfo: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_info"),
			"Channel metadata including game, title and language.",
			[]string{"username", "game", "title", "language"}, nil,
		), prometheus.GaugeValue},

		channelDelaySeconds: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_delay_seconds"),
			"The stream delay in seconds for a channel.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelInfoCollector) Update(ch chan<- prometheus.Metric) error {
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

	broadcasterIDs := make([]string, 0, len(usersResp.Data.Users))
	usersByID := make(map[string]string, len(usersResp.Data.Users))
	for _, user := range usersResp.Data.Users {
		broadcasterIDs = append(broadcasterIDs, user.ID)
		usersByID[user.ID] = user.DisplayName
	}

	channelResp, err := c.client.GetChannelInformation(&helix.GetChannelInformationParams{
		BroadcasterIDs: broadcasterIDs,
	})

	if err != nil {
		c.logger.Error("Failed to collect channel information from Twitch helix API", "err", err)
		return err
	}

	if channelResp.StatusCode != 200 {
		c.logger.Error("Failed to collect channel information from Twitch helix API", "err", channelResp.ErrorMessage)
		return errors.New(channelResp.ErrorMessage)
	}

	for _, channel := range channelResp.Data.Channels {
		username := usersByID[channel.BroadcasterID]
		ch <- c.channelInfo.mustNewConstMetric(1, username, channel.GameName, channel.Title, channel.BroadcasterLanguage)
		ch <- c.channelDelaySeconds.mustNewConstMetric(float64(channel.Delay), username)
	}

	return nil
}
