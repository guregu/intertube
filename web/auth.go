package web

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	mailer "github.com/guregu/intertube/email"
	"github.com/guregu/intertube/tube"
)

const sessionCookie = "sesh"

func allowGuest(path ...string) func(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
		for _, p := range path {
			if r.URL.Path == p {
				ctx = withBypass(ctx, true)
				return ctx
			}
		}
		return ctx
	}
}

func requireLogin(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	maybeRedir := func() context.Context {
		if bypassFrom(ctx) {
			return ctx
		}
		redirectLogin(ctx, w, r)
		return nil
	}

	if isSubsonicReq(r) {
		return subsonicAuth(ctx, w, r)
	}

	cookieAuth, err := r.Cookie(sessionCookie)
	if err != nil {
		return maybeRedir()
	}

	sesh, err := tube.GetSession(ctx, cookieAuth.Value)
	if err != nil {
		for _, cookie := range expiredAuthCookies() {
			http.SetCookie(w, cookie)
		}
		return maybeRedir()
	}

	user, err := tube.GetUser(ctx, sesh.UserID)
	if err != nil {
		for _, cookie := range expiredAuthCookies() {
			http.SetCookie(w, cookie)
		}
		return maybeRedir()
	}

	// if err := ensureStripeCustomer(ctx, &user); err != nil {
	// 	panic(err)
	// }

	ctx = withUser(ctx, user)
	return ctx
}

func redirectLogin(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var q string
	if param := encodeRedirect(r.URL); param != "" {
		q = "?r=" + encodeRedirect(r.URL)
	}
	http.Redirect(w, r, "/login"+q, http.StatusTemporaryRedirect)
}

func requireAdmin(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	u, _ := userFrom(ctx)
	if u.ID != 2 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	return ctx
}

func loginForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	_, loggedIn := userFrom(ctx)
	if loggedIn {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	var data = struct {
		Jump string
	}{}

	if rawJump := r.URL.Query().Get("r"); rawJump != "" {
		data.Jump, _ = decodeRedirect(rawJump)
	}

	if err := getTemplate(ctx, "login").Execute(w, data); err != nil {
		panic(err)
	}
}

// TODO: friendly error messages
func login(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	pass := r.FormValue("password")
	jump := r.FormValue("jump")

	user, err := tube.GetUserByEmail(ctx, email)
	if err != nil {
		panic(err)
	}

	if !user.ValidPassword(pass) {
		panic("bad password")
	}

	sesh, err := tube.CreateSession(ctx, user.ID, ipAddress(r))
	if err != nil {
		panic(err)
	}

	if jump == "" {
		jump = "/"
	}

	http.SetCookie(w, validAuthCookie(sesh))
	http.Redirect(w, r, jump, http.StatusSeeOther)
}

func logout(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	for _, cookie := range expiredAuthCookies() {
		http.SetCookie(w, cookie)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func registerForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var data = struct {
		Email    string
		Invite   string
		ErrorMsg string
	}{
		Invite: r.URL.Query().Get("invite"),
	}
	if err := getTemplate(ctx, "register").Execute(w, data); err != nil {
		panic(err)
	}
}

func register(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	pass := r.FormValue("password")
	conf := r.FormValue("password-confirm")
	// secret := r.FormValue("secret")
	agree := r.FormValue("agree") == "on"

	renderError := func(err error) {
		var data = struct {
			Email    string
			Invite   string
			ErrorMsg string
		}{
			Email: email,
			// Invite:   secret,
			ErrorMsg: err.Error(),
		}
		if err := getTemplate(ctx, "register").Execute(w, data); err != nil {
			panic(err)
		}
	}

	if _, err := mail.ParseAddress(email); err != nil {
		renderError(fmt.Errorf("invalid e-mail address: %w", err))
		return
	}

	if pass != conf {
		renderError(fmt.Errorf("password and confirmation don't match"))
		return
	}

	// if secret != registerSecret {
	// 	renderError(fmt.Errorf("invalid invite code"))
	// 	return
	// }

	if !agree {
		renderError(fmt.Errorf("you must agree to the terms"))
		return
	}

	user, err := tube.GetUserByEmail(ctx, email)
	if err == nil {
		renderError(fmt.Errorf("e-mail already registered"))
		return
	}
	if err != tube.ErrNotFound {
		renderError(err)
		return
	}

	pwhash, err := tube.HashPassword(pass)
	if err != nil {
		renderError(err)
		return
	}

	user = tube.User{
		Email:    email,
		Password: pwhash,
	}
	if err := user.Create(ctx); err != nil {
		renderError(err)
		return
	}

	sesh, err := tube.CreateSession(ctx, user.ID, ipAddress(r))
	if err != nil {
		renderError(err)
		return
	}

	http.SetCookie(w, validAuthCookie(sesh))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func forgotForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var data = struct {
		ErrorMsg string
	}{}
	if err := getTemplate(ctx, "forgot").Execute(w, data); err != nil {
		panic(err)
	}
}

func forgot(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")

	renderError := func(err error) {
		var data = struct {
			ErrorMsg string
		}{
			ErrorMsg: err.Error(),
		}
		if err := getTemplate(ctx, "forgot").Execute(w, data); err != nil {
			panic(err)
		}
	}

	u, err := tube.GetUserByEmail(ctx, email)
	if err == tube.ErrNotFound {
		renderError(fmt.Errorf("there is no account with that e-mail address"))
		return
	}
	if err != nil {
		renderError(err)
		return
	}

	if err := u.SetRandomRecovery(ctx); err != nil {
		renderError(err)
		return
	}

	subject := fmt.Sprintf("Password reset for %s", Domain)
	url := fmt.Sprintf("https://%s/recover?id=%d&token=%s", Domain, u.ID, u.Recovery)
	body := fmt.Sprintf(`A password reset for your account %s at %s has been requested. <br>
	Please use the following link to change your password: <br>
	<a href="%s">%s</a> <br>
	If you did not request this, you may ignore it.`, u.Email, Domain, url, url)

	if err := mailer.Send(Domain+" Account", u.Email, subject, body); err != nil {
		renderError(err)
		return
	}

	data := struct {
		Email string
	}{
		Email: email,
	}
	if err := getTemplate(ctx, "forgot-sent").Execute(w, data); err != nil {
		panic(err)
	}
}

func recoverForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	token := r.URL.Query().Get("token")

	u, err := tube.GetUser(ctx, id)
	if err != nil {
		panic(err)
	}

	if u.Recovery == "" || u.Recovery != token {
		panic("invalid token")
	}

	var data = struct {
		UserID   int
		Token    string
		Email    string
		ErrorMsg string
	}{
		UserID: id,
		Token:  token,
		Email:  u.Email,
	}

	if err := getTemplate(ctx, "recover").Execute(w, data); err != nil {
		panic(err)
	}
}

func doRecover(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID, _ := strconv.Atoi(r.FormValue("userid"))
	token := r.FormValue("token")
	pass := r.FormValue("password")
	conf := r.FormValue("password-confirm")
	email := r.FormValue("email")

	renderError := func(err error) {
		var data = struct {
			Email    string
			Token    string
			UserID   int
			ErrorMsg string
		}{
			Email:    email,
			Token:    token,
			UserID:   userID,
			ErrorMsg: err.Error(),
		}
		if err := getTemplate(ctx, "recover").Execute(w, data); err != nil {
			panic(err)
		}
	}

	u, err := tube.GetUser(ctx, userID)
	if err != nil {
		renderError(err)
		return
	}

	if u.Recovery == "" || u.Recovery != token {
		renderError(fmt.Errorf("invalid recovery token"))
		return
	}

	if pass != conf {
		renderError(fmt.Errorf("password and confirmation don't match"))
		return
	}

	pwhash, err := tube.HashPassword(pass)
	if err != nil {
		renderError(fmt.Errorf("password and confirmation don't match"))
		return
	}

	if err := u.SetPassword(ctx, pwhash); err != nil {
		renderError(err)
		return
	}

	sesh, err := tube.CreateSession(ctx, u.ID, ipAddress(r))
	if err != nil {
		renderError(err)
		return
	}

	http.SetCookie(w, validAuthCookie(sesh))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func validAuthCookie(sesh tube.Session) *http.Cookie {
	domain := "." + Domain
	if DebugMode {
		domain = ""
	}
	return &http.Cookie{
		Name:     sessionCookie,
		Domain:   domain,
		Path:     "/",
		Value:    sesh.Token,
		Expires:  sesh.Expires,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   !DebugMode,
	}
}

func expiredAuthCookies() []*http.Cookie {
	domain := "." + Domain
	if DebugMode {
		domain = ""
	}
	return []*http.Cookie{
		&http.Cookie{
			Name:     sessionCookie,
			Domain:   "." + domain,
			Path:     "/",
			Value:    "",
			Expires:  time.Now().Add(-10000 * time.Hour),
			SameSite: http.SameSiteLaxMode,
			HttpOnly: true,
			Secure:   !DebugMode,
		}, &http.Cookie{
			Name:     sessionCookie,
			Domain:   domain,
			Path:     "/",
			Value:    "",
			Expires:  time.Now().Add(-10000 * time.Hour),
			SameSite: http.SameSiteLaxMode,
			HttpOnly: true,
			Secure:   !DebugMode,
		},
	}
}

func ipAddress(r *http.Request) string {
	return r.Header.Get("X-Forwarded-For")
}

func encodeRedirect(href *url.URL) string {
	uri := strings.TrimPrefix(href.RequestURI(), "/")
	return base64.RawURLEncoding.EncodeToString([]byte(uri))
}

func decodeRedirect(param string) (string, error) {
	uri, err := base64.RawURLEncoding.DecodeString(param)
	if err != nil {
		return "", err
	}
	return "/" + string(uri), nil
}
