package admin

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

type initialUserStoreFake struct {
	exists       bool
	createCalls  int
	login        string
	passwordHash string
}

func (store *initialUserStoreFake) LoginExists(context.Context, string) (bool, error) {
	return store.exists, nil
}

func (store *initialUserStoreFake) CreateInitialUser(_ context.Context, login, passwordHash string) error {
	store.createCalls++
	store.login = login
	store.passwordHash = passwordHash
	return nil
}

func factoryFor(store InitialUserStore) StoreFactory {
	return func(context.Context) (InitialUserStore, func() error, error) {
		return store, nil, nil
	}
}
func passwordSequence(values ...string) PasswordReader {
	index := 0
	return func() ([]byte, error) {
		if index >= len(values) {
			return nil, errors.New("unexpected password read")
		}
		value := []byte(values[index])
		index++
		return value, nil
	}
}

func TestCreateUserReadsAndConfirmsPasswordWithoutPrintingIt(t *testing.T) {
	store := &initialUserStoreFake{}
	output := &bytes.Buffer{}
	password := "strong-password-123"
	err := Run(context.Background(), []string{"create-user", "--login", " ADMIN@example.com "}, output, passwordSequence(password, password), factoryFor(store))
	if err != nil {
		t.Fatal(err)
	}
	if store.createCalls != 1 || store.login != "admin@example.com" {
		t.Fatalf("created user = calls %d login %q", store.createCalls, store.login)
	}
	if store.passwordHash == password || bcrypt.CompareHashAndPassword([]byte(store.passwordHash), []byte(password)) != nil {
		t.Fatal("store did not receive a bcrypt password hash")
	}
	if strings.Contains(output.String(), password) || strings.Contains(output.String(), store.passwordHash) || strings.Contains(output.String(), store.login) {
		t.Fatalf("command output leaked credentials: %q", output.String())
	}
}

func TestCreateUserRejectsPasswordArgumentBeforeReadingPassword(t *testing.T) {
	reads := 0
	err := Run(context.Background(), []string{"create-user", "--login", "admin", "--password", "secret"}, &bytes.Buffer{}, func() ([]byte, error) {
		reads++
		return nil, nil
	}, factoryFor(&initialUserStoreFake{}))
	if !errors.Is(err, ErrUnsupportedCommand) || reads != 0 {
		t.Fatalf("error = %v, password reads = %d", err, reads)
	}
}

func TestCreateUserExistingLoginDoesNotReadOrMutate(t *testing.T) {
	store := &initialUserStoreFake{exists: true}
	reads := 0
	err := Run(context.Background(), []string{"create-user", "--login", "admin"}, &bytes.Buffer{}, func() ([]byte, error) {
		reads++
		return nil, nil
	}, factoryFor(store))
	if !errors.Is(err, ErrLoginExists) || reads != 0 || store.createCalls != 0 {
		t.Fatalf("error = %v, reads = %d, create calls = %d", err, reads, store.createCalls)
	}
}

func TestCreateUserRejectsMismatchAndShortPasswordWithoutMutation(t *testing.T) {
	for _, test := range []struct {
		name      string
		passwords []string
		want      error
	}{
		{name: "mismatch", passwords: []string{"strong-password-123", "different-password"}, want: ErrPasswordMismatch},
		{name: "short", passwords: []string{"short", "short"}, want: ErrPasswordTooShort},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &initialUserStoreFake{}
			err := Run(context.Background(), []string{"create-user", "--login", "admin"}, &bytes.Buffer{}, passwordSequence(test.passwords...), factoryFor(store))
			if !errors.Is(err, test.want) || store.createCalls != 0 {
				t.Fatalf("error = %v, create calls = %d", err, store.createCalls)
			}
		})
	}
}
