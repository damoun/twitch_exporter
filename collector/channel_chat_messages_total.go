package collector

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	chatMessages      = MessageCounter{}
	chatMessagesMutex = sync.Mutex{}
)

type MessageCounter map[string]map[string]int

func (m MessageCounter) Add(username string, chatterUsername string) {
	m.ensure(username, chatterUsername)

	chatMessagesMutex.Lock()
	defer chatMessagesMutex.Unlock()

	chatMessages[username][chatterUsername]++
}

func (m MessageCounter) Reset(username string, chatterUsername string) {
	m.ensure(username, chatterUsername)

	chatMessagesMutex.Lock()
	defer chatMessagesMutex.Unlock()

	chatMessages[username][chatterUsername] = 0
}

// ensure ensures that the username and chatterUsername exist in the map
func (m MessageCounter) ensure(username string, chatterUsername string) {
	if _, ok := chatMessages[username]; !ok {
		chatMessages[username] = make(map[string]int)
	}

	if _, ok := chatMessages[username][chatterUsername]; !ok {
		chatMessages[username][chatterUsername] = 0
	}
}

func (m MessageCounter) Get(username string, chatterUsername string) int {
	m.ensure(username, chatterUsername)

	chatMessagesMutex.Lock()
	defer chatMessagesMutex.Unlock()

	return chatMessages[username][chatterUsername]
}

type ChannelChatMessagesCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	channelNames ChannelNames

	channelChatMessages typedDesc
}

func init() {
	// disabled by default since you need to use webhooks to listen for events using an app access token
	// which requires it to be exposed to the internet
	registerCollector("channel_chat_messages_total", defaultDisabled, NewChannelChatMessagesCollector)
}

func NewChannelChatMessagesCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, channelNames ChannelNames) (Collector, error) {
	// this means that eventsub.enabled must be true, otherwise the default client will not be set
	if eventsubClient == nil {
		return nil, eventsub.ErrEventsubClientNotSet
	}

	broadcasterIDs := []string{}
	users, err := client.GetUsers(&helix.UsersParams{
		Logins: channelNames,
	})
	if err != nil {
		return nil, err
	}

	for _, user := range users.Data.Users {
		broadcasterIDs = append(broadcasterIDs, user.ID)
	}

	err = eventsubClient.On("channel.chat.message", func(eventRaw json.RawMessage) {
		var event eventsub.ChannelChatMessageEvent

		if err := json.Unmarshal(eventRaw, &event); err != nil {
			logger.Error("failed to unmarshal channel chat message event", "error", err)
			return
		}

		chatMessages.Add(event.BroadcasterUserLogin, event.ChatterUserLogin)

		logger.Info(
			"channel chat message",
			"count", chatMessages.Get(event.BroadcasterUserLogin, event.ChatterUserLogin),
		)
	})

	// todo: we can only subscribe to broadcasters with an access token and refresh token, so this
	// would generally just be a single user, the broadcaster
	for _, broadcasterID := range broadcasterIDs {
		err = eventsubClient.Subscribe("channel.chat.message", broadcasterID)
		if err != nil {
			logger.Error("failed to subscribe to channel chat messages", "error", err)
		}
	}

	// in theory the only error this could be is ErrEventsubDefaultClientNotSet which is already handled
	// but it returns an error in case that expands
	if err != nil {
		return nil, err
	}

	c := ChannelChatMessagesCollector{
		logger:       logger,
		client:       client,
		channelNames: channelNames,

		// we keep the use of the username as the label to avoid adding a bunch of duplicate labels under
		// a new name of broadcaster_username, which would just match with the other metrics using username
		// however to group by the chatters we provide chatter_username as a label.
		// this metric would increase label cardinality a lot for larger channels, so it should be used with
		// care and ideally only on a small subset of channels.
		channelChatMessages: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_chat_messages_total"),
			"The number of chat messages sent in a channel.",
			[]string{"username", "chatter_username"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c ChannelChatMessagesCollector) Update(ch chan<- prometheus.Metric) error {
	if len(c.channelNames) == 0 {
		return ErrNoData
	}

	// loop all the channels and push the counts
	for username, count := range chatMessages {
		for chatterUsername, count := range count {
			ch <- prometheus.MustNewConstMetric(c.channelChatMessages.desc, prometheus.CounterValue, float64(count), username, chatterUsername)
		}
	}

	return nil
}
