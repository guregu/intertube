package web

import (
	"context"
	"net/http"
	"sort"

	"github.com/guregu/intertube/tube"
)

func adminIndex(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	users, err := tube.GetAllUsers(ctx)
	if err != nil {
		panic(err)
	}

	sort.Slice(users, func(i, j int) bool {
		a, b := users[i], users[j]
		return a.LastMod.After(b.LastMod)
	})

	data := struct {
		Users []tube.User
	}{
		Users: users,
	}

	renderTemplate(ctx, w, "admin", data, http.StatusOK)
}
