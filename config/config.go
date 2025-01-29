package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	yaml "gopkg.in/yaml.v3"
)

var (
	twitchClientID    = kingpin.Flag("twitch.client-id", "Client ID for the Twitch Helix API.").Default("").String()
	twitchAccessToken = kingpin.Flag("twitch.access-token", "Access Token for the Twitch Helix API.").Default("").String()
	twitchChannel     = kingpin.Flag("twitch.channel", "Name of a Twitch Channel to request metrics.").Default("").Strings()

	ErrMissingClientID    = errors.New("missing client id config")
	ErrMissingAccessToken = errors.New("missing access token config")
)

// twitch_exporter config
type Config struct {
	// twitch configuration
	Twitch twitch `yaml:"twitch"`
}

type twitch struct {
	// twitch client id
	ClientID string `yaml:"client-id"`
	// twitch client secret, can be either an access token or app token.
	// available collectors will depend on type of token used
	AccessToken string `yaml:"access-token"`
	// list of channels to monitor
	Channels *ChannelNames `yaml:"channels"`
}

// safeconfig is used as a wrapper around config to enable reload of config
// based on changes or SIGHUP for example
type SafeConfig struct {
	sync.RWMutex

	C                   *Config
	configReloadSuccess prometheus.Gauge
	configReloadSeconds prometheus.Gauge
}

// ChannelNames represents a list of twitch channels.
type ChannelNames []string

// IsCumulative is required for kingpin interfaces to allow multiple values
func (c ChannelNames) IsCumulative() bool {
	return true
}

// Set sets the value of a ChannelNames
func (c *ChannelNames) Set(v string) error {
	*c = append(*c, v)
	return nil
}

// String returns a string representation of the Channels type.
func (c ChannelNames) String() string {
	return fmt.Sprintf("%v", []string(c))
}

// Channels creates a collection of Channels from a kingpin command line argument.
func Channels(s *[]string) (target *ChannelNames) {
	target = &ChannelNames{}

	for _, c := range *s {
		target.Set(c)
	}

	return target
}

func NewSafeConfig(reg prometheus.Registerer) *SafeConfig {
	configReloadSuccess := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Namespace: "blackbox_exporter",
		Name:      "config_last_reload_successful",
		Help:      "Blackbox exporter config loaded successfully.",
	})

	configReloadSeconds := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Namespace: "blackbox_exporter",
		Name:      "config_last_reload_success_timestamp_seconds",
		Help:      "Timestamp of the last successful configuration reload.",
	})
	return &SafeConfig{C: &Config{}, configReloadSuccess: configReloadSuccess, configReloadSeconds: configReloadSeconds}
}

func (sc *SafeConfig) ReloadConfig(confFile string, logger *slog.Logger) (err error) {
	var c = &Config{}
	defer func() {
		if err != nil {
			sc.configReloadSuccess.Set(0)
		} else {
			sc.configReloadSuccess.Set(1)
			sc.configReloadSeconds.SetToCurrentTime()
		}
	}()

	yamlReader, err := os.Open(confFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %s", err)
	}
	defer yamlReader.Close()
	decoder := yaml.NewDecoder(yamlReader)
	decoder.KnownFields(true)

	if err = decoder.Decode(c); err != nil {
		return fmt.Errorf("error parsing config file: %s", err)
	}

	// if flags then override config
	if twitchClientID != nil && *twitchClientID != "" {
		c.Twitch.ClientID = *twitchClientID
	}

	if twitchAccessToken != nil && *twitchAccessToken != "" {
		c.Twitch.AccessToken = *twitchAccessToken
	}

	if twitchChannel != nil && len(*twitchChannel) > 0 {
		c.Twitch.Channels = Channels(twitchChannel)
	}

	if c.Twitch.ClientID == "" {
		return ErrMissingClientID
	}

	if c.Twitch.AccessToken == "" {
		return ErrMissingAccessToken
	}

	if len(*c.Twitch.Channels) == 0 {
		logger.Warn("no channels defined in params or config")
	}

	sc.Lock()
	sc.C = c
	sc.Unlock()

	return nil
}
