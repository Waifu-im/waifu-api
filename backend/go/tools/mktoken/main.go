// Command mktoken issues a JWT for local testing.
//
// It exists because there is no local user yet, so there is no way to log in and
// obtain a token without first hitting Discord OAuth.
package main

import (
	"fmt"
	"os"
	"strconv"

	"waifuapi/internal/auth"
	"waifuapi/internal/config"
)

func main() {
	roleName := "User"
	if len(os.Args) > 1 {
		roleName = os.Args[1]
	}
	userID := int64(1)
	if len(os.Args) > 2 {
		if parsed, err := strconv.ParseInt(os.Args[2], 10, 64); err == nil {
			userID = parsed
		}
	}

	role, err := config.ParseRole(roleName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	issuer := auth.NewTokenIssuer(config.JWTConfig{
		Key:      os.Getenv("Jwt__Key"),
		Issuer:   os.Getenv("Jwt__Issuer"),
		Audience: os.Getenv("Jwt__Audience"),
	})

	token, _, err := issuer.Issue(userID, "tester", role, "999")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(token)
}
