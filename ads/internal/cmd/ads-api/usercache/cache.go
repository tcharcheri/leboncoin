// Package usercache provides a read-through, in-memory cache in front of the user service.
//
// The ad page is read far more often than nicknames change, so caching them keeps
// the user service out of the hot path and absorbs traffic spikes on popular ads.
package usercache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
)

type entry struct {
	user      usersclient.User
	expiresAt time.Time
}

// Client caches GetUserByID results for a fixed TTL. Errors are never cached.
// It is safe for concurrent use.
type Client struct {
	next       usersclient.UserClient
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.RWMutex
	entries map[model.UserID]entry

	// group collapses concurrent misses on the same user into a single upstream call.
	group singleflight.Group
}

// New wraps next with a cache keeping at most maxEntries users for ttl.
func New(next usersclient.UserClient, ttl time.Duration, maxEntries int) *Client {
	return &Client{
		next:       next,
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        time.Now,
		entries:    make(map[model.UserID]entry, maxEntries),
	}
}

// GetUserByID returns the cached user, or fetches it from the user service on a miss.
func (c *Client) GetUserByID(ctx context.Context, userID model.UserID) (usersclient.User, error) {
	if user, ok := c.lookup(userID); ok {
		return user, nil
	}

	v, err, _ := c.group.Do(userID.String(), func() (any, error) {
		// The shared call must not fail for every waiter because the first caller went away:
		// detach it from that caller's cancellation. The HTTP client timeout still bounds it.
		user, err := c.next.GetUserByID(context.WithoutCancel(ctx), userID)
		if err != nil {
			return usersclient.User{}, err
		}

		c.store(userID, user)

		return user, nil
	})
	if err != nil {
		return usersclient.User{}, err
	}

	return v.(usersclient.User), nil
}

// ListUsers is not cached: it is not on the hot path.
func (c *Client) ListUsers(ctx context.Context) ([]usersclient.User, error) {
	return c.next.ListUsers(ctx)
}

func (c *Client) lookup(userID model.UserID) (usersclient.User, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[userID]
	if !ok || !c.now().Before(e.expiresAt) {
		return usersclient.User{}, false
	}

	return e.user, true
}

func (c *Client) store(userID model.UserID, user usersclient.User) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxEntries {
		c.evict()
	}

	c.entries[userID] = entry{user: user, expiresAt: c.now().Add(c.ttl)}
}

// evict drops expired entries and, if the cache is still full, one arbitrary entry,
// so memory stays bounded. Callers must hold the write lock.
func (c *Client) evict() {
	now := c.now()
	for id, e := range c.entries {
		if !now.Before(e.expiresAt) {
			delete(c.entries, id)
		}
	}

	if len(c.entries) < c.maxEntries {
		return
	}

	for id := range c.entries {
		delete(c.entries, id)
		return
	}
}
