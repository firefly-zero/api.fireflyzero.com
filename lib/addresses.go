package lib

import (
	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

func getAddress(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

	me, err := queries.GetUserByID(r.Context(), myID)
	if err != nil {
		return ServerErrorR(r, "failed to get user info", err)
	}
	customer, err := client.V1Customers.Retrieve(r.Context(), me.StripeID, &stripe.CustomerRetrieveParams{})
	if err != nil {
		return ServerErrorR(r, "failed to update customer", err)
	}
	return josh.Ok(josh.Data[stripe.Address]{
		ID:         formatID(myID),
		Type:       "address",
		Attributes: *customer.Address,
	})
}

func setAddress(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

	body, err := schemas.ReadBody[stripe.AddressParams](r, "checkout", "_req")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	me, err := queries.GetUserByID(r.Context(), myID)
	if err != nil {
		return ServerErrorR(r, "failed to get user info", err)
	}
	customer, err := client.V1Customers.Update(r.Context(), me.StripeID, &stripe.CustomerUpdateParams{
		Address: &body.Attributes,
	})
	if err != nil {
		return ServerErrorR(r, "failed to update customer", err)
	}
	return josh.Ok(josh.Data[stripe.Address]{
		ID:         formatID(myID),
		Type:       "address",
		Attributes: *customer.Address,
	})
}
