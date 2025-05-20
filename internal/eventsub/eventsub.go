package eventsub

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/LinneB/twitchwh"
	"github.com/nicklaw5/helix/v2"
)

type ChannelChatMessageEvent struct {
	BroadcasterUserID       string `json:"broadcaster_user_id"`
	BroadcasterUserLogin    string `json:"broadcaster_user_login"`
	BroadcasterUserName     string `json:"broadcaster_user_name"`
	SourceBroadcasterUserID string `json:"source_broadcaster_user_id"`
	SourceBroadcasterLogin  string `json:"source_broadcaster_user_login"`
	SourceBroadcasterName   string `json:"source_broadcaster_user_name"`
	ChatterUserID           string `json:"chatter_user_id"`
	ChatterUserLogin        string `json:"chatter_user_login"`
	ChatterUserName         string `json:"chatter_user_name"`
	MessageID               string `json:"message_id"`
	SourceMessageID         string `json:"source_message_id"`
	IsSourceOnly            bool   `json:"is_source_only"`
	Message                 struct {
		Text      string `json:"text"`
		Fragments []struct {
			Type      string      `json:"type"`
			Text      string      `json:"text"`
			Cheermote interface{} `json:"cheermote"`
			Emote     interface{} `json:"emote"`
			Mention   interface{} `json:"mention"`
		} `json:"fragments"`
	} `json:"message"`
	Color                       string  `json:"color"`
	Badges                      []Badge `json:"badges"`
	SourceBadges                []Badge `json:"source_badges"`
	MessageType                 string  `json:"message_type"`
	Cheer                       string  `json:"cheer"`
	Reply                       string  `json:"reply"`
	ChannelPointsCustomRewardID string  `json:"channel_points_custom_reward_id"`
	ChannelPointsAnimationID    string  `json:"channel_points_animation_id"`
}

type Badge struct {
	SetID string `json:"set_id"`
	ID    string `json:"id"`
	Info  string `json:"info"`
}

var ErrEventsubClientNotSet = errors.New("eventsub client not set")

type Client struct {
	webhookURL    string
	webhookSecret string

	appClient *helix.Client
	logger    *slog.Logger
	cl        *twitchwh.Client
}

func New(
	clientID, clientSecret, webhookURL, webhookSecret string,
	logger *slog.Logger,
	appClient *helix.Client,
) (*Client, error) {
	eventsubCl := &Client{
		appClient:     appClient,
		logger:        logger,
		webhookURL:    webhookURL,
		webhookSecret: webhookSecret,
	}

	cl, err := twitchwh.New(twitchwh.ClientConfig{
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		WebhookURL:    webhookURL,
		WebhookSecret: webhookSecret,
		Debug:         true,
	})

	if err != nil {
		return nil, err
	}

	eventsubCl.cl = cl

	return eventsubCl, nil
}

func (c *Client) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.logger.Debug("received event", "body", r.Body, "headers", r.Header)
		c.cl.Handler(w, r)
		c.logger.Debug("event handled", "headers", w.Header())
	}
}

func (c *Client) On(event string, callback func(eventRaw json.RawMessage)) error {
	// juuust in case
	if c.cl == nil {
		c.logger.Warn("eventsub client not set")
		return ErrEventsubClientNotSet
	}

	c.cl.On(event, callback)
	return nil
}

func (c *Client) Subscribe(eventType string, broadcasterID string) error {
	if c.cl == nil {
		c.logger.Warn("eventsub client not set")
		return ErrEventsubClientNotSet
	}

	c.logger.Info("subscribing to event", "event", eventType, "broadcaster_id", broadcasterID)

	// cannot filter by both the user id and the event type, so the better option is to get all the user
	// subscriptions and see if the event type is found already
	subscriptions, err := c.appClient.GetEventSubSubscriptions(&helix.EventSubSubscriptionsParams{
		UserID: broadcasterID,
	})

	if err != nil {
		return err
	}

	for _, v := range subscriptions.Data.EventSubSubscriptions {
		if v.Type == eventType && (v.Status == "enabled" || v.Status == "webhook_callback_verification_pending") {
			c.logger.Info("subscription already exists", "event", eventType, "broadcaster_id", broadcasterID)
			return nil
		}
	}

	res, err := c.appClient.CreateEventSubSubscription(&helix.EventSubSubscription{
		Type:    eventType,
		Version: "1",
		// the bot user and the broadcaster user are the same, assuming that the access token is for the broadcaster
		Condition: helix.EventSubCondition{
			UserID:            broadcasterID,
			BroadcasterUserID: broadcasterID,
		},
		Transport: helix.EventSubTransport{
			Method:   "webhook",
			Callback: c.webhookURL,
			Secret:   c.webhookSecret,
		},
	})

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusAccepted {
		c.logger.Info("failed to create subscription", "res", res)
		return errors.Join(errors.New("failed to create subscription"), errors.New(res.ErrorMessage))
	}

	c.logger.Info("subscription created", "error", res.Error, "status_code", res.StatusCode, "data", res.Data)

	// c.logger.Info("subscription created", "event", eventType, "broadcaster_id", broadcasterID, "subscription_id", res.Data.EventSubSubscriptions[0].ID)

	return nil
}
