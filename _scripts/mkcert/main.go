package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sethvargo/go-githubactions"
)

func main() {
	dir := os.Args[1]
	load := func(filename string) string {
		c, err := ioutil.ReadFile(filepath.Join(dir, filename))
		check(err)
		return string(c)
	}
	cert := load("cert.pem")
	key := load("key.pem")
	githubactions.SetEnv("PLAYWITHGODEV_CERT_FILE", cert)
	githubactions.SetEnv("PLAYWITHGODEV_KEY_FILE", key)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
