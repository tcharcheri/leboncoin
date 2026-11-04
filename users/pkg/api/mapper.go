package api

import "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"

// toUser maps a user returned by the identity service to the client model.
func toUser(u User) pkg.User {
	return pkg.User{
		UserID:   u.UserID,
		Nickname: u.Nickname,
	}
}

// toUsers maps users returned by the identity service to the client model.
func toUsers(us []User) []pkg.User {
	users := make([]pkg.User, 0, len(us))
	for _, u := range us {
		users = append(users, toUser(u))
	}

	return users
}
