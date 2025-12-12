Micro API backend — Quickstart

1) Configure env (use .env file):
   DATABASE_URL, REDIS_ADDR, ADMIN_API_KEY, STRIPE_API_KEY, STRIPE_WEBHOOK_SECRET

2) Apply migrations:
   psql $DATABASE_URL -f migrations/001_init.sql

3) Build & run:
   go build -o microapi ./cmd/server
   PORT=8080 DATABASE_URL=... REDIS_ADDR=... ADMIN_API_KEY=... STRIPE_API_KEY=... ./microapi

4) Admin flow:
   - Create tenant via POST /admin/tenants (use admin header)
   - Create API key for tenant via POST /admin/api_keys -> returns plaintext key
   - Create Stripe customer/subscription and assign subscription item id to tenant (via POST /admin/assign_stripe or update DB directly)
   - Clients call /v1/* endpoints with header x-api-key: <key>

5) Stripe:
   - Use meter-based pricing. For each API call the service posts a UsageRecord for the tenant's subscription_item_id.
   - Configure a webhook endpoint /webhook/stripe in Stripe with STRIPE_WEBHOOK_SECRET and set it in env.

Security checklist before selling:
- Replace any demo logic; use constant-time comparison for keys (sha256 compare).
- Enforce TLS at edge; never expose DB/Redis to public.
- Use Vault / Secrets Manager for production secrets.
- Run security scans and pen testing.

