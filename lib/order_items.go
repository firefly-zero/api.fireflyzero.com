package lib

import (
	"strings"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/jackc/pgx/v5"
	"github.com/orsinium-labs/josh"
)

// https://docs.stripe.com/tax/tax-codes
const (
	// Video Gaming Console - Portable
	taxConsole = "txcd_34022001"
	// Cash Donation
	taxDonation = "txcd_90000001"
	// Video Games - downloaded - non subscription - with permanent rights
	taxGame = "txcd_10201000"
	// Clothing & Footwear
	taxShirt = "txcd_30011000"
)

type OrderItem struct {
	Name        string              `json:"name"`
	ProductSlug dbtypes.ProductSlug `json:"product_slug"`
	ReleaseID   *dbtypes.ReleaseID  `json:"release_id"`
	Quantity    int32               `json:"quantity"`
	RetailPrice *int32              `json:"retail_price"`
	Fulfilled   bool                `json:"fulfilled"`
}

// POST /order/items
//
// Add an item into the pending order.
func addOrderItem(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	body, err := schemas.ReadBody[OrderItem](r, "order_item", "_add")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	isApp := strings.Contains(string(attrs.ProductSlug), ".")
	if isApp && attrs.Quantity != 1 {
		return josh.BadRequest(josh.Error{
			Detail: "you can buy only one copy of an app",
		})
	}
	product, err := queries.GetProduct(r.Context(), attrs.ProductSlug)
	if err == pgx.ErrNoRows {
		_, err := queries.GetGroup(r.Context(), string(product.Slug))
		if err == nil {
			return josh.BadRequest(josh.Error{
				Detail: "group slug cannot be used as product slug",
			})
		}
		return josh.BadRequest(josh.Error{
			Detail: "product not found",
		})
	}
	if err != nil {
		return ServerErrorR(r, "failed to create order", err)
	}

	// TODO(@orsinium): Lock the column to prevent adding items to non-draft orders.
	order, err := ensureOrder(r)
	if err != nil {
		return ServerErrorR(r, "failed to create order", err)
	}

	price := product.RetailPrice
	if product.Slug == "donation" {
		if attrs.RetailPrice == nil {
			price = 5_00
		} else {
			price = *attrs.RetailPrice
		}
	}
	qty := attrs.Quantity
	// Quantity is not specified in the request for donations.
	if qty == 0 {
		qty = 1
	}
	item, err := queries.CreateOrderItem(r.Context(), db.CreateOrderItemParams{
		Order:       order.ID,
		Product:     attrs.ProductSlug,
		Release:     nil,
		Quantity:    qty,
		RetailPrice: price,
	})
	if err != nil {
		return ServerErrorR(r, "failed to add product to the order", err)
	}

	return josh.Created(formatOrderItem(item, product))
}

// GET /order/items
//
// List items in the draft order.
func listDraftOrderItems(r josh.Req) josh.Resp {
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	order, err := queries.GetDraftOrder(r.Context(), myID)
	if err == pgx.ErrNoRows {
		return josh.Ok([]any{})
	}
	if err != nil {
		return ServerErrorR(r, "failed to get order", err)
	}
	return listItemsInOrder(r, order)
}

// GET /order/{order}/items
//
// List items in the given order.
func listOrderItems(r josh.Req) josh.Resp {
	order := josh.Must(josh.GetSingleton[db.Order](r))

	if order.Status == db.OrderStatusDraft {
		return josh.BadRequest(josh.Error{
			Detail: "order is in draft",
		})
	}
	return listItemsInOrder(r, order)
}

func listItemsInOrder(r josh.Req, order db.Order) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	items, err := queries.ListOrderItems(r.Context(), order.ID)
	if err != nil {
		return ServerErrorR(r, "failed to get products in the order", err)
	}

	resps := make([]josh.Data[OrderItem], 0, len(items))
	for _, item := range items {
		product, err := queries.GetProduct(r.Context(), item.Product)
		if err != nil {
			// No return, it's ok'ish to return
			// the order item without product info.
			ServerErrorR(r, "cannot find product", err)
		}
		resps = append(resps, formatOrderItem(item, product))
	}
	return josh.Ok(resps)
}

// Get the draft order, creating it if it doesn't exist.
func ensureOrder(r josh.Req) (db.Order, error) {
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	order, err := queries.EnsureOrder(r.Context(), myID)
	if err != pgx.ErrNoRows {
		return order, err
	}
	return queries.GetDraftOrder(r.Context(), myID)
}

func formatOrderItem(item db.OrderItem, product db.Product) josh.Data[OrderItem] {
	return josh.Data[OrderItem]{
		ID:   formatID(item.ID),
		Type: "order_item",
		Attributes: OrderItem{
			Name:        product.Name,
			ProductSlug: item.Product,
			ReleaseID:   item.Release,
			Quantity:    item.Quantity,
			RetailPrice: &item.RetailPrice,
			Fulfilled:   item.Fulfilled,
		},
	}
}
