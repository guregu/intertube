package web

import (
	"context"
	"fmt"
	"time"

	"github.com/guregu/intertube/tube"
)

func MIGRATE_MAKEDUMPS() {
	ctx := context.Background()
	users, err := tube.GetAllUsers(ctx)
	if err != nil {
		panic(err)
	}
	for _, u := range users {
		if u.Tracks == 0 {
			continue
		}
		if err := tube.RecreateDump(ctx, u.ID, time.Now().UTC()); err != nil {
			panic(err)
		}
		fmt.Println("dumped", u.ID, u.Email)
	}
}

func MIGRATE_SORTID() {
	ctx := context.Background()

	fmt.Println("fixing sort IDs...")

	iter := tube.GetALLTracks(ctx)
	var t tube.Track
	for iter.Next(&t) {
		if t.SortID == t.SortKey() {
			fmt.Println("skipping", t.SortKey())
			continue
		}
		if err := t.RefreshSortID(ctx); err != nil {
			fmt.Println("ERROR", err, t.ID, t.UserID, t.SortKey())
			continue
		}
		fmt.Println("set", t.SortID, "/", t.UserID)
	}
	if iter.Err() != nil {
		panic(iter.Err())
	}
}
