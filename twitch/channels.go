package twitch

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/damoun/twitch_exporter/cache"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/nicklaw5/helix/v2"
)

var (
	ErrNotFound = store.NotFound{}
)

// GetChannel
// caches the user in memory for 24hrs, to help reduce unnecessary API calls
func GetUsersByUsername(logger *slog.Logger, c *helix.Client, usernames []string) (*[]helix.User, error) {
	ctx := context.Background()

	cacheKey := buildCacheKey("channel", "username", base64.StdEncoding.EncodeToString([]byte(strings.Join(usernames, "-"))))
	slog.Info("checking cache for user", "key", cacheKey)

	// check if we already have it in cache
	data, err := cache.DefaultCache.Get(ctx, cacheKey)
	if err != nil && !errors.Is(err, ErrNotFound) {
		err = errors.Join(errors.New("could not retrieve user by username from cache"), err)
		return nil, err
	}

	// if we do then unmarshal that and return it
	if len(data) > 0 {
		usrs := &[]helix.User{}
		json.Unmarshal(data, &usrs)
		return usrs, nil
	}

	// we need to get a fresh api call
	usersResp, err := c.GetUsers(&helix.UsersParams{
		Logins: usernames,
	})

	if err != nil {
		return nil, errors.Join(errors.New("failed to collect users stats from Twitch helix API"), err)
	}

	if usersResp.StatusCode != 200 {
		return nil, errors.Join(errors.New("failed to get user by id from Twitch helix API"), errors.New(usersResp.ErrorMessage))
	}

	// dont cache empty responses
	if len(usersResp.Data.Users) == 0 {
		return nil, nil
	}

	usrs := usersResp.Data.Users
	cacheData, err := json.Marshal(usrs)
	if err != nil {
		// warn since we want to express something went wrong, but we don't want to prevent
		// a response due to cache being unavailable
		logger.Warn("could not marshal user response for cache", "err", err.Error())
	} else {
		err = cache.DefaultCache.Set(ctx, cacheKey, cacheData)
		if err != nil {
			logger.Warn("could not cache user response", "err", err.Error())
		}
	}

	return &usrs, nil
}

func buildCacheKey(parts ...string) string {
	// redefine the parts, but with our own prefix
	parts = append([]string{
		"twitch_exporter",
	}, parts...)

	return strings.Join(parts, ":")
}
