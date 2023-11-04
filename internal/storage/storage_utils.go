package storage

import (
	"golang.org/x/crypto/bcrypt"
)

func PasswordHasher(plainPass string) ([]byte, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plainPass), bcrypt.DefaultCost)
	return bytes, err
}
