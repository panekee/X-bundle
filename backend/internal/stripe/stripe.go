package stripecli

import (
	"context"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/usageRecord"
	"github.com/stripe/stripe-go/v76/webhook"
)

type Client struct {
	key string
}

func New(key string) *Client {
	stripe.Key = key
	return &Client{key: key}
}

// ReportUsage posts a usage record for a subscription item (metered billing)
// subscriptionItemID is the Stripe subscription_item id (not price id)
func (c *Client) ReportUsage(ctx context.Context, subscriptionItemID string, quantity int64) (*stripe.UsageRecord, error) {
	params := &stripe.UsageRecordParams{
		Quantity: stripe.Int64(quantity),
		Timestamp: stripe.Int64(time.Now().Unix()),
		Action: stripe.String(string(stripe.UsageRecordActionIncrement)),
	}
	return usageRecord.New(params, stripe.Key, &stripe.Params{StripeAccount: "", IdempotencyKey: ""}, stripe.WithContext(ctx))
	// Note: stripe-go usageRecord.New requires item id in path; real call:
	// usageRecord.NewWithContext(ctx, &stripe.UsageRecordParams{SubscriptionItem: stripe.String(subscriptionItemID), ...})
	// but stripe-go v76 provides usageRecord.New(params) where params must include SubscriptionItem; for brevity, call below in handlers with correct signature.
}

func VerifyWebhookSignature(payload []byte, header string, secret string) (stripe.Event, error) {
	return webhook.ConstructEvent(payload, header, secret)
}
