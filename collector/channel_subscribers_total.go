package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
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
	channelNames ChannelNames

	channelSubscribersTotal typedDesc
}

func init() {
	registerCollector("channel_subscribers_total", defaultDisabled, NewChannelSubscriberTotalCollector)
}

func NewChannelSubscriberTotalCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := ChannelSubscriberTotalCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		channelSubscribersTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_subscribers_total"),
			"The number of subscriber of a channel.",
			[]string{"username", "tier", "gifted"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c ChannelSubscriberTotalCollector) Update(ch chan<- prometheus.Metric) error {
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

	// todo: we can avoid this with a shared cache of username to userID that has a short TTL
	for _, user := range usersResp.Data.Users {
		subscribtionsResp, err := c.client.GetSubscriptions(&helix.SubscriptionsParams{
			BroadcasterID: user.ID,
		})

		if err != nil {
			c.logger.Error("Failed to collect subscribers stats from Twitch helix API", "err", err)
			return err
		}

		if subscribtionsResp.StatusCode != 200 {
			c.logger.Error("Failed to collect subscirbers stats from Twitch helix API", "err", subscribtionsResp.ErrorMessage)
			return errors.New(subscribtionsResp.ErrorMessage)
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
