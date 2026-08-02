package utils

import (
	"slices"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

const validJWTSecret = "test-only-secret-with-at-least-32-characters"

func TestValidateJWTConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "missing secret", wantErr: true},
		{name: "weak secret", secret: "too-short", wantErr: true},
		{name: "valid secret", secret: validJWTSecret},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", tt.secret)

			err := ValidateJWTConfiguration()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateJWTConfiguration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateAndValidateJWT(t *testing.T) {
	t.Setenv("JWT_SECRET", validJWTSecret)

	token, err := GenerateJWT(42)
	if err != nil {
		t.Fatalf("GenerateJWT() error = %v", err)
	}

	claims, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT() error = %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("ValidateJWT() user ID = %d, want 42", claims.UserID)
	}
	if claims.Issuer != jwtIssuer {
		t.Fatalf("ValidateJWT() issuer = %q, want %q", claims.Issuer, jwtIssuer)
	}
	if !slices.Contains(claims.Audience, jwtAudience) {
		t.Fatalf("ValidateJWT() audience = %v, want %q", claims.Audience, jwtAudience)
	}
}

func TestGenerateJWTRejectsInvalidInput(t *testing.T) {
	t.Setenv("JWT_SECRET", validJWTSecret)

	if _, err := GenerateJWT(0); err == nil {
		t.Fatal("GenerateJWT() expected an error for an invalid user ID")
	}
}

func TestValidateJWTRejectsWrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", validJWTSecret)
	token, err := GenerateJWT(42)
	if err != nil {
		t.Fatalf("GenerateJWT() error = %v", err)
	}

	t.Setenv("JWT_SECRET", strings.Repeat("x", jwtSecretMinLength))
	if _, err := ValidateJWT(token); err == nil {
		t.Fatal("ValidateJWT() accepted a token signed with another secret")
	}
}

func TestValidateJWTRejectsUnexpectedAlgorithm(t *testing.T) {
	t.Setenv("JWT_SECRET", validJWTSecret)
	claims := &Claims{
		UserID: 42,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   jwtIssuer,
			Audience: jwt.ClaimStrings{jwtAudience},
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("creating unsigned token: %v", err)
	}

	if _, err := ValidateJWT(token); err == nil {
		t.Fatal("ValidateJWT() accepted an unexpected signing algorithm")
	}
}
