package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const (
	pullImageMissing = "missing"
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
	r.waitCmd = newWaitCmd()
	r.preCmd = newPreCmd()
	r.newUserCmd = newInitCmd()

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
}

func newFlagSet(name string, setupFlags func(*flag.FlagSet)) string {
	res := flag.NewFlagSet(name, flag.ContinueOnError)
	var b bytes.Buffer
	res.SetOutput(&b)
	setupFlags(res)
	res.PrintDefaults()
	res.SetOutput(ioutil.Discard)
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
		res.fRootURL = fs.String("rootURL", "https://play-with-go.dev", "root URL for all requests")
	})
	return res
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

type waitCmd struct {
	fs           *flag.FlagSet
	flagDefaults string
	fWait        *string
}

func newWaitCmd() *waitCmd {
	res := &waitCmd{}
	res.flagDefaults = newFlagSet("gitea wait", func(fs *flag.FlagSet) {
		res.fs = fs
		res.fWait = fs.String("wait", "10s", "max time to wait for API server")
	})
	return res
}

func (g *waitCmd) usage() string {
	return fmt.Sprintf(`
usage: gitea wait

%s`[1:], g.flagDefaults)
}

func (g *waitCmd) usageErr(format string, args ...interface{}) usageErr {
	return usageErr{fmt.Errorf(format, args...), g}
}

type preCmd struct {
	fs           *flag.FlagSet
	flagDefaults string
}

func newPreCmd() *preCmd {
	res := &preCmd{}
	res.flagDefaults = newFlagSet("gitea pre", func(fs *flag.FlagSet) {
		res.fs = fs
	})
	return res
}

func (g *preCmd) usage() string {
	return fmt.Sprintf(`
usage: gitea pre

%s`[1:], g.flagDefaults)
}

func (g *preCmd) usageErr(format string, args ...interface{}) usageErr {
	return usageErr{fmt.Errorf(format, args...), g}
}

type newUserCmd struct {
	fs           *flag.FlagSet
	flagDefaults string
}

func newInitCmd() *newUserCmd {
	res := &newUserCmd{}
	res.flagDefaults = newFlagSet("gitea init", func(fs *flag.FlagSet) {
		res.fs = fs
	})
	return res
}

func (i *newUserCmd) usage() string {
	return fmt.Sprintf(`
usage: gitea init

%s`[1:], i.flagDefaults)
}

func (i *newUserCmd) usageErr(format string, args ...interface{}) usageErr {
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
