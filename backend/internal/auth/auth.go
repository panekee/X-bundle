package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func HashKey(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(h[:])
}

// ValidateKey returns tenant_id if key valid
func (s *Service) ValidateKey(ctx context.Context, keyPlain string) (string, error) {
	if keyPlain == "" { return "", errors.New("missing key") }
	hash := HashKey(keyPlain)
	var tenantID string
	err := s.db.QueryRow(ctx, "SELECT tenant_id FROM api_keys WHERE key_hash=$1 AND active=TRUE", hash).Scan(&tenantID)
	if err != nil { return "", err }
	return tenantID, nil
}

// CreateKey stores hash and returns plaintext to caller (plaintext shown once)
func (s *Service) CreateKey(ctx context.Context, tenantID string, plain string) error {
	hash := HashKey(plain)
	_, err := s.db.Exec(ctx, "INSERT INTO api_keys(tenant_id, key_hash) VALUES($1,$2)", tenantID, hash)
	return err
}
