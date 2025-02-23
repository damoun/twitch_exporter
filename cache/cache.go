package cache

import (
	"time"

	cacheLib "github.com/eko/gocache/lib/v4/cache"
	gocache_store "github.com/eko/gocache/store/go_cache/v4"
	gocache "github.com/patrickmn/go-cache"
)

// DefaultCache
// override with own cache client to implement alternative cache endpoint
var DefaultCache *cacheLib.Cache[[]byte]

func init() {
	gocacheClient := gocache.New(30*time.Minute, time.Hour)
	gocacheStore := gocache_store.NewGoCache(gocacheClient)

	DefaultCache = cacheLib.New[[]byte](gocacheStore)
}
