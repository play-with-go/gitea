package main

import (
	"time"

	"code.gitea.io/sdk/gitea"
	"gopkg.in/retry.v1"
)

func (r *runner) runPre(args []string) error {
	if err := r.preCmd.fs.Parse(args); err != nil {
		return r.preCmd.usageErr("failed to parse flags: %v", err)
	}
	// Try the version endpoint with backoff until success or timeout
	wait, err := time.ParseDuration(*r.preCmd.fWait)
	check(err, "failed to parse duration from -wait flag: %v", err)

	strategy := retry.LimitTime(wait,
		retry.Exponential{
			Initial: 10 * time.Millisecond,
			Factor:  1.5,
		},
	)
	for a := retry.Start(strategy, nil); a.Next(); {
		_, err = r.client.ServerVersion()
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	_, err = r.client.CreateOrg(gitea.CreateOrgOption{
		UserName:   UserGuidesRepo,
		Visibility: "private",
	})
	check(err, "failed to create %v organisation: %v", UserGuidesRepo, err)
	return nil
}