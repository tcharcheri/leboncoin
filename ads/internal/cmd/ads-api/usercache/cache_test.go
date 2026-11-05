package usercache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersmock "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/mock"
)

var alice = usersclient.User{UserID: "user-1", Nickname: "alice"}

// newTestCache returns a cache whose clock is driven by the test.
func newTestCache(next usersclient.UserClient, ttl time.Duration, maxEntries int) (*Client, *time.Time) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	c := New(next, ttl, maxEntries)
	c.now = func() time.Time { return now }

	return c, &now
}

func TestClientImplementsUserClient(t *testing.T) {
	assert.Implements(t, (*usersclient.UserClient)(nil), new(Client))
}

func TestGetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("serves the second call from the cache", func(t *testing.T) {
		m := usersmock.New(t)
		m.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(alice, nil).Once()
		c, _ := newTestCache(m, time.Minute, 10)

		for range 2 {
			got, err := c.GetUserByID(ctx, "user-1")
			require.NoError(t, err)
			assert.Equal(t, alice, got)
		}
	})

	t.Run("fetches again once the entry expired", func(t *testing.T) {
		m := usersmock.New(t)
		m.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(alice, nil).Twice()
		c, now := newTestCache(m, time.Minute, 10)

		_, err := c.GetUserByID(ctx, "user-1")
		require.NoError(t, err)

		*now = now.Add(time.Minute)

		_, err = c.GetUserByID(ctx, "user-1")
		require.NoError(t, err)
	})

	t.Run("does not cache errors", func(t *testing.T) {
		m := usersmock.New(t)
		boom := errors.New("user service down")
		m.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(usersclient.User{}, boom).Once()
		m.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(alice, nil).Once()
		c, _ := newTestCache(m, time.Minute, 10)

		_, err := c.GetUserByID(ctx, "user-1")
		require.ErrorIs(t, err, boom)

		got, err := c.GetUserByID(ctx, "user-1")
		require.NoError(t, err)
		assert.Equal(t, alice, got)
	})

	t.Run("never holds more than maxEntries users", func(t *testing.T) {
		m := usersmock.New(t)
		m.On("GetUserByID", mock.Anything, mock.Anything).Return(alice, nil)
		c, _ := newTestCache(m, time.Minute, 2)

		for _, id := range []model.UserID{"user-1", "user-2", "user-3"} {
			_, err := c.GetUserByID(ctx, id)
			require.NoError(t, err)
		}

		assert.Len(t, c.entries, 2)
	})

	t.Run("collapses concurrent misses into one upstream call", func(t *testing.T) {
		m := usersmock.New(t)
		release := make(chan time.Time)
		m.On("GetUserByID", mock.Anything, model.UserID("user-1")).
			WaitUntil(release).
			Return(alice, nil).
			Once()
		c, _ := newTestCache(m, time.Minute, 10)

		const callers = 20
		var wg sync.WaitGroup
		for range callers {
			wg.Go(func() {
				got, err := c.GetUserByID(ctx, "user-1")
				assert.NoError(t, err)
				assert.Equal(t, alice, got)
			})
		}

		time.Sleep(50 * time.Millisecond) // let every caller join the in-flight call
		close(release)
		wg.Wait()
	})
}
