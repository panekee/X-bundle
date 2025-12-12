package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v76"
	ustr "github.com/stripe/stripe-go/v76/usagerecord"

	"microapi/internal/auth"
	"microapi/internal/ratelimit"
	"microapi/internal/stripecli"
)

type Handler struct {
	db      *pgxpool.Pool
	rl      *ratelimit.RedisLimiter
	auth    *auth.Service
	stripe  *stripecli.Client
	adminKey string
}

// NewHandler wiring
func NewHandler(db *pgxpool.Pool, rl *ratelimit.RedisLimiter, auth *auth.Service, stripe *stripecli.Client, adminKey string) *Handler {
	return &Handler{db: db, rl: rl, auth: auth, stripe: stripe, adminKey: adminKey}
}

// Logging middleware
func (h *Handler) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		start := time.Now()
		next.ServeHTTP(w,r)
		log.Info().Str("method", r.Method).Str("path", r.URL.Path).Dur("dur", time.Since(start)).Msg("")
	})
}

// Admin auth middleware (simple header)
func (h *Handler) AdminAuthMiddleware(adminKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
			k := r.Header.Get("Authorization")
			if k == "" { http.Error(w,"unauthorized", http.StatusUnauthorized); return }
			if k != "Bearer "+adminKey && k != adminKey { http.Error(w,"unauthorized", http.StatusUnauthorized); return }
			next.ServeHTTP(w,r)
		})
	}
}

// CreateTenant: { "name": "ACME", "stripe_customer_id": "cus_xxx" }
func (h *Handler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name string `json:"name"`; StripeCustomerID string `json:"stripe_customer_id"` }
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w,"bad request",400); return }
	var id string
	err := h.db.QueryRow(context.Background(), "INSERT INTO tenants(name, stripe_customer_id) VALUES($1,$2) RETURNING id", in.Name, nullEmpty(in.StripeCustomerID)).Scan(&id)
	if err != nil { http.Error(w,"db error",500); return }
	jsonResponse(w, map[string]string{"id": id})
}

// CreateAPIKey: { "tenant_id": "uuid", "expires_at": "2026-01-01T00:00:00Z" }
// Returns {"api_key":"PLAINTEXT_KEY","id":"..."}
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var in struct{ TenantID string `json:"tenant_id"`; ExpiresAt *time.Time `json:"expires_at,omitempty"` }
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.TenantID=="" { http.Error(w,"bad request",400); return }
	plain := genKey(40)
	if err := h.auth.CreateKey(r.Context(), in.TenantID, plain); err != nil { http.Error(w,"db error",500); return }
	jsonResponse(w, map[string]string{"api_key": plain})
}

// AssignStripeCustomer maps tenant -> stripe customer id
func (h *Handler) AssignStripeCustomer(w http.ResponseWriter, r *http.Request) {
	var in struct{ TenantID string `json:"tenant_id"`; StripeCustomerID string `json:"stripe_customer_id"` }
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.TenantID=="" || in.StripeCustomerID=="" { http.Error(w,"bad request",400); return }
	_, err := h.db.Exec(context.Background(), "UPDATE tenants SET stripe_customer_id=$1 WHERE id=$2", in.StripeCustomerID, in.TenantID)
	if err != nil { http.Error(w,"db error",500); return }
	jsonResponse(w, map[string]string{"status":"ok"})
}

// GetUsage returns aggregated usage for tenant (simple)
func (h *Handler) GetUsage(w http.ResponseWriter, r *http.Request) {
	tid := chi.URLParam(r, "tenant_id")
	if tid=="" { http.Error(w,"missing tenant",400); return }
	var count int64
	err := h.db.QueryRow(context.Background(), "SELECT COALESCE(SUM(qty),0) FROM usage_records WHERE tenant_id=$1 AND ts > now() - interval '30 days'", tid).Scan(&count)
	if err != nil { http.Error(w,"db error",500); return }
	jsonResponse(w, map[string]any{"tenant_id": tid, "requests_last_30d": count})
}

func (h *Handler) MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request){
		jsonResponse(w, map[string]any{"uptime_s": 0, "status":"ok"})
	}
}

// APIHandler generic handler for endpoints
func (h *Handler) APIHandler(endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// auth
		key := r.Header.Get("Authorization")
		if key == "" { key = r.Header.Get("x-api-key") }
		tenantID, err := h.auth.ValidateKey(r.Context(), key)
		if err != nil { httpError(w, "unauthorized", 401); return }

		// rate limit: per-tenant bucket. Configurable values (example)
		capacity := 100 // capacity tokens
		refill := 10.0  // tokens per second
		ok, _, err := h.rl.Allow(r.Context(), "rl:"+tenantID+":"+endpoint, capacity, refill, 1)
		if err != nil { httpError(w, "rate error", 500); return }
		if !ok { httpError(w, "rate_limited", 429); return }

		// record usage in DB (async minimal)
		go func() {
			_, _ = h.db.Exec(context.Background(), "INSERT INTO usage_records(tenant_id, endpoint, qty) VALUES($1,$2,$3)", tenantID, endpoint, 1)
		}()

		// business logic minimal deterministic responses (replace for richer logic)
		var payload map[string]any
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)

		var result any
		switch endpoint {
		case "transform":
			result = map[string]any{"id": genUUID(), "result": map[string]string{"text": toLowerFromPayload(payload)}}
		case "summarize":
			result = map[string]any{"id": genUUID(), "result": map[string]string{"summary": summarizeSimple(payload)}}
		case "clean":
			result = map[string]any{"id": genUUID(), "result": map[string]string{"text": stripPunctFromPayload(payload)}}
		case "generate":
			result = map[string]any{"id": genUUID(), "result": "generated"}
		default:
			result = map[string]any{"id": genUUID(), "ok": true}
		}

		// report usage to Stripe if tenant has a stripe_customer_id and a subscription item assigned
		// For simplicity we expect an "stripe_subscription_item_id" column in tenants or pass via request header X-Stripe-Subscription-Item
		subItem := r.Header.Get("X-Stripe-Subscription-Item")
		if subItem != "" {
			// create usage record (increment by 1)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err := ustr.New(&stripe.UsageRecordParams{
					Quantity:         stripe.Int64(1),
					Timestamp:        stripe.Int64(time.Now().Unix()),
					SubscriptionItem: stripe.String(subItem),
					Action:           stripe.String(string(stripe.UsageRecordActionIncrement)),
				})
				if err != nil {
					log.Error().Err(err).Str("tenant", tenantID).Msg("stripe usage record failed")
				}
			}()
		}

		jsonResponse(w, map[string]any{"id": genUUID(), "result": result})
	}
}

// Stripe webhook handler (verifies signature)
func (h *Handler) StripeWebhookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := "" // read from env in production
		if s := r.Header.Get("Stripe-Signature"); s != "" { secret = s } // placeholder
		payload, _ := io.ReadAll(r.Body)
		// Verify signature
		ev, err := stripecli.VerifyWebhookSignature(payload, r.Header.Get("Stripe-Signature"), secret)
		if err != nil {
			httpError(w, "invalid webhook signature", 400); return
		}
		// handle event types
		switch ev.Type {
		case "invoice.payment_succeeded":
			// handle invoice success if needed
		case "customer.subscription.created":
			// handle subs
		}
		w.WriteHeader(200)
	}
}

// helpers
func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type","application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func httpError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func genKey(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
func genUUID() string {
	b := make([]byte, 16)
	rand.Read(b); b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b)
}

// simple payload helpers (rudimentary but deterministic)
func toLowerFromPayload(p map[string]any) string {
	if p==nil { return "" }
	if d, ok := p["data"].(map[string]any); ok {
		if t, ok := d["text"].(string); ok {
			b := []rune(t)
			for i := range b { if b[i]>='A' && b[i]<='Z' { b[i]+=32 } }
			return string(b)
		}
	}
	return ""
}
func summarizeSimple(p map[string]any) string {
	s := toLowerFromPayload(p)
	if len(s) > 120 { return s[:117] + "..." }
	return s
}
func stripPunctFromPayload(p map[string]any) string {
	s := toLowerFromPayload(p)
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r>='a'&&r<='z') || (r>='0'&&r<='9') || r==' ' { out = append(out, r) }
	}
	return string(out)
}

func nullEmpty(s string) *string {
	if s=="" { return nil }
	return &s
}
