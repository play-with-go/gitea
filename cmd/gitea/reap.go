// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"fmt"
	"os"
	"time"

	"code.gitea.io/sdk/gitea"
)

func (rc *reapCmd) run(args []string) error {
	if err := rc.fs.Parse(args); err != nil {
		return rc.usageErr("failed to parse flags: %v", err)
	}

	var err error

	rc.now = time.Now()

	rc.age, err = time.ParseDuration(*rc.fAge)
	check(err, "failed to parse duration from %v: %v", *rc.fAge, err)

	// Requires real root credentials
	rc.client, err = gitea.NewClient(*rc.fRootURL)
	check(err, "failed to create root client: %v", err)
	rc.client.SetBasicAuth(os.Getenv(EnvRootUser), os.Getenv(EnvRootPassword))

	rc.removeOldRepos()
	rc.removeOldUsers()

	return nil
}

func (rc *reapCmd) removeOldRepos() {
	opt := gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{
			PageSize: 10,
		},
	}
	for {
		repos, _, err := rc.client.ListUserRepos(GiteaOrg, opt)
		check(err, "failed to list repos via %v: %v", *rc.fRootURL, err)
		for _, repo := range repos {
			if delta := rc.now.Sub(repo.Created); delta > rc.age {
				_, err := rc.client.DeleteRepo(GiteaOrg, repo.Name)
				check(err, "failed to delete repo %v/%v: %v", GiteaOrg, repo.Name, err)
				fmt.Fprintf(os.Stderr, "deleted repo %v/%v (was %v old)\n", GiteaOrg, repo.Name, delta)
			}
		}
		if len(repos) < opt.PageSize {
			break
		}
		opt.Page++
	}

}

func (rc *reapCmd) removeOldUsers() {
	opt := gitea.AdminListUsersOptions{
		ListOptions: gitea.ListOptions{
			PageSize: 10,
		},
	}
	for {
		users, _, err := rc.client.AdminListUsers(opt)
		check(err, "failed to list users: %v", err)
		for _, user := range users {
			if user.FullName != TemporaryUserFullName {
				continue
			}
			delta := rc.now.Sub(user.Created)
			if delta < rc.age {
				continue
			}
			_, err := rc.client.AdminDeleteUser(user.UserName)
			check(err, "failed to delete user %v: %v", user.UserName, err)
			fmt.Fprintf(os.Stderr, "deleted user %v (was %v old)\n", user.UserName, delta)
		}
		if len(users) < opt.PageSize {
			break
		}
		opt.Page++
	}
}
