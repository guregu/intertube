package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/guregu/intertube/storage"
	"github.com/guregu/intertube/tube"
)

type settingsFormData struct {
	User         tube.User
	Plan         tube.Plan
	HasSub       bool
	ErrorMsg     string
	CacheEnabled bool
}

func settingsForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	plan := tube.GetPlan(u.Plan)

	var hasSub bool
	if UseStripe {
		cust, err := getCustomer(u.CustomerID)
		if err != nil {
			panic(err)
		}
		// spew.Dump(cust)
		hasSub = cust.Subscriptions != nil && len(cust.Subscriptions.Data) > 0
	}

	data := settingsFormData{
		User:         u,
		HasSub:       hasSub,
		Plan:         plan,
		CacheEnabled: storage.IsCacheEnabled(),
	}
	if err := getTemplate(ctx, "settings").Execute(w, data); err != nil {
		panic(err)
	}
}

func settings(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	plan := tube.GetPlan(u.Plan)

	cust, err := getCustomer(u.CustomerID)
	if err != nil {
		fmt.Println("cust err", err)
		// panic(err)
	}
	// spew.Dump(cust)
	hasSub := cust != nil && cust.Subscriptions != nil && len(cust.Subscriptions.Data) > 0

	renderError := func(err error) {
		data := settingsFormData{
			User:         u,
			Plan:         plan,
			HasSub:       hasSub,
			ErrorMsg:     err.Error(),
			CacheEnabled: storage.IsCacheEnabled(),
		}
		if err := getTemplate(ctx, "settings").Execute(w, data); err != nil {
			panic(err)
		}
	}

	email := r.FormValue("email")
	if email != "" && u.Email != email {
		if err := u.SetEmail(ctx, email); err != nil {
			renderError(err)
			return
		}
	}

	theme := r.FormValue("theme")
	if u.Theme != theme {
		if err := u.SetTheme(ctx, theme); err != nil {
			renderError(err)
			return
		}
	}

	disp := tube.DisplayOptions{}
	disp.Stretch = r.FormValue("display-stretch") == "on"
	switch r.FormValue("musiclink") {
	case "albums":
		disp.MusicLink = tube.MusicLinkAlbums
	default:
		disp.MusicLink = tube.MusicLinkDefault
	}
	switch r.FormValue("trackselect") {
	case "ctrl":
		disp.TrackSelect = tube.TrackSelCtrlKey
	default:
		disp.TrackSelect = tube.TrackSelDefault
	}
	if u.Display != disp {
		if err := u.SetDisplayOpt(ctx, disp); err != nil {
			renderError(err)
			return
		}
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func changePasswordForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	data := struct {
		User     tube.User
		ErrorMsg string
		Success  bool
	}{
		User: u,
	}
	if err := getTemplate(ctx, "settings-password").Execute(w, data); err != nil {
		panic(err)
	}
}

func changePassword(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	renderError := func(err error) {
		data := struct {
			User     tube.User
			ErrorMsg string
			Success  bool
		}{
			User:     u,
			ErrorMsg: err.Error(),
		}
		if err := getTemplate(ctx, "settings-password").Execute(w, data); err != nil {
			panic(err)
		}
	}

	oldpw := r.FormValue("old-password")
	newpw := r.FormValue("new-password")
	confirm := r.FormValue("new-password-confirm")

	if !u.ValidPassword(oldpw) {
		renderError(fmt.Errorf("current password is incorrect"))
		return
	}

	if newpw == "" || confirm == "" {
		renderError(fmt.Errorf("missing input"))
		return
	}

	if newpw != confirm {
		renderError(fmt.Errorf("new password and confirmation don't match"))
		return
	}

	hashed, err := tube.HashPassword(newpw)
	if err != nil {
		renderError(err)
		return
	}

	if err := u.SetPassword(ctx, hashed); err != nil {
		renderError(err)
		return
	}

	data := struct {
		User     tube.User
		ErrorMsg string
		Success  bool
	}{
		User:    u,
		Success: true,
	}
	if err := getTemplate(ctx, "settings-password").Execute(w, data); err != nil {
		panic(err)
	}
}
