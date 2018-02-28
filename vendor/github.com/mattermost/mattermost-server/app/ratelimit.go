// Copyright (c) 2018-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package app

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	l4g "github.com/alecthomas/log4go"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/utils"
	"github.com/pkg/errors"
	throttled "gopkg.in/throttled/throttled.v2"
	"gopkg.in/throttled/throttled.v2/store/memstore"
)

type RateLimiter struct {
	throttledRateLimiter *throttled.GCRARateLimiter
	useAuth              bool
	useIP                bool
	header               string
}

func NewRateLimiter(settings *model.RateLimitSettings) (*RateLimiter, error) {
	store, err := memstore.New(*settings.MemoryStoreSize)
	if err != nil {
		return nil, errors.Wrap(err, utils.T("api.server.start_server.rate_limiting_memory_store"))
	}

	quota := throttled.RateQuota{
		MaxRate:  throttled.PerSec(*settings.PerSec),
		MaxBurst: *settings.MaxBurst,
	}

	throttledRateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		return nil, errors.Wrap(err, utils.T("api.server.start_server.rate_limiting_rate_limiter"))
	}

	return &RateLimiter{
		throttledRateLimiter: throttledRateLimiter,
		useAuth:              *settings.VaryByUser,
		useIP:                *settings.VaryByRemoteAddr,
		header:               settings.VaryByHeader,
	}, nil
}

func (rl *RateLimiter) GenerateKey(r *http.Request) string {
	key := ""

	if rl.useAuth {
		token, tokenLocation := ParseAuthTokenFromRequest(r)
		if tokenLocation != TokenLocationNotFound {
			key += token
		} else if rl.useIP { // If we don't find an authentication token and IP based is enabled, fall back to IP
			key += utils.GetIpAddress(r)
		}
	} else if rl.useIP { // Only if Auth based is not enabed do we use a plain IP based
		key += utils.GetIpAddress(r)
	}

	// Note that most of the time the user won't have to set this because the utils.GetIpAddress above tries the
	// most common headers anyway.
	if rl.header != "" {
		key += strings.ToLower(r.Header.Get(rl.header))
	}

	return key
}

func (rl *RateLimiter) RateLimitWriter(key string, w http.ResponseWriter) bool {
	limited, context, err := rl.throttledRateLimiter.RateLimit(key, 1)
	if err != nil {
		l4g.Critical("Internal server error when rate limiting. Rate Limiting broken. Error:" + err.Error())
		return false
	}

	setRateLimitHeaders(w, context)

	if limited {
		l4g.Error("Denied due to throttling settings code=429 key=%v", key)
		http.Error(w, "limit exceeded", 429)
	}

	return limited
}

func (rl *RateLimiter) UserIdRateLimit(userId string, w http.ResponseWriter) bool {
	if rl.useAuth {
		if rl.RateLimitWriter(userId, w) {
			return true
		}
	}
	return false
}

func (rl *RateLimiter) RateLimitHandler(wrappedHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.GenerateKey(r)
		limited := rl.RateLimitWriter(key, w)

		if !limited {
			wrappedHandler.ServeHTTP(w, r)
		}
	})
}

// Copied from https://github.com/throttled/throttled http.go
func setRateLimitHeaders(w http.ResponseWriter, context throttled.RateLimitResult) {
	if v := context.Limit; v >= 0 {
		w.Header().Add("X-RateLimit-Limit", strconv.Itoa(v))
	}

	if v := context.Remaining; v >= 0 {
		w.Header().Add("X-RateLimit-Remaining", strconv.Itoa(v))
	}

	if v := context.ResetAfter; v >= 0 {
		vi := int(math.Ceil(v.Seconds()))
		w.Header().Add("X-RateLimit-Reset", strconv.Itoa(vi))
	}

	if v := context.RetryAfter; v >= 0 {
		vi := int(math.Ceil(v.Seconds()))
		w.Header().Add("Retry-After", strconv.Itoa(vi))
	}
}
