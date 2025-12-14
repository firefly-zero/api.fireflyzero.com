package lib

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/jackc/pgx/v5"
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type Order struct {
	Paid bool `json:"paid"`
}

type CheckoutReq struct {
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

type CheckoutResp struct {
	RedirectURL string `json:"redirect_url"`
}

type ConfirmReq struct {
	SessionID string `json:"session_id"`
}

func withOrder(h josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		queries := josh.Must(josh.GetSingleton[*db.Queries](r))
		myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

		orderID, errResp := josh.GetID[dbtypes.OrderID](r, "order")
		if errResp != nil {
			return josh.BadRequest(*errResp)
		}
		params := db.GetOrderParams{
			User: myID,
			ID:   orderID,
		}
		order, err := queries.GetOrder(r.Context(), params)
		if err == pgx.ErrNoRows {
			return josh.NotFound(josh.Error{
				Detail: "order not found",
				Code:   "order-404",
			})
		}
		if err != nil {
			return ServerErrorR(r, "failed to get order", err)
		}
		r = josh.Must(josh.WithSingleton(r, order))
		return h(r)
	}
}

func checkoutOrder(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

	body, err := schemas.ReadBody[CheckoutReq](r, "checkout", "_req")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	order, err := queries.GetDraftOrder(r.Context(), myID)
	if err != nil {
		return ServerErrorR(r, "failed to get order", err)
	}
	if order.Status != db.OrderStatusDraft {
		return josh.BadRequest(josh.Error{
			Detail: "you can checkout only draft order",
		})
	}
	items, err := queries.ListOrderItems(r.Context(), order.ID)
	if err != nil {
		return ServerErrorR(r, "failed to list order items", err)
	}

	lineItems := make([]*stripe.CheckoutSessionCreateLineItemParams, 0, len(items))
	for _, item := range items {
		product, err := queries.GetProduct(r.Context(), item.Product)
		if err != nil {
			return ServerErrorR(r, "failed to get product info", err)
		}
		lineItem := stripe.CheckoutSessionCreateLineItemParams{
			Quantity: ptr(int64(item.Quantity)),
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:    ptr(string(stripe.CurrencyEUR)),
				ProductData: ptr(makeProductData(product)),
				TaxBehavior: new(string),
				UnitAmount:  ptr(int64(item.RetailPrice)),
			},
		}
		lineItems = append(lineItems, &lineItem)
	}

	// From this point on, the request cannot be canceled.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()

	customerID, err := ensureCustomer(ctx)
	if err != nil {
		return ServerErrorR(r, "failed to get Stripe customer", err)
	}

	orderIDStr := formatID(order.ID)
	params := stripe.CheckoutSessionCreateParams{
		Params: stripe.Params{
			IdempotencyKey: ptr("order-checkout-" + orderIDStr),
		},
		Mode:       ptr(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: ptr(formatOrderURL(attrs.SuccessURL, order.ID)),
		CancelURL:  ptr(formatOrderURL(attrs.CancelURL, order.ID)),
		Customer:   &customerID,
		LineItems:  lineItems,
		Metadata: map[string]string{
			"order_id": orderIDStr,
		},
		AllowPromotionCodes: ptr(false),
		AutomaticTax: &stripe.CheckoutSessionCreateAutomaticTaxParams{
			Enabled: ptr(true),
		},
		Currency: ptr(string(stripe.CurrencyEUR)),
	}
	session, err := client.V1CheckoutSessions.Create(ctx, &params)
	if err != nil {
		return ServerErrorR(r, "failed to create checkout session", err)
	}

	_, err = queries.SetOrderStatus(ctx, db.SetOrderStatusParams{
		Status: db.OrderStatusPending,
		ID:     order.ID,
		User:   myID,
	})
	if err != nil {
		return ServerErrorR(r, "failed to update order status", err)
	}

	return josh.Ok(josh.Data[CheckoutResp]{
		ID:   orderIDStr,
		Type: "checkout",
		Attributes: CheckoutResp{
			RedirectURL: session.URL,
		},
	})
}

func confirmOrder(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))
	order := josh.Must(josh.GetSingleton[db.Order](r))

	body, err := schemas.ReadBody[ConfirmReq](r, "confirm", "_req")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	session, err := client.V1CheckoutSessions.Retrieve(r.Context(), attrs.SessionID, nil)
	if err != nil || session == nil {
		return ServerErrorR(r, "failed to get checkout session info", err)
	}
	if session.PaymentStatus == stripe.CheckoutSessionPaymentStatusUnpaid {
		return josh.BadRequest(josh.Error{
			Detail: "the order is not paid",
		})
	}
	if session.Metadata["order_id"] != formatID(order.ID) {
		return josh.BadRequest(josh.Error{
			Detail: "the checkout session was created for a different order",
		})
	}

	order, err = queries.SetOrderPaid(r.Context(), db.SetOrderPaidParams{
		ID:   order.ID,
		User: myID,
	})
	if err != nil {
		return ServerErrorR(r, "failed to mark the order as paid", err)
	}

	// TODO(@orsinium): auto-fullfill digital orders (apps, donations).

	return josh.Ok(formatOrder(order))
}

// Create Stripe customer for the user if it doesn't exist.
//
// Returns the Stripe customer ID.
func ensureCustomer(ctx context.Context) (string, error) {
	queries := josh.Must(josh.CGetSingleton[*db.Queries](ctx))
	client := josh.Must(josh.CGetSingleton[*stripe.Client](ctx))
	myID := josh.Must(josh.CGetSingleton[dbtypes.UserID](ctx))

	me, err := queries.GetUserByID(ctx, myID)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}
	if me.StripeID != nil {
		return *me.StripeID, nil
	}

	customer, err := client.V1Customers.Create(ctx, &stripe.CustomerCreateParams{
		Params: stripe.Params{
			IdempotencyKey: ptr("create-customer-" + formatID(me.ID)),
		},
		Email: &me.Email,
	})
	if err != nil {
		return "", fmt.Errorf("create customer: %w", err)
	}

	_, err = queries.UpdateUser(ctx, db.UpdateUserParams{
		StripeID: &customer.ID,
		ID:       myID,
	})
	if err != nil {
		return "", fmt.Errorf("update user: %w", err)
	}

	return customer.ID, nil
}

func formatOrderURL(template string, id dbtypes.OrderID) string {
	return strings.ReplaceAll(template, "{order}", formatID(id))
}

func makeProductData(p db.Product) stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams {
	return stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
		Metadata: map[string]string{
			"product_slug": string(p.Slug),
		},
		Name:    &p.Name,
		TaxCode: ptr(getTaxCode(p.Slug)),
	}
}

func formatOrder(order db.Order) josh.Data[Order] {
	return josh.Data[Order]{
		ID:   formatID(order.ID),
		Type: "order",
		Attributes: Order{
			Paid: order.Paid,
		},
	}
}

// https://docs.stripe.com/tax/tax-codes
func getTaxCode(slug dbtypes.ProductSlug) string {
	isApp := strings.Contains(string(slug), ".")
	if isApp {
		// Video Games - downloaded - non subscription - with permanent rights
		return "txcd_10201000"
	}
	switch slug {
	case "donation":
		// Cash Donation
		return "txcd_90000001"
	case "console", "device", "firefly", "firefly-zero":
		// Video Gaming Console - Portable
		return "txcd_34022001"
	default:
		// Clothing & Footwear
		return "txcd_30011000"
	}
}

func ptr[T any](v T) *T {
	return &v
}
