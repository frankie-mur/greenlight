package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"time"

	"github.com/frankie-mur/greenlight/internal/validator"
)

// Scopes for our tokens
const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
)

type TokenModel struct {
	DB *sql.DB
}

type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

// Method wraps token generation and insertion into database
func (m TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	//Generate token
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}
	//Insert into database and return
	err = m.Insert(token)
	return token, err
}

func (m TokenModel) Insert(token *Token) error {
	query := `INSERT INTO tokens(user_id, expiry, hash, scope)
			  VALUES ($1, $2, $3, $4)
			  RETURNING user_id`

	args := []any{token.UserID, token.Expiry, token.Hash, token.Scope}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, args...)

	if err != nil {
		return err
	}

	return nil
}

func (m TokenModel) DeleteAllForUser(scope string, userID int64) error {
	query := `DELETE FROM tokens WHERE scope = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, scope, userID)
	return err
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}
	//Create our token
	randomBytes := make([]byte, 16)
	//Fill random bytes into our slice
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	//Store the token, so send to user in email
	//Note: We encode with no padding
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	//Generate the tokn hash
	hash := sha256.Sum256([]byte(token.Plaintext))
	//Store into token strut as slice
	token.Hash = hash[:]

	return token, nil
}

// Check that the plaintext token has been provided and is exactly 26 bytes long.
func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}
