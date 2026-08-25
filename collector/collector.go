// Much of this file has been referenced from:
// https://github.com/prometheus/node_exporter/blob/master/collector/collector.go

package collector

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/damoun/twitch_exporter/internal/eventsub"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/singleflight"
)

const namespace = "twitch"

// AuthMode describes the level of Twitch credentials a collector requires.
type AuthMode int

const (
	// AuthApp collectors only need an app access token. They query
	// non-privileged, public data and are safe to expose via the /probe
	// endpoint.
	AuthApp AuthMode = iota
	// AuthUser collectors require a user access token (or EventSub) to reach
	// privileged data such as subscribers, moderators or chat events. They
	// cannot be used in probe mode.
	AuthUser
)

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"node_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"node_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

const (
	defaultEnabled  = true
	defaultDisabled = false
)

var (
	factories              = make(map[string]func(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, channelNames ChannelNames) (Collector, error))
	initiatedCollectorsMtx = sync.Mutex{}
	initiatedCollectors    = make(map[string]Collector)
	collectorState         = make(map[string]*bool)
	collectorAuthModes     = make(map[string]AuthMode)
	forcedCollectors       = map[string]bool{} // collectors which have been explicitly enabled or disabled
)

func registerCollector(collector string, isDefaultEnabled bool, authMode AuthMode, factory func(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, channelNames ChannelNames) (Collector, error)) {
	var helpDefaultState string
	if isDefaultEnabled {
		helpDefaultState = "enabled"
	} else {
		helpDefaultState = "disabled"
	}

	flagName := "collector." + collector
	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s).", collector, helpDefaultState)
	defaultValue := fmt.Sprintf("%v", isDefaultEnabled)

	flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Action(collectorFlagAction(collector)).Bool()
	collectorState[collector] = flag
	collectorAuthModes[collector] = authMode

	factories[collector] = factory
}

// ProbeableCollectors returns, sorted alphabetically, the names of the
// collectors that only require an app access token and are therefore safe to
// serve from the /probe endpoint.
func ProbeableCollectors() []string {
	names := make([]string, 0, len(collectorAuthModes))
	for name, mode := range collectorAuthModes {
		if mode == AuthApp {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

type Exporter struct {
	Collectors map[string]Collector
	logger     *slog.Logger
}

// Describe describes all the metrics ever exported by the Twitch exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// collectorFlagAction generates a new action function for the given collector
// to track whether it has been explicitly enabled or disabled from the command line.
// A new action function is needed for each collector flag because the ParseContext
// does not contain information about which flag called the action.
// See: https://github.com/alecthomas/kingpin/issues/294
func collectorFlagAction(collector string) func(ctx *kingpin.ParseContext) error {
	return func(ctx *kingpin.ParseContext) error {
		forcedCollectors[collector] = true
		return nil
	}
}

func NewExporter(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, channelNames ChannelNames, filters ...string) (*Exporter, error) {
	f := make(map[string]bool)
	for _, filter := range filters {
		enabled, exist := collectorState[filter]
		if !exist {
			return nil, fmt.Errorf("missing collector: %s", filter)
		}

		if !*enabled {
			return nil, fmt.Errorf("disabled collector: %s", filter)
		}
		f[filter] = true
	}

	collectors := make(map[string]Collector)
	initiatedCollectorsMtx.Lock()
	defer initiatedCollectorsMtx.Unlock()
	for key, enabled := range collectorState {
		if !*enabled || (len(f) > 0 && !f[key]) {
			continue
		}
		if collector, ok := initiatedCollectors[key]; ok {
			collectors[key] = collector
		} else {
			collector, err := factories[key](logger, client, eventsubClient, channelNames)
			if err != nil {
				return nil, err
			}
			collectors[key] = collector
			initiatedCollectors[key] = collector
		}
	}

	for k := range collectors {
		logger.Info("enabled collector", "collector", k)
	}

	return &Exporter{
		Collectors: collectors,
		logger:     logger,
	}, nil
}

// NewProbeExporter builds a single-use Exporter for one /probe request. Unlike
// NewExporter it ignores the --collector.* flag state and instantiates the
// requested collectors fresh against the given channel names, so each probe can
// target a different set of channels. Only app-token (AuthApp) collectors are
// permitted; requesting a privileged collector or an unknown name is an error.
func NewProbeExporter(logger *slog.Logger, client *helix.Client, channelNames ChannelNames, requested []string) (*Exporter, error) {
	if len(requested) == 0 {
		return nil, errors.New("no collectors requested")
	}

	collectors := make(map[string]Collector)
	for _, name := range requested {
		if _, done := collectors[name]; done {
			continue
		}

		factory, ok := factories[name]
		if !ok {
			return nil, fmt.Errorf("unknown collector: %q", name)
		}

		if collectorAuthModes[name] != AuthApp {
			return nil, fmt.Errorf("collector %q requires a user access token and cannot be used in probe mode", name)
		}

		// probe collectors never need an eventsub client (that path is
		// user-token/webhook only), so it is always nil here.
		collector, err := factory(logger, client, nil, channelNames)
		if err != nil {
			return nil, err
		}
		collectors[name] = collector
	}

	return &Exporter{
		Collectors: collectors,
		logger:     logger,
	}, nil
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(e.Collectors))
	for name, c := range e.Collectors {
		go func(name string, c Collector) {
			execute(name, c, ch, e.logger)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}

func execute(name string, c Collector, ch chan<- prometheus.Metric, logger *slog.Logger) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		if IsNoDataError(err) {
			logger.Error("collector returned no data", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		} else {
			logger.Error("collector failed", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		}
		success = 0
	} else {
		logger.Info("collector succeeded", "name", name, "duration_seconds", duration.Seconds())
		success = 1
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

// Collector is the interface a collector has to implement.
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Update(ch chan<- prometheus.Metric) error
}

type typedDesc struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
}

func (d *typedDesc) mustNewConstMetric(value float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.desc, d.valueType, value, labels...)
}

var ErrNoData = errors.New("collector returned no data")

func IsNoDataError(err error) bool {
	return errors.Is(err, ErrNoData)
}

// countPaginated counts items across paginated API responses.
// fetchPage is called with a cursor (empty string for the first page) and
// returns the number of items on that page, the next cursor, and any error.
func countPaginated(fetchPage func(cursor string) (count int, next string, err error)) (int, error) {
	var total int
	cursor := ""

	for {
		count, next, err := fetchPage(cursor)
		if err != nil {
			return 0, err
		}

		total += count

		if next == "" {
			break
		}
		cursor = next
	}

	return total, nil
}

// maxHelixIDsPerRequest is the maximum number of logins/IDs the Twitch Helix
// API accepts in a single batched request (Get Users, Get Streams, Get Channel
// Information, etc). Requests with more entries must be split into chunks.
const maxHelixIDsPerRequest = 100

// chunkStrings splits s into consecutive slices of at most size elements. The
// returned slices share the backing array of s. An empty input yields nil.
func chunkStrings(s []string, size int) [][]string {
	if len(s) == 0 {
		return nil
	}
	if size <= 0 || len(s) <= size {
		return [][]string{s}
	}
	chunks := make([][]string, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

type userCacheEntry struct {
	user      helix.User
	expiresAt time.Time
}

var (
	// userCacheTTL bounds how long a resolved login -> user mapping is reused.
	// Twitch user IDs are stable and display names change rarely, so a short
	// TTL lets every collector in a scrape (and successive scrapes) share a
	// single Get Users call without serving meaningfully stale data.
	userCacheTTL = 5 * time.Minute

	userCacheMu    sync.RWMutex
	userCacheStore = make(map[string]userCacheEntry)

	// userResolveSF coalesces concurrent resolutions of the same set of logins
	// into a single API call. Collectors run concurrently within a scrape and
	// typically request the same channels, so without this the cold-cache case
	// would fan out into one Get Users call per collector.
	userResolveSF singleflight.Group
)

func cachedUser(login string) (helix.User, bool) {
	userCacheMu.RLock()
	defer userCacheMu.RUnlock()
	e, ok := userCacheStore[strings.ToLower(login)]
	if !ok || time.Now().After(e.expiresAt) {
		return helix.User{}, false
	}
	return e.user, true
}

func storeUsers(users []helix.User) {
	userCacheMu.Lock()
	defer userCacheMu.Unlock()
	expiresAt := time.Now().Add(userCacheTTL)
	for _, u := range users {
		userCacheStore[strings.ToLower(u.Login)] = userCacheEntry{user: u, expiresAt: expiresAt}
	}
}

// getUsers resolves channel login names to Twitch user objects, batching the
// lookups into grouped Get Users requests and sharing results through a
// short-lived cache so repeated resolutions across collectors and scrapes
// collapse to a single API call. Results preserve the order of logins;
// logins that do not resolve to a user are omitted. Passing no logins returns
// the authenticated user and bypasses the cache.
func getUsers(client *helix.Client, logger *slog.Logger, logins []string) ([]helix.User, error) {
	if len(logins) == 0 {
		return getUsersAPI(client, logger, nil)
	}

	resolved := make(map[string]helix.User, len(logins))
	missSet := make(map[string]bool, len(logins))
	var misses []string
	for _, login := range logins {
		key := strings.ToLower(login)
		if _, ok := resolved[key]; ok {
			continue
		}
		if u, ok := cachedUser(login); ok {
			resolved[key] = u
		} else if !missSet[key] {
			missSet[key] = true
			misses = append(misses, login)
		}
	}

	if len(misses) > 0 {
		fetched, err := resolveMissingUsers(client, logger, misses)
		if err != nil {
			return nil, err
		}
		for _, u := range fetched {
			resolved[strings.ToLower(u.Login)] = u
		}
	}

	result := make([]helix.User, 0, len(resolved))
	added := make(map[string]bool, len(resolved))
	for _, login := range logins {
		key := strings.ToLower(login)
		if added[key] {
			continue
		}
		if u, ok := resolved[key]; ok {
			result = append(result, u)
			added[key] = true
		}
	}
	return result, nil
}

// resolveMissingUsers fetches the users for the given logins, coalescing
// concurrent identical resolutions and caching the results.
func resolveMissingUsers(client *helix.Client, logger *slog.Logger, misses []string) ([]helix.User, error) {
	key := singleflightKey(misses)
	v, err, _ := userResolveSF.Do(key, func() (interface{}, error) {
		var users []helix.User
		for _, chunk := range chunkStrings(misses, maxHelixIDsPerRequest) {
			fetched, err := getUsersAPI(client, logger, chunk)
			if err != nil {
				return nil, err
			}
			users = append(users, fetched...)
		}
		storeUsers(users)
		return users, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]helix.User), nil
}

// singleflightKey builds an order-independent key for a set of logins so that
// concurrent callers requesting the same channels share one in-flight fetch.
func singleflightKey(logins []string) string {
	keys := make([]string, len(logins))
	for i, l := range logins {
		keys[i] = strings.ToLower(l)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// getUsersAPI performs a single Get Users call. Callers are responsible for
// chunking logins to maxHelixIDsPerRequest. Passing nil logins returns the
// authenticated user.
func getUsersAPI(client *helix.Client, logger *slog.Logger, logins []string) ([]helix.User, error) {
	resp, err := client.GetUsers(&helix.UsersParams{
		Logins: logins,
	})
	if err != nil {
		logger.Error("Failed to collect users stats from Twitch helix API", "err", err)
		return nil, err
	}
	if resp.StatusCode != 200 {
		logger.Error("Failed to collect users stats from Twitch helix API", "err", resp.ErrorMessage)
		return nil, errors.New(resp.ErrorMessage)
	}
	return resp.Data.Users, nil
}
