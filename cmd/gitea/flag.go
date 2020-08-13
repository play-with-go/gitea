package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
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
	r.setupCmd = newSetupCmd()
	r.preCmd = newPreCmd()
	r.serveCmd = newServeCmd()
	r.newUserCmd = newNewUserCmd()

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

type setupCmd struct {
	fs           *flag.FlagSet
	flagDefaults string
}

func newSetupCmd() *setupCmd {
	res := &setupCmd{}
	res.flagDefaults = newFlagSet("gitea setup", func(fs *flag.FlagSet) {
		res.fs = fs
	})
	return res
}

func (i *setupCmd) usage() string {
	return fmt.Sprintf(`
usage: gitea setup

%s`[1:], i.flagDefaults)
}

func (i *setupCmd) usageErr(format string, args ...interface{}) usageErr {
	return usageErr{fmt.Errorf(format, args...), i}
}

type preCmd struct {
	fs           *flag.FlagSet
	flagDefaults string
	fWait        *string
}

func newPreCmd() *preCmd {
	res := &preCmd{}
	res.flagDefaults = newFlagSet("gitea pre", func(fs *flag.FlagSet) {
		res.fs = fs
		res.fWait = fs.String("wait", "100s", "max time to wait for API server")
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

type serveCmd struct {
	fs           *flag.FlagSet
	flagDefaults string
	fPort        *string
}

func newServeCmd() *serveCmd {
	res := &serveCmd{}
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

type newUserCmd struct {
	fs           *flag.FlagSet
	flagDefaults string
	fDocker      *bool
}

func newNewUserCmd() *newUserCmd {
	res := &newUserCmd{}
	res.flagDefaults = newFlagSet("gitea newuser", func(fs *flag.FlagSet) {
		res.fs = fs
		res.fDocker = fs.Bool("docker", false, "process is running within docker container")
	})
	return res
}

func (i *newUserCmd) usage() string {
	return fmt.Sprintf(`
usage: gitea newuser

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
