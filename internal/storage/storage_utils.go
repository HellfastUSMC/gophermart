package storage

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

func GetLastUserID() int64 {
	return 0
}

func GetLastOrderID() int64 {
	return 0
}
func CheckDB() bool {
	return true
}
func CheckCashback() bool {
	return true
}

func GetLastItemID() int64 {
	return 0
}

func BasicCredDecode(encodedCredentials string) (login, password string, err error) {
	b64creds, err := base64.StdEncoding.DecodeString(encodedCredentials)
	loginPass := strings.Split(string(b64creds), ":")
	if err != nil {
		return "", "", err
	}
	login = loginPass[0]
	password = loginPass[1]
	regEx := regexp.MustCompile(`^[+][0-9]{11}$`)
	if regEx.MatchString(login) {
		return login, password, nil
	} else {
		if string(login[0]) == "8" {
			newLogin := "7" + login[1:]
			login = newLogin
		}
		login = "+" + login
		if regEx.MatchString(login) {
			return login, password, nil
		}
	}
	return "", "", fmt.Errorf("cannot use provided credentials")
}

func PasswordHasher(plainPass string) (hashedPass []byte) {
	h := sha256.New()
	h.Write([]byte(plainPass))
	hashedPass = h.Sum(nil)
	return hashedPass
}
