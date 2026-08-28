package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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
		Expand: []*string{new("data.line_items"), new("data.customer")},
		Status: new("complete"),
	})
	for session, err := range sessions {
		if err != nil {
			return fmt.Errorf("list checkout sessions: %v", err)
		}
		fmt.Println("- id:          " + session.PaymentIntent.ID)
		fmt.Println("  email:       " + session.Customer.Email)
		fmt.Println("  created:     " + time.Unix(session.Created, 0).String())
		fmt.Printf("  amount:      %d %s\n", session.AmountTotal/100, session.Currency)
		fmt.Println("  items:")
		for _, item := range session.LineItems.Data {
			fmt.Println("    - name:    " + item.Description)
			if item.Price.LookupKey != "" {
				fmt.Println("      variant: " + item.Price.LookupKey)
			}
		}
		fmt.Println()
	}
	return nil
}
