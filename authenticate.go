package main

import (
	"strings"

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

	if !isAdmin {
		username += "-"
	}
	hash = GetMD5Hash(password + username + "_!@")
	return
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
