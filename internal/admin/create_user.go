package admin

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const minimumPasswordBytes = 12

var (
	ErrLoginRequired      = errors.New("login is required")
	ErrLoginExists        = errors.New("login already exists")
	ErrPasswordMismatch   = errors.New("password confirmation does not match")
	ErrPasswordTooShort   = errors.New("password must contain at least 12 bytes")
	ErrPasswordTooLong    = errors.New("password exceeds bcrypt 72-byte limit")
	ErrUnsupportedCommand = errors.New("supported command: create-user --login <login>")
)

type InitialUserStore interface {
	LoginExists(context.Context, string) (bool, error)
	CreateInitialUser(context.Context, string, string) error
}

type StoreFactory func(context.Context) (InitialUserStore, func() error, error)
type PasswordReader func() ([]byte, error)

func Run(ctx context.Context, args []string, output io.Writer, readPassword PasswordReader, stores StoreFactory) error {
	if len(args) == 0 || args[0] != "create-user" {
		return ErrUnsupportedCommand
	}
	if output == nil || readPassword == nil || stores == nil {
		return errors.New("admin command dependencies are required")
	}
	flags := flag.NewFlagSet("create-user", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	loginFlag := flags.String("login", "", "initial user login")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return ErrUnsupportedCommand
	}
	login := strings.ToLower(strings.TrimSpace(*loginFlag))
	if login == "" {
		return ErrLoginRequired
	}

	store, closeStore, err := stores(ctx)
	if err != nil {
		return errors.New("cannot open staging user store")
	}
	if closeStore != nil {
		defer closeStore()
	}
	exists, err := store.LoginExists(ctx, login)
	if err != nil {
		return errors.New("cannot verify initial user")
	}
	if exists {
		return ErrLoginExists
	}

	if _, err := fmt.Fprint(output, "Password: "); err != nil {
		return err
	}
	password, err := readPassword()
	if err != nil {
		return fmt.Errorf("read password securely: %w", err)
	}
	defer clearBytes(password)
	if _, err := fmt.Fprint(output, "\nConfirm password: "); err != nil {
		return err
	}
	confirmation, err := readPassword()
	if err != nil {
		return fmt.Errorf("read password confirmation securely: %w", err)
	}
	defer clearBytes(confirmation)
	if _, err := fmt.Fprintln(output); err != nil {
		return err
	}
	if len(password) < minimumPasswordBytes {
		return ErrPasswordTooShort
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}
	if !bytes.Equal(password, confirmation) {
		return ErrPasswordMismatch
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return errors.New("cannot hash initial user password")
	}
	defer clearBytes(hash)
	if err := store.CreateInitialUser(ctx, login, string(hash)); err != nil {
		return errors.New("cannot create initial user")
	}
	_, err = fmt.Fprintln(output, "Initial user created.")
	return err
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
