package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/firefly-zero/api.fireflyzero.com/lib"
	"github.com/stripe/stripe-go/v84"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config := lib.Config{}
	err := config.ParseEnv(os.Environ())
	if err != nil {
		return fmt.Errorf("read config: %v", err)
	}
	client := stripe.NewClient(config.StripeKey)
	ctx := context.Background()

	sessions := client.V1CheckoutSessions.List(ctx, &stripe.CheckoutSessionListParams{
		Expand: []*string{new("data.line_items")},
		Status: new("complete"),
	})

	counts := make(map[string]int)
	for session, err := range sessions {
		if err != nil {
			return fmt.Errorf("list checkout sessions: %v", err)
		}
		for _, item := range session.LineItems.Data {
			counts[item.Price.ID]++
		}
	}

	pricesList, err := loadStripeList(client.V1Prices.List(ctx, &stripe.PriceListParams{
		Expand: []*string{new("data.product")},
	}))
	if err != nil {
		return fmt.Errorf("list prices: %v", err)
	}
	slices.SortStableFunc(pricesList, func(a, b *stripe.Price) int {
		bCount := counts[b.ID]
		aCount := counts[a.ID]
		if aCount == bCount {
			return cmp.Compare(a.Product.Name, b.Product.Name)
		}
		return -cmp.Compare(aCount, bCount)
	})

	for _, price := range pricesList {
		count := counts[price.ID]
		name := price.Product.Name
		if price.LookupKey != "" {
			name += " (" + price.LookupKey + ")"
		}
		fmt.Printf("%-40s %5d\n", name, count)
	}

	return nil
}

func loadStripeList[T any](seq stripe.Seq2[T, error]) ([]T, error) {
	res := make([]T, 0)
	for val, err := range seq {
		if err != nil {
			return nil, err
		}
		res = append(res, val)
	}
	return res, nil
}
