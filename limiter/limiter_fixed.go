package limiter

import (
	"github.com/sujit-baniya/framework/contracts/http"
	"github.com/sujit-baniya/framework/utils"
	"strconv"
	"sync"
	"sync/atomic"
)

type FixedWindow struct{}

// New creates a new fixed window middleware handler
func (FixedWindow) New(cfg Config) http.HandlerFunc {
	var (
		// Limiter variables
		mux        = &sync.RWMutex{}
		max        = strconv.Itoa(cfg.Max)
		expiration = uint64(cfg.Expiration.Seconds())
	)

	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage)

	// Update timestamp every second
	utils.StartTimeStampUpdater()

	// Return new handler
	return func(c http.Context) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get key from request
		key := cfg.KeyGenerator(c)

		// Lock entry
		mux.Lock()

		// Get entry from pool and release when finished
		e := manager.get(key)

		// Get timestamp
		ts := uint64(atomic.LoadUint32(&utils.Timestamp))

		// Set expiration if entry does not exist
		if e.exp == 0 {
			e.exp = ts + expiration
		} else if ts >= e.exp {
			// Check if entry is expired
			e.currHits = 0
			e.exp = ts + expiration
		}

		// Increment hits
		e.currHits++

		// Calculate when it resets in seconds
		resetInSec := e.exp - ts

		// Set how many hits we have left
		remaining := cfg.Max - e.currHits

		// Update storage
		manager.set(key, e, cfg.Expiration)

		// Unlock entry
		mux.Unlock()

		// Check if hits exceed the cfg.Max
		if remaining < 0 {
			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc6584
			c.SetHeader(utils.HeaderRetryAfter, strconv.FormatUint(resetInSec, 10))

			// Call LimitReached handler
			return cfg.LimitReached(c)
		}

		// Continue stack for reaching c.Response().StatusCode()
		// Store err for returning
		err := c.Next()

		// Check for SkipFailedRequests and SkipSuccessfulRequests
		if (cfg.SkipSuccessfulRequests && c.StatusCode() < utils.StatusBadRequest) ||
			(cfg.SkipFailedRequests && c.StatusCode() >= utils.StatusBadRequest) {
			// Lock entry
			mux.Lock()
			e = manager.get(key)
			e.currHits--
			remaining++
			manager.set(key, e, cfg.Expiration)
			// Unlock entry
			mux.Unlock()
		}

		// We can continue, update RateLimit headers
		c.SetHeader(xRateLimitLimit, max)
		c.SetHeader(xRateLimitRemaining, strconv.Itoa(remaining))
		c.SetHeader(xRateLimitReset, strconv.FormatUint(resetInSec, 10))

		return err
	}
}
