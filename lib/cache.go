package lib

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/orsinium-labs/josh"
	"github.com/orsinium-labs/josh/statuses"
)

type Cache = *ttlcache.Cache[string, any]

func withCache(ttl time.Duration, h josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		cache := josh.Must(josh.GetSingleton[Cache](r))
		key := r.URL.Path
		cached := cache.Get(key)
		if cached != nil {
			return josh.Ok(cached.Value())
		}
		// TODO: single flight
		resp := h(r)
		if resp.Status == statuses.OK {
			cache.Set(key, resp.Data, ttl)
		}
		return resp
	}
}
