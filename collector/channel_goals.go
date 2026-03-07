package collector

import (
	"errors"
	"log/slog"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type channelGoalsCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	goalCurrent typedDesc
	goalTarget  typedDesc
}

func init() {
	registerCollector("channel_goals", defaultDisabled, NewChannelGoalsCollector)
}

func NewChannelGoalsCollector(logger *slog.Logger, client *helix.Client, _ *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	c := channelGoalsCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		goalCurrent: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_goal_current"),
			"The current amount for a creator goal in a channel.",
			[]string{"username", "type"}, nil,
		), prometheus.GaugeValue},

		goalTarget: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_goal_target"),
			"The target amount for a creator goal in a channel.",
			[]string{"username", "type"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelGoalsCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	users, err := getUsers(c.client, c.logger, c.channelNames)
	if err != nil {
		return err
	}

	for _, user := range users {
		goalsResp, err := c.client.GetCreatorGoals(&helix.GetCreatorGoalsParams{
			BroadcasterID: user.ID,
		})

		if err != nil {
			c.logger.Error("Failed to collect creator goals from Twitch helix API", "err", err)
			return err
		}

		if goalsResp.StatusCode != 200 {
			c.logger.Error("Failed to collect creator goals from Twitch helix API", "err", goalsResp.ErrorMessage)
			return errors.New(goalsResp.ErrorMessage)
		}

		for _, goal := range goalsResp.Data.Goals {
			ch <- c.goalCurrent.mustNewConstMetric(float64(goal.CurrentAmount), user.DisplayName, goal.Type)
			ch <- c.goalTarget.mustNewConstMetric(float64(goal.TargetAmount), user.DisplayName, goal.Type)
		}
	}

	return nil
}
