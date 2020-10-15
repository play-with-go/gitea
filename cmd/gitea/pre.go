// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"os"

	"code.gitea.io/sdk/gitea"
)

func (pc *preCmd) run(args []string) error {
	if err := pc.fs.Parse(args); err != nil {
		return pc.usageErr("failed to parse flags: %v", err)
	}

	// Requires real root credentials
	client, err := gitea.NewClient(*pc.fRootURL)
	check(err, "failed to create client: %v", err)
	client.SetBasicAuth(os.Getenv(EnvRootUser), os.Getenv(EnvRootPassword))

	_, _, err = client.CreateOrg(gitea.CreateOrgOption{
		Name:       GiteaOrg,
		Visibility: "private",
	})
	check(err, "failed to create %v organisation: %v", GiteaOrg, err)
	return nil
}
