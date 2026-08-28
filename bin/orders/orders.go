package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
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
		Expand: []*string{
			new("data.line_items"),
			new("data.customer"),
			new("data.payment_intent.shipping.address"),
		},
		Status: new("complete"),
	})

	productsList := client.V1Products.List(ctx, nil)
	products := map[string]*stripe.Product{}
	productSlugs := map[string]*stripe.Product{}
	for product, err := range productsList {
		if err != nil {
			return fmt.Errorf("list products: %v", err)
		}
		products[product.ID] = product
		productSlugs[product.Metadata["slug"]] = product
	}

	pricesList := client.V1Prices.List(ctx, nil)
	prices := map[string]*stripe.Price{}
	for price, err := range pricesList {
		if err != nil {
			return fmt.Errorf("list prices: %v", err)
		}
		prices[price.ID] = price
	}

	for session, err := range sessions {
		if err != nil {
			return fmt.Errorf("list checkout sessions: %v", err)
		}
		fmt.Println("- id:               \033[94m" + session.PaymentIntent.ID + "\033[0m")
		fmt.Println("  email:            " + session.Customer.Email)
		fmt.Println("  created:          " + time.Unix(session.Created, 0).String())
		fmt.Printf("  amount:           %d %s\n", session.AmountTotal/100, session.Currency)
		fmt.Println("  address:")
		fmt.Println("    city:           " + session.PaymentIntent.Shipping.Address.City)
		fmt.Println("    country:        " + session.PaymentIntent.Shipping.Address.Country)
		fmt.Println("    line1:          " + session.PaymentIntent.Shipping.Address.Line1)
		if session.PaymentIntent.Shipping.Address.Line2 != "" {
			fmt.Println("    line2:          " + session.PaymentIntent.Shipping.Address.Line2)
		}
		fmt.Println("    zip:            " + session.PaymentIntent.Shipping.Address.PostalCode)
		if session.PaymentIntent.Shipping.Address.State != "" {
			fmt.Println("    state:          " + session.PaymentIntent.Shipping.Address.State)
		}

		// Items in the order.
		fmt.Println("  items:")
		for _, item := range session.LineItems.Data {
			fmt.Println("    - name:         \033[92m" + item.Description + "\033[0m")
			if item.Quantity != 1 {
				fmt.Printf("    - qty:          %d\n", item.Quantity)
			}
			if item.Price.LookupKey != "" {
				fmt.Println("      variant:      " + item.Price.LookupKey)
			}
			product := products[item.Price.Product.ID]
			if product != nil {
				if product.Metadata["products"] != "" {
					fmt.Println("      bundle:")
					slugs := strings.SplitSeq(product.Metadata["products"], ",")
					for slug := range slugs {
						if slug != "" {
							subProduct := productSlugs[slug]
							fmt.Println("        - name:     \033[95m" + subProduct.Name + "\033[0m")
						}
					}
				}
			}
			if item.Metadata["variants"] != "" {
				fmt.Println("      variants:")
				for variantID := range strings.SplitSeq(item.Metadata["variants"], ",") {
					price := prices[variantID]
					if price != nil {
						fmt.Println("        - \033[93m" + price.LookupKey + "\033[0m")
					}
				}
			}
		}

		fmt.Println()
	}
	return nil
}
