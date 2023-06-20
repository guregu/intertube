package web

import (
	"context"
	"net/http"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/guregu/intertube/tube"
)

type localizerkey struct{}
type langkey struct{}
type userkey struct{}
type pathkey struct{}
type bypasskey struct{}

func discover(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	acceptlang := r.Header.Get("Accept-Language")
	localizer := i18n.NewLocalizer(translations, acceptlang)
	ctx = withLocalizer(ctx, localizer)
	ctx = withLanguage(ctx, acceptlang)
	ctx = withPath(ctx, r.URL.Path)
	return ctx
}

var serverStartTime = time.Now().UTC()

func cacheHeaders(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
	// w.Header().Set("Cache-Control", "no-cache, must-revalidate")

	// if DebugMode {
	// 	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	// 	return ctx
	// }

	if u, ok := userFrom(ctx); ok && !u.LastMod.IsZero() {
		lm := lastestMod(u.LastMod)
		w.Header().Set("Last-Modified", lm.Format(http.TimeFormat))
	} else {
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	}
	return ctx
}

func lastestMod(usermod time.Time) time.Time {
	if usermod.Before(Deployed) {
		return Deployed
	}
	return usermod
}

func withLocalizer(ctx context.Context, loc *i18n.Localizer) context.Context {
	return context.WithValue(ctx, localizerkey{}, loc)
}

func localizerFrom(ctx context.Context) *i18n.Localizer {
	loc, _ := ctx.Value(localizerkey{}).(*i18n.Localizer)
	if loc == nil {
		return i18n.NewLocalizer(translations, "en")
	}
	return loc
}

func withLanguage(ctx context.Context, langs ...string) context.Context {
	for _, lang := range langs {
		if lang != "" {
			return context.WithValue(ctx, langkey{}, lang)
		}
	}
	return ctx
}

func languageFrom(ctx context.Context) string {
	lang, ok := ctx.Value(langkey{}).(string)
	if !ok {
		return "ja"
	}
	return lang
}

func withUser(ctx context.Context, user tube.User) context.Context {
	return context.WithValue(ctx, userkey{}, user)
}

func userFrom(ctx context.Context) (tube.User, bool) {
	u, ok := ctx.Value(userkey{}).(tube.User)
	return u, ok
}

// TODO: maybe change this to the URL obj
func withPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, pathkey{}, path)
}

func pathFrom(ctx context.Context) string {
	path, _ := ctx.Value(pathkey{}).(string)
	return path
}

func withBypass(ctx context.Context, ok bool) context.Context {
	return context.WithValue(ctx, bypasskey{}, ok)
}

func bypassFrom(ctx context.Context) bool {
	ok, _ := ctx.Value(bypasskey{}).(bool)
	return ok
}
