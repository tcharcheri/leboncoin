package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
)

func TestToUser(t *testing.T) {
	t.Run("maps every field", func(t *testing.T) {
		in := User{UserID: "00000000-0000-0000-0000-000000000001", Nickname: "alice"}
		got := toUser(in)
		assert.Equal(t, pkg.User{UserID: "00000000-0000-0000-0000-000000000001", Nickname: "alice"}, got)
	})

	t.Run("maps a zero value to a zero value", func(t *testing.T) {
		got := toUser(User{})
		assert.Equal(t, pkg.User{}, got)
	})
}

func TestToUsers(t *testing.T) {
	t.Run("maps every user and keeps the order", func(t *testing.T) {
		in := []User{
			{UserID: "00000000-0000-0000-0000-000000000001", Nickname: "alice"},
			{UserID: "00000000-0000-0000-0000-000000000002", Nickname: "bob"},
		}

		got := toUsers(in)

		assert.Equal(t, []pkg.User{
			{UserID: "00000000-0000-0000-0000-000000000001", Nickname: "alice"},
			{UserID: "00000000-0000-0000-0000-000000000002", Nickname: "bob"},
		}, got)
	})

	t.Run("returns an empty non-nil slice for an empty input", func(t *testing.T) {
		got := toUsers([]User{})
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("returns an empty non-nil slice for a nil input", func(t *testing.T) {
		got := toUsers(nil)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}
