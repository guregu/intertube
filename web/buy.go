package web

import (
	"context"
	"fmt"
	"net/http"

	// "github.com/davecgh/go-spew/spew"
	"github.com/stripe/stripe-go/v72"

	"github.com/guregu/intertube/tube"
)

func buyForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if !UseStripe {
		http.Error(w, "payment is disabled", http.StatusForbidden)
		return
	}

	u, loggedIn := userFrom(ctx)
	plans := tube.GetPlans()
	prices, err := getStripePrices(plans)
	if err != nil {
		panic(err)
	}

	var hasSub bool
	if loggedIn {
		cust, err := getCustomer(u.CustomerID)
		if err != nil {
			panic(err)
		}
		// spew.Dump(cust)
		hasSub = cust.Subscriptions != nil && len(cust.Subscriptions.Data) > 0
	}

	data := struct {
		StripeKey string
		Plans     []tube.Plan
		Prices    map[tube.PlanKind]*stripe.Price
		User      tube.User
		HasSub    bool
	}{
		StripeKey: stripePublicKey,
		Plans:     plans,
		Prices:    prices,
		User:      u,
		HasSub:    hasSub,
	}

	if err := getTemplate(ctx, "buy").Execute(w, data); err != nil {
		panic(err)
	}
}

func buySuccess(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")

	fmt.Fprintf(w, "success sesh id %s", sessionID)
}
