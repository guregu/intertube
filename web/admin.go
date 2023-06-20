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
		if a.Usage == b.Usage {
			return a.ID < b.ID
		}
		return a.Usage > b.Usage
	})

	data := struct {
		Users []tube.User
	}{
		Users: users,
	}

	if err := getTemplate(ctx, "admin").Execute(w, data); err != nil {
		panic(err)
	}
}
