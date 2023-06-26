package tube

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"
)

const (
	tableSessions = "Sessions"

	sessionTTL = time.Hour * 24 * 7
)

type Session struct {
	Token   string `dynamo:",hash"`
	UserID  int
	Expires time.Time `dynamo:",unixtime"`
	IP      string
}

func CreateSession(ctx context.Context, userID int, ipaddr string) (Session, error) {
	token, err := randomString(64)
	if err != nil {
		return Session{}, err
	}

	sesh := Session{
		Token:   token,
		UserID:  userID,
		Expires: time.Now().UTC().Add(sessionTTL),
		IP:      ipaddr,
	}
	sessions := dynamoTable(tableSessions)
	err = sessions.Put(sesh).If("attribute_not_exists('Token')").Run()
	if err != nil {
		return Session{}, err
	}
	return sesh, nil
}

func GetSession(ctx context.Context, token string) (Session, error) {
	sessions := dynamoTable(tableSessions)
	var sesh Session
	err := sessions.Get("Token", token).One(&sesh)
	if err != nil {
		return Session{}, err
	}
	if time.Now().After(sesh.Expires) {
		return Session{}, ErrNotFound
	}
	return sesh, nil
}

func randomString(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
