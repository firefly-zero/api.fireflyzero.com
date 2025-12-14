package lib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

func stripeWebhook(r josh.Req) josh.Resp {
	config := josh.Must(josh.GetSingleton[Config](r))
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	// https://docs.stripe.com/webhooks
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return ServerErrorR(r, "failed to read request body", err)
	}
	event, err := stripe.ConstructEvent(body, r.Header.Get("Stripe-Signature"), config.StripeSecret)
	if err != nil {
		return josh.BadRequest(josh.Error{
			Detail: err.Error(),
		})
	}

	// https://docs.stripe.com/checkout/fulfillment
	completed := event.Type == stripe.EventTypeCheckoutSessionCompleted
	paid := event.Type == stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded
	if completed || paid {
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			return ServerErrorR(r, "failed to parse checkout session", err)
		}
		err = fulfillCheckout(queries, session)
		if err != nil {
			return ServerErrorR(r, "failed to fulfill checkout session", err)
		}
	}

	return josh.Ok(struct{}{})
}

func fulfillCheckout(queries *db.Queries, session stripe.CheckoutSession) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if session.PaymentStatus == stripe.CheckoutSessionPaymentStatusUnpaid {
		return errors.New("the order is not paid")
	}
	if session.Metadata["order_id"] == "" {
		return errors.New("session metadata has no order_id")
	}
	orderID := parseID[dbtypes.OrderID](session.Metadata["order_id"])
	_, err := queries.SetOrderPaid(ctx, orderID)
	if err != nil {
		return fmt.Errorf("mark order as paid: %v", err)
	}

	// TODO(@orsinium): auto-fullfill digital orders (apps, donations).
	return nil
}
