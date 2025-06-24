package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrNoAuthHeaderIncluded = errors.New("no auth header included in request")

func HashPassword(password string) (string, error) {
	println("HASING PASSWORD")
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	return string(hash), nil
}

func CheckPasswordHash(hash, password string) error {
	fmt.Println(password)
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresIn)),
		Subject:   userID.String(),
	})
	signed, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		fmt.Println("An error occured in MakeJWT():", err)
	}
	return signed, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	subject, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.UUID{}, err
	}
	return subject, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", ErrNoAuthHeaderIncluded
	}
	splitAuth := strings.Split(authHeader, " ")
	if len(splitAuth) < 2 || splitAuth[0] != "Bearer" {
		return "", errors.New("malformed authorization header")
	}

	return splitAuth[1], nil
}

func MakeRefreshToken() (string, error) {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key), nil
}

func GetAPIKey(headers http.Header) (string, error) {
	authValues, exists := headers["Authorization"]
	if !exists {
		return "", errors.New("No Authorization header found")
	}
	return strings.Split(authValues[0], " ")[1], nil
}
