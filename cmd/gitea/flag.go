// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
)

type usageErr struct {
	err error
	u   cmd
}

func (u usageErr) Error() string { return u.err.Error() }

func main() { os.Exit(main1()) }

func main1() int {
	r := newRunner()

	r.rootCmd = newRootCmd()
	r.serveCmd = newServeCmd(r)
	r.newContributorCmd = newNewContributorCmd(r)
	r.reapCmd = newReapCmd(r)

	err := r.mainerr()
	if err == nil {
		return 0
	}
	switch err := err.(type) {
	case usageErr:
		if err.err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, err.err)
		}
		fmt.Fprint(os.Stderr, err.u.usage())
		return 2
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

type cmd interface {
	usage() string
	usageErr(format string, args ...interface{}) usageErr
}

type rootCmd struct {
	fs           *flag.FlagSet
	flagDefaults string
	fDebug       *bool
	fRootURL     *string

	// hostname is the parse host from -rootURL
	hostname string
}

func newFlagSet(name string, setupFlags func(*flag.FlagSet)) string {
	res := flag.NewFlagSet(name, flag.ContinueOnError)
	var b bytes.Buffer
	res.SetOutput(&b)
	setupFlags(res)
	res.PrintDefaults()
	res.SetOutput(io.Discard)
	s := b.String()
	const indent = "\t"
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			lines[i] = ""
		} else {
			lines[i] = indent + strings.Replace(l, "\t", "    ", 1)
		}
	}
	return strings.Join(lines, "\n")
}

func newRootCmd() *rootCmd {
	res := &rootCmd{}
	res.flagDefaults = newFlagSet("gitea", func(fs *flag.FlagSet) {
		res.fs = fs
		res.fDebug = fs.Bool("debug", false, "include debug output")
		res.fRootURL = fs.String("rootURL", envOrVal("GITEA_ROOT_URL", "https://gopher.live"), "root URL for all requests")
	})
	return res
}

func envOrVal(env string, val string) string {
	if v, ok := os.LookupEnv(env); ok {
		return v
	}
	return val
}

func (r *rootCmd) usage() string {
	return fmt.Sprintf(`
Usage of gitea:

gitea defines the following flags:

%s`[1:], r.flagDefaults)
}

func (r *rootCmd) usageErr(format string, args ...interface{}) usageErr {
	return usageErr{fmt.Errorf(format, args...), r}
}

type serveCmd struct {
	*runner
	fs           *flag.FlagSet
	flagDefaults string
	fPort        *string

	client *gitea.Client

	keyScan string
}

func newServeCmd(r *runner) *serveCmd {
	res := &serveCmd{runner: r}
	res.flagDefaults = newFlagSet("gitea serve", func(fs *flag.FlagSet) {
		res.fs = fs
		res.fPort = fs.String("port", "8080", "port on which to listen")
	})
	return res
}

func (g *serveCmd) usage() string {
	return fmt.Sprintf(`
usage: gitea serve

%s`[1:], g.flagDefaults)
}

func (g *serveCmd) usageErr(format string, args ...interface{}) usageErr {
	return usageErr{fmt.Errorf(format, args...), g}
}

type newContributorCmd struct {
	*runner
	fs           *flag.FlagSet
	fEmail       *string
	fFullName    *string
	fUsername    *string
	flagDefaults string
}

func newNewContributorCmd(r *runner) *newContributorCmd {
	res := &newContributorCmd{runner: r}
	res.flagDefaults = newFlagSet("gitea newuser", func(fs *flag.FlagSet) {
		res.fs = fs
		res.fEmail = fs.String("email", "", "New contributor email address")
		res.fUsername = fs.String("username", "", "New contributor username")
		res.fFullName = fs.String("fullname", "", "New contributor full name")
	})
	return res
}

func (i *newContributorCmd) usage() string {
	return fmt.Sprintf(`
usage: gitea newcontributor

%s`[1:], i.flagDefaults)
}

func (i *newContributorCmd) usageErr(format string, args ...interface{}) usageErr {
	return usageErr{fmt.Errorf(format, args...), i}
}

type reapCmd struct {
	*runner
	fs           *flag.FlagSet
	fAge         *string
	flagDefaults string

	now    time.Time
	age    time.Duration
	client *gitea.Client
}

func newReapCmd(r *runner) *reapCmd {
	res := &reapCmd{runner: r}
	res.flagDefaults = newFlagSet("gitea newuser", func(fs *flag.FlagSet) {
		res.fs = fs
		res.fAge = fs.String("age", "3h", "Age beyond which users and repositories will be reaped")
	})
	return res
}

func (i *reapCmd) usage() string {
	return fmt.Sprintf(`
usage: gitea reap

%s`[1:], i.flagDefaults)
}

func (i *reapCmd) usageErr(format string, args ...interface{}) usageErr {
	return usageErr{fmt.Errorf(format, args...), i}
}

func check(err error, format string, args ...interface{}) {
	if err != nil {
		if format != "" {
			err = fmt.Errorf(format, args...)
		}
		panic(knownErr{err})
	}
}

func raise(format string, args ...interface{}) {
	panic(knownErr{fmt.Errorf(format, args...)})
}

type knownErr struct{ error }

func handleKnown(err *error) {
	switch r := recover().(type) {
	case nil:
	case knownErr:
		*err = r
	default:
		panic(r)
	}
}
