package main

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	result, err := readUsersFile()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		for i, line := range result {
			fmt.Printf("Result: %d %s\n", i, line)
		}
	}
	setUser("motaz", "test", &result)
	setUser("ahmed", "test", &result)
	setUser("ahmed2", "test2", &result)
	fmt.Printf("%+v\n", result)
}
