package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v72"
	portalsession "github.com/stripe/stripe-go/v72/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v72/checkout/session"
	"github.com/stripe/stripe-go/v72/customer"
	stripeprice "github.com/stripe/stripe-go/v72/price"
	"github.com/stripe/stripe-go/v72/product"
	"github.com/stripe/stripe-go/v72/webhook"

	// "github.com/stripe/stripe-go/v72/client"
	"github.com/davecgh/go-spew/spew"

	"github.com/guregu/intertube/tube"
)

var (
	stripePublicKey string
	stripeSigSecret string // for webhook
)

func initStripe() {
	key := os.Getenv("STRIPE_KEY")
	if key == "" {
		log.Println("no stripe private key")
		return
	}
	stripe.Key = key

	stripePublicKey = os.Getenv("STRIPE_PUBLIC")
	if stripePublicKey == "" {
		log.Println("no stripe public key")
	}

	stripeSigSecret = os.Getenv("STRIPE_SIG")
	if stripeSigSecret == "" {
		log.Println("no stripe webhook sig")
	}
}

func stripePortal(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	// get id
	// if u.CustomerID == "" {
	// 	params := &stripe.CustomerParams{
	// 		Email: stripe.String(u.Email),
	// 	}
	// 	params.AddMetadata("account", strconv.Itoa(u.ID))
	// 	c, err := customer.New(params)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	if err := u.SetCustomerID(ctx, c.ID); err != nil {
	// 		panic(err)
	// 	}
	// }

	sesh, err := portalsession.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(u.CustomerID),
		ReturnURL: stripe.String(fmt.Sprintf("https://%s/settings", Domain)),
	})
	// TODO: nice error msg
	if err != nil {
		panic(err)
	}

	http.Redirect(w, r, sesh.URL, http.StatusSeeOther)
}

func stripeCheckout(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	price := r.FormValue("price")
	if price == "" {
		panic("missing price")
	}
	u, _ := userFrom(ctx)
	var email, customerID *string
	// var customerID *string
	if u.CustomerID != "" {
		customerID = stripe.String(u.CustomerID)
	} else {
		email = stripe.String(u.Email)
	}
	var subparam *stripe.CheckoutSessionSubscriptionDataParams
	if !u.TrialOver && u.TimeRemaining() > (2*time.Hour*24)+(10*time.Minute) {
		subparam = &stripe.CheckoutSessionSubscriptionDataParams{
			TrialEnd: stripe.Int64(u.PlanExpire.UTC().Unix()),
		}
	}
	resp, err := checkoutsession.New(&stripe.CheckoutSessionParams{
		SuccessURL:         stripe.String(fmt.Sprintf("https://%s/buy/success?session_id={CHECKOUT_SESSION_ID}", Domain)),
		CancelURL:          stripe.String(fmt.Sprintf("https://%s/buy/?cancel", Domain)),
		Mode:               stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		PaymentMethodTypes: []*string{stripe.String("card")},

		ClientReferenceID: stripe.String(fmt.Sprintf("%d", u.ID)),
		CustomerEmail:     email,
		Customer:          customerID,

		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(price),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: subparam,
	})
	if err != nil {
		panic(err)
	}

	data := struct {
		SessionID string
	}{
		SessionID: resp.ID,
	}

	r.Header.Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}

func stripeCheckoutResult(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		panic("no session ID")
	}

	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("subscription")
	params.AddExpand("customer.subscriptions")
	sesh, err := checkoutsession.Get(sessionID, params)
	if err != nil {
		panic(err)
	}

	switch sesh.PaymentStatus {
	case stripe.CheckoutSessionPaymentStatusNoPaymentRequired, stripe.CheckoutSessionPaymentStatusPaid:
		if sesh.Subscription != nil {
			u, err := reconcileSub(ctx, sesh.Subscription)
			if err != nil {
				panic(err)
			}
			data := struct {
				User   tube.User
				Status string
			}{
				User:   u,
				Status: string(sesh.PaymentStatus),
			}
			if err := getTemplate(ctx, "checkout").Execute(w, data); err != nil {
				panic(err)
			}
			return
		}
	case stripe.CheckoutSessionPaymentStatusUnpaid:
		log.Println("checkout failure")
		spew.Dump(sesh)

		data := struct {
			Status string
		}{
			Status: string(sesh.PaymentStatus),
		}
		if err := getTemplate(ctx, "checkout-unpaid").Execute(w, data); err != nil {
			panic(err)
		}
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func stripeWebhook(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	event, err := webhook.ConstructEvent(data, r.Header.Get("Stripe-Signature"), stripeSigSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error verifying webhook signature: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Println("========")
	log.Println("got stripe webhook:", event.Type)
	log.Println(string(event.Data.Raw))

	switch event.Type {
	case "checkout.session.completed":
		var sesh *stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sesh); err != nil {
			panic(err)
		}
		uid, err := strconv.Atoi(sesh.ClientReferenceID)
		if err != nil {
			panic(err)
		}
		u, err := tube.GetUser(ctx, uid)
		if err != nil {
			panic(err)
		}
		if u.CustomerID != sesh.Customer.ID {
			log.Println("set customerID for: ", u.ID, sesh.Customer.ID, "; old:", u.CustomerID)
			if err := u.SetCustomerID(ctx, sesh.Customer.ID); err != nil {
				panic(err)
			}
		}
	case "customer.subscription.trial_will_end":
		// TODO
	case "invoice.paid":
		var invoice *stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			panic(err)
		}

		u, err := getUserByCustomer(ctx, invoice.Customer)
		if err != nil {
			panic(err)
		}

		item := invoice.Lines.Data[0]
		expires := time.Unix(item.Period.End, 0)
		plan, err := getPlanByProdID(item.Price.Product.ID)
		if err != nil {
			panic(err)
		}
		// update expire date
		canceled := invoice.Subscription != nil && (invoice.Subscription.CancelAt > 0 && invoice.Subscription.CancelAtPeriodEnd)
		if err := u.SetPlan(ctx, plan.Kind, tube.PlanStatusActive, expires, canceled); err != nil {
			panic(err)
		}
	case /*"customer.subscription.created", */ "customer.subscription.updated", "customer.subscription.deleted":
		var sub *stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			panic(err)
		}
		if _, err := reconcileSub(ctx, sub); err != nil {
			panic(err)
		}
	default:
		return
	}
}

func reconcileSub(ctx context.Context, sub *stripe.Subscription) (tube.User, error) {
	if len(sub.Items.Data) == 0 {
		return tube.User{}, fmt.Errorf("no item data")
	}

	u, err := getUserByCustomer(ctx, sub.Customer)
	if err != nil {
		return tube.User{}, err
	}
	if u.CustomerID == "" {
		if err := u.SetCustomerID(ctx, sub.Customer.ID); err != nil {
			return tube.User{}, err
		}
	}
	item := sub.Items.Data[0]
	log.Println("prod ID", item)
	plan, err := getPlanByProdID(item.Price.Product.ID)
	if err != nil {
		panic(err)
	}
	expires := time.Unix(sub.CurrentPeriodEnd, 0)
	canceled := sub.CancelAt > 0 && sub.CancelAtPeriodEnd
	log.Println("update sub for", u.ID, "to", plan, "expire", expires, "canceled?", canceled)
	err = u.SetPlan(ctx, plan.Kind, tube.PlanStatus(sub.Status), expires, canceled)
	return u, err
}

func getStripePrices(plans []tube.Plan) (map[tube.PlanKind]*stripe.Price, error) {
	prices := make(map[tube.PlanKind]*stripe.Price, len(plans))
	iter := stripeprice.List(&stripe.PriceListParams{
		Active: stripe.Bool(true),
	})
	for iter.Next() {
		price := iter.Price()
		fmt.Println("meta", price)
		for _, plan := range plans {
			if plan.PriceID == price.ID {
				prices[plan.Kind] = price
				break
			}
		}
	}
	return prices, iter.Err()
}

func getUserByCustomer(ctx context.Context, c *stripe.Customer) (tube.User, error) {
	if acc, ok := c.Metadata["account"]; ok {
		uid, err := strconv.Atoi(acc)
		if err != nil {
			return tube.User{}, fmt.Errorf("invalid account metadata: %s acc; %w", acc, err)
		}
		u, err := tube.GetUser(ctx, uid)
		if err == nil {
			return u, nil
		}
		log.Println("getUserByCustomer error1:", err)
	}

	if u, err := tube.GetUserByCustomerID(ctx, c.ID); err == nil {
		return u, nil
	} else {
		log.Println("getUserByCustomer error2:", err)
	}

	return tube.GetUserByEmail(ctx, c.Email)
}

func getPlanByProdID(prodID string) (tube.Plan, error) {
	prod, err := product.Get(prodID, nil)
	if err != nil {
		return tube.Plan{}, err
	}
	kind := prod.Metadata["kind"]
	if kind == "" {
		return tube.Plan{}, fmt.Errorf("missing kind: %v", prod)
	}
	return tube.GetPlan(tube.PlanKind(kind)), nil
}

func getCustomer(id string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{}
	params.AddExpand("subscriptions")
	cust, err := customer.Get(id, params)
	return cust, err
}

func ensureCustomer(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	if u, ok := userFrom(ctx); ok {
		if err := ensureStripeCustomer(ctx, &u); err != nil {
			panic(err)
		}
		ctx = withUser(ctx, u)
		return ctx
	}
	return ctx
}

func ensureStripeCustomer(ctx context.Context, u *tube.User) error {
	if u.CustomerID != "" {
		return nil
	}
	params := &stripe.CustomerParams{
		Email: stripe.String(u.Email),
	}
	params.AddMetadata("account", strconv.Itoa(u.ID))
	c, err := customer.New(params)
	if err != nil {
		return err
	}
	if err := u.SetCustomerID(ctx, c.ID); err != nil {
		return err
	}
	return nil
}

func formatCurrency(amt int64, currency stripe.Currency) string {
	dec := float64(amt) / 100.0
	switch currency {
	case stripe.CurrencyUSD:
		if amt%100 == 0 {
			return fmt.Sprintf("$%g USD", dec)
		} else {
			return fmt.Sprintf("$%.2f USD", dec)
		}
	case stripe.CurrencyJPY:
		return fmt.Sprintf("¥%d", amt)
	}
	return fmt.Sprintf("%g %s", dec, strings.ToUpper(string(currency)))
}
