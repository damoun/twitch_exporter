package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelChatSettingsCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	chatEmoteOnly           typedDesc
	chatFollowersOnly       typedDesc
	chatSubscriberOnly      typedDesc
	chatSlowMode            typedDesc
	chatSlowModeWaitSeconds typedDesc
}

func init() {
	registerCollector("channel_chat_settings", defaultEnabled, NewChannelChatSettingsCollector)
}

func NewChannelChatSettingsCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelChatSettingsCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		chatEmoteOnly: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_chat_emote_only"),
			"Whether emote-only mode is enabled in a channel's chat.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},

		chatFollowersOnly: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_chat_followers_only"),
			"Whether followers-only mode is enabled in a channel's chat.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},

		chatSubscriberOnly: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_chat_subscriber_only"),
			"Whether subscriber-only mode is enabled in a channel's chat.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},

		chatSlowMode: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_chat_slow_mode"),
			"Whether slow mode is enabled in a channel's chat.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},

		chatSlowModeWaitSeconds: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_chat_slow_mode_wait_seconds"),
			"The slow mode wait time in seconds for a channel's chat.",
			[]string{"username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (c channelChatSettingsCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := getUsers(c.client, c.logger, c.channelNames)
	if err != nil {
		return err
	}

	for _, user := range users {
		settingsResp, err := c.client.GetChatSettings(&helix.GetChatSettingsParams{
			BroadcasterID: user.ID,
		})

		if err != nil {
			c.logger.Error("Failed to collect chat settings from Twitch helix API", "err", err)
			return err
		}

		if settingsResp.StatusCode != 200 {
			c.logger.Error("Failed to collect chat settings from Twitch helix API", "err", settingsResp.ErrorMessage)
			return errors.New(settingsResp.ErrorMessage)
		}

		if len(settingsResp.Data.Settings) == 0 {
			continue
		}

		s := settingsResp.Data.Settings[0]
		ch <- c.chatEmoteOnly.mustNewConstMetric(boolToFloat64(s.EmoteMode), user.DisplayName)
		ch <- c.chatFollowersOnly.mustNewConstMetric(boolToFloat64(s.FollowerMode), user.DisplayName)
		ch <- c.chatSubscriberOnly.mustNewConstMetric(boolToFloat64(s.SubscriberMode), user.DisplayName)
		ch <- c.chatSlowMode.mustNewConstMetric(boolToFloat64(s.SlowMode), user.DisplayName)
		ch <- c.chatSlowModeWaitSeconds.mustNewConstMetric(float64(s.SlowModeWaitTime), user.DisplayName)
	}

	return nil
}
