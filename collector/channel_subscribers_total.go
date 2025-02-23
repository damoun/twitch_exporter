package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/config"
	"github.com/damoun/twitch_exporter/twitch"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	giftedSub    = "true"
	notGiftedSub = "false"
)

type ChannelSubscriberTotalCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames *config.ChannelNames

	channelSubscribersTotal typedDesc
}

func init() {
	registerCollector("channel_subscribers_total", defaultDisabled, NewChannelSubscriberTotalCollector)
}

func NewChannelSubscriberTotalCollector(logger *slog.Logger, client *helix.Client, cfg *config.Config) (Collector, error) {
	c := ChannelSubscriberTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: cfg.Twitch.Channels,

		channelSubscribersTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_subscribers_total"),
			"The number of subscriber of a channel.",
			[]string{"username", "tier", "gifted"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c ChannelSubscriberTotalCollector) Update(ch chan<- prometheus.Metric) error {
	if len(*c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := twitch.GetUsersByUsername(c.logger, c.client, *c.channelNames)
	if err != nil {
		err = errors.Join(errors.New("failed to get users by username for channel_subscribers_total"), err)
		return err
	}

	for _, user := range *users {
		subscribtionsResp, err := c.client.GetSubscriptions(&helix.SubscriptionsParams{
			BroadcasterID: user.ID,
		})

		if err != nil {
			c.logger.Error("Failed to collect subscribers stats from Twitch helix API", "err", err.Error())
			return errors.Join(errors.New("failed to collect subscribers stats from Twitch helix API"), err)
		}

		if subscribtionsResp.StatusCode != 200 {
			c.logger.Error("Failed to collect subscirbers stats from Twitch helix API", "err", subscribtionsResp.ErrorMessage)
			return errors.Join(errors.New("failed to collect subscirbers stats from Twitch helix API"), errors.New(subscribtionsResp.ErrorMessage))
		}

		subCounter := make(map[string]int)
		giftedSubCounter := make(map[string]int)

		for _, subscription := range subscribtionsResp.Data.Subscriptions {
			if subscription.IsGift {
				if _, ok := giftedSubCounter[subscription.Tier]; !ok {
					giftedSubCounter[subscription.Tier] = 0
				}
				giftedSubCounter[subscription.Tier] = giftedSubCounter[subscription.Tier] + 1
			} else {
				if _, ok := subCounter[subscription.Tier]; !ok {
					subCounter[subscription.Tier] = 0
				}
				subCounter[subscription.Tier] = subCounter[subscription.Tier] + 1
			}
		}

		for tier, counter := range giftedSubCounter {
			ch <- c.channelSubscribersTotal.mustNewConstMetric(float64(counter), user.DisplayName, tier, giftedSub)
		}

		for tier, counter := range subCounter {
			ch <- c.channelSubscribersTotal.mustNewConstMetric(float64(counter), user.DisplayName, tier, notGiftedSub)
		}
	}

	return nil
}
