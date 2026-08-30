package lib

import (
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type Goal struct {
	// How much money we received.
	Reached int64 `json:"reached"`
	// How much money we need to start the production.
	Total int64 `json:"total"`
}

// GET /goal
//
// Get our financial goal and how much of it is reached.
func getGoal(r josh.Req) josh.Resp {
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	balance, err := client.V1Balance.Retrieve(r.Context(), nil)
	if err != nil {
		return ServerErrorR(r, "failed to fetch balance", err)
	}
	return josh.Ok(josh.Data[Goal]{
		ID:   "1",
		Type: "goal",
		Attributes: Goal{
			Reached: balance.Available[0].Amount + balance.Pending[0].Amount,
			Total:   1_000_000 * EUR,
		},
	})
}
