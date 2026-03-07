package collector

import (
	"errors"
	"log/slog"
	"math"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelCharityCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	charityCurrentAmount typedDesc
	charityTargetAmount  typedDesc
}

func init() {
	registerCollector("channel_charity", defaultDisabled, NewChannelCharityCollector)
}

func NewChannelCharityCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelCharityCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		charityCurrentAmount: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_charity_current_amount"),
			"The current amount raised for the charity campaign in a channel.",
			[]string{"username", "currency"}, nil,
		), prometheus.GaugeValue},

		charityTargetAmount: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_charity_target_amount"),
			"The target amount for the charity campaign in a channel.",
			[]string{"username", "currency"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelCharityCollector) Update(ch chan<- prometheus.Metric) error {
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

	for _, user := range usersResp.Data.Users {
		charityResp, err := c.client.GetCharityCampaigns(&helix.CharityCampaignsParams{
			BroadcasterID: user.ID,
		})

		if err != nil {
			c.logger.Error("Failed to collect charity campaigns from Twitch helix API", "err", err)
			return err
		}

		if charityResp.StatusCode != 200 {
			c.logger.Error("Failed to collect charity campaigns from Twitch helix API", "err", charityResp.ErrorMessage)
			return errors.New(charityResp.ErrorMessage)
		}

		if len(charityResp.Data.Campaigns) == 0 {
			ch <- c.charityCurrentAmount.mustNewConstMetric(0, user.DisplayName, "")
			ch <- c.charityTargetAmount.mustNewConstMetric(0, user.DisplayName, "")
			continue
		}

		campaign := charityResp.Data.Campaigns[0]
		currentValue := float64(campaign.CurrentAmount.Value) / math.Pow(10, float64(campaign.CurrentAmount.DecimalPlaces))
		targetValue := float64(campaign.TargetAmount.Value) / math.Pow(10, float64(campaign.TargetAmount.DecimalPlaces))
		ch <- c.charityCurrentAmount.mustNewConstMetric(currentValue, user.DisplayName, campaign.CurrentAmount.Currency)
		ch <- c.charityTargetAmount.mustNewConstMetric(targetValue, user.DisplayName, campaign.TargetAmount.Currency)
	}

	return nil
}
