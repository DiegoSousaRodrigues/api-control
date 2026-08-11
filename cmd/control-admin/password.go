package main

import (
	"errors"
	"os"

	"golang.org/x/term"
)

func readPasswordSecurely() ([]byte, error) {
	fileDescriptor := int(os.Stdin.Fd())
	if !term.IsTerminal(fileDescriptor) {
		return nil, errors.New("password input requires an interactive terminal")
	}
	password, err := term.ReadPassword(fileDescriptor)
	if err != nil {
		return nil, errors.New("cannot read password from terminal")
	}
	return password, nil
}
