package stagingv2

import (
	"context"
	"errors"
	"testing"

	domainv2 "github.com/api-control/internal/domain/v2"
	"golang.org/x/crypto/bcrypt"
)

type userLookupFake struct {
	user *domainv2.User
	err  error
}

func (lookup userLookupFake) FindActiveUserByLogin(context.Context, string) (*domainv2.User, error) {
	return lookup.user, lookup.err
}

func TestStagingAuthenticatorUsesV2PasswordHash(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("strong-password-123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	authenticator, err := NewAuthenticator(userLookupFake{user: &domainv2.User{ID: 7, PasswordHash: string(hash), Active: true}})
	if err != nil {
		t.Fatal(err)
	}
	user, err := authenticator.Authenticate(context.Background(), "admin", "strong-password-123")
	if err != nil || user.ID != 7 {
		t.Fatalf("Authenticate user = %+v, error = %v", user, err)
	}
	if _, err := authenticator.Authenticate(context.Background(), "admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
}

func TestStagingNormalizersAreDeterministic(t *testing.T) {
	if got := normalizeLogin(" ADMIN@Example.COM "); got != "admin@example.com" {
		t.Fatalf("normalizeLogin = %q", got)
	}
	if got := normalizeDocument("123.456.789-00"); got != "12345678900" {
		t.Fatalf("normalizeDocument = %q", got)
	}
}
