package main

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	result := runApp("SMSRestGo")
	fmt.Println("Result: " + result)
}
