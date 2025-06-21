package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

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
