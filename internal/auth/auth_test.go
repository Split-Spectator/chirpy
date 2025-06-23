package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordAndCheckPasswordHash_Success(t *testing.T) {
	password := "mySecretPassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty hash")
	}

	// Check that the hash matches the password
	err = CheckPasswordHash(hash, password)
	if err != nil {
		t.Errorf("CheckPasswordHash failed for correct password: %v", err)
	}
}

func TestCheckPasswordHash_WrongPassword(t *testing.T) {
	password := "correctPassword"
	wrongPassword := "wrongPassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	err = CheckPasswordHash(hash, wrongPassword)
	if err == nil {
		t.Error("CheckPasswordHash did not fail for incorrect password")
	}
	if err != bcrypt.ErrMismatchedHashAndPassword {
		t.Errorf("Expected ErrMismatchedHashAndPassword, got: %v", err)
	}
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword returned error for empty password: %v", err)
	}
	if hash == "" {
		t.Error("HashPassword returned empty hash for empty password")
	}
}

func TestCheckPasswordHash_InvalidHash(t *testing.T) {
	invalidHash := "notAValidHash"
	password := "anyPassword"
	err := CheckPasswordHash(invalidHash, password)
	if err == nil {
		t.Error("CheckPasswordHash did not fail for invalid hash")
	}
}

func TestMakeJWTAndValidateJWT_Success(t *testing.T) {
	userID := uuid.New()
	secret := "supersecret"
	expiresIn := time.Minute * 5

	token, err := MakeJWT(userID, secret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT returned error: %v", err)
	}
	if token == "" {
		t.Fatal("MakeJWT returned empty token")
	}

	parsedID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT returned error: %v", err)
	}
	if parsedID != userID {
		t.Errorf("ValidateJWT returned wrong userID: got %v, want %v", parsedID, userID)
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	secret := "supersecret"
	invalidToken := "this.is.not.a.valid.jwt"

	_, err := ValidateJWT(invalidToken, secret)
	if err == nil {
		t.Error("ValidateJWT did not fail for invalid token")
	}
}

func TestGetBearerToken(t *testing.T) {
	headers := http.Header{}
	headers["Authorization"] = []string{"Bearer 12345"}
	tokenString, err := GetBearerToken(headers)
	if err != nil || tokenString != "12345" {
		t.Errorf("Failed to get Bearer token. Err: %v", err)
	}
}
