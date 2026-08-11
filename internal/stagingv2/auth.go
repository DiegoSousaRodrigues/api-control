package stagingv2

import (
	"context"
	"errors"

	domainv2 "github.com/api-control/internal/domain/v2"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type UserLookup interface {
	FindActiveUserByLogin(context.Context, string) (*domainv2.User, error)
}

type Authenticator struct {
	users UserLookup
}

func NewAuthenticator(users UserLookup) (*Authenticator, error) {
	if users == nil {
		return nil, errors.New("staging v2 user lookup is required")
	}
	return &Authenticator{users: users}, nil
}

func (authenticator *Authenticator) Authenticate(ctx context.Context, login, password string) (*domainv2.User, error) {
	user, err := authenticator.users.FindActiveUserByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}
