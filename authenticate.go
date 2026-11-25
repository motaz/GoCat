package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/scrypt"

	"io/ioutil"
	"os"

	"github.com/motaz/codeutils"
)

func readUsersFile() (usersList []string, err error) {

	var file *os.File
	filename := codeutils.GetCurrentAppDir() + "/users.dat"
	file, err = os.OpenFile(filename, os.O_RDONLY, 0644)
	if err == nil {
		defer file.Close()
		var contents []byte
		contents, err = ioutil.ReadAll(file)
		if err == nil {
			usersList = strings.Split(string(contents), "\n")
		}
	}
	return
}

func saveUsersFile(usersList []string) (err error) {

	var file *os.File
	filename := codeutils.GetCurrentAppDir() + "/users.dat"

	file, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer file.Close()
		for _, line := range usersList {
			file.WriteString(line + "\n")
		}
	}
	return
}

func getPasswordHash(username, password string, isAdmin bool) (hash string) {
	hashed, err := hashPassword(password)
	if err == nil {
		return hashed
	}
	return getPasswordHashLegacy(username, password, isAdmin)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	const n, r, p = 32768, 8, 1
	derived, err := scrypt.Key([]byte(password), salt, n, r, p, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$scrypt$%d$%d$%d$%s.%s", n, r, p, hex.EncodeToString(salt), hex.EncodeToString(derived)), nil
}

func verifyPassword(username, password string, isAdmin bool) bool {
	found, storedHash, storedAdmin := getUserCredentials(username)
	if !found || storedAdmin != isAdmin {
		return false
	}
	if strings.HasPrefix(storedHash, "$scrypt$") {
		parts := strings.Split(storedHash, "$")
		if len(parts) != 6 {
			return false
		}
		n, _ := strconv.Atoi(parts[2])
		r, _ := strconv.Atoi(parts[3])
		p, _ := strconv.Atoi(parts[4])
		saltHash := strings.Split(parts[5], ".")
		if len(saltHash) != 2 {
			return false
		}
		salt, err := hex.DecodeString(saltHash[0])
		if err != nil {
			return false
		}
		expected, err := hex.DecodeString(saltHash[1])
		if err != nil || len(expected) != 32 {
			return false
		}
		derived, err := scrypt.Key([]byte(password), salt, n, r, p, 32)
		return err == nil && subtle.ConstantTimeCompare(derived, expected) == 1
	}

	legacyHash := getPasswordHashLegacy(username, password, isAdmin)
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(legacyHash)) == 1
}

func getPasswordHashLegacy(username, password string, isAdmin bool) string {
	if !isAdmin {
		username += "-"
	}
	return GetMD5Hash(password + username + "_!@")
}

func setUser(username, password string, isAdmin bool) (err error) {

	usersList, _ := readUsersFile()
	username = strings.ToLower(username)
	var perm string
	if isAdmin {
		perm = "1"
	} else {
		perm = "0"
	}
	hashpass := getPasswordHash(username, password, isAdmin)
	found := false
	credentials := username + ":" + perm + ":" + hashpass
	for i, line := range usersList {
		if strings.HasPrefix(line, username+":") {
			found = true
			usersList[i] = credentials
			break
		}
	}
	if !found {
		usersList = append(usersList, credentials)
	}
	err = saveUsersFile(usersList)
	return
}

func getUserCredentials(username string) (found bool, hashpass string, isAdmin bool) {

	usersList, err := readUsersFile()
	found = false
	if err == nil {
		for _, line := range usersList {
			if strings.HasPrefix(line, username+":") {

				credList := strings.Split(line, ":")
				if len(credList) > 2 {
					found = true
					isAdmin = credList[1] == "1"
					hashpass = credList[2]

				}
			}
		}
	}
	return
}
