// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"code.gitea.io/sdk/gitea"
)

func (ncc *newContributorCmd) run(args []string) error {
	if err := ncc.fs.Parse(args); err != nil {
		return ncc.usageErr("failed to parse flags: %v", err)
	}

	if *ncc.fEmail == "" {
		raise("must supply a new contributor email address")
	}
	if *ncc.fFullName == "" {
		raise("must supply a new contributor full name")
	}
	if *ncc.fUsername == "" {
		raise("must supply a new contributor username")
	}

	// Requires real root credentials
	client, err := gitea.NewClient(*ncc.fRootURL)
	check(err, "failed to create root client: %v", err)
	client.SetBasicAuth(os.Getenv(EnvRootUser), os.Getenv(EnvRootPassword))

	yes := true
	no := false

	password := randomPassword()

	// Create the user
	user, _, err := client.AdminCreateUser(gitea.CreateUserOption{
		Email:              *ncc.fEmail,
		FullName:           *ncc.fFullName,
		MustChangePassword: &no,
		Password:           password,
		SendNotify:         false,
		Username:           *ncc.fUsername,
	})
	check(err, "failed to create new contributor %v: %v", *ncc.fUsername, err)

	// Set further user options
	_, err = client.AdminEditUser(user.UserName, gitea.EditUserOption{
		Admin:    &yes,
		Email:    user.Email,
		FullName: user.FullName,
	})
	check(err, "failed to edit contributor %v: %v", user.UserName, err)

	// Create an access key as the user
	userClient, err := gitea.NewClient(*ncc.fRootURL)
	check(err, "failed to create user client: %v", err)
	userClient.SetBasicAuth(user.UserName, password)

	token, _, err := userClient.CreateAccessToken(gitea.CreateAccessTokenOption{
		Name: "newcontributor-created access token",
	})
	check(err, "failed to create access token for %v: %v", user.UserName, err)
	tokenAsJSON, err := json.Marshal(token)
	check(err, "failed to JSON-marshal token: %v", err)
	fmt.Printf("%s\n", tokenAsJSON)

	return nil
}

func randomPassword() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	check(err, "failed to read a random stream of bytes: %v", err)
	return fmt.Sprintf("%x", b)
}
