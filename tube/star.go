package tube

import (
	"context"
	"time"
)

type Star struct {
	UserID int  `dynamo:",hash"`
	SSID   SSID `dynamo:",range"`
	Date   time.Time
}

func SetStar(ctx context.Context, userID int, ssid SSID, date time.Time) error {
	table := dynamoTable("Stars")
	return table.Put(Star{
		UserID: userID,
		SSID:   ssid,
		Date:   date,
	}).Run()
}

func DeleteStar(ctx context.Context, userID int, ssid string) error {
	table := dynamoTable("Stars")
	return table.Delete("UserID", userID).Range("SSID", ssid).Run()
}

func GetStars(ctx context.Context, userID int) (map[SSID]Star, error) {
	table := dynamoTable("Stars")
	iter := table.Get("UserID", userID).Iter()
	stars := make(map[SSID]Star)
	var s Star
	for iter.Next(&s) {
		stars[s.SSID] = s
	}
	if err := iter.Err(); err != nil && iter.Err() != ErrNotFound {
		return nil, err
	}
	return stars, nil
}
