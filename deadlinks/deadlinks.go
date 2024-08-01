// deadlinks.go - find dead symlinks in one or more dir trees
//
// Author: Sudhi Herle (sw@herle.net)
// License: GPLv2

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/opencoff/go-walk"
	flag "github.com/opencoff/pflag"
)

var Z string = path.Base(os.Args[0])

type Result struct {
	Link   string
	Target string
}

func main() {
	var version, zero, showTarget bool
	var ignores []string = []string{".git", ".hg"}

	flag.BoolVarP(&version, "version", "", false, "Show version info and quit")
	flag.BoolVarP(&zero, "null", "0", false, "use \\0 as the output 'line separator'")
	flag.BoolVarP(&showTarget, "show-dead-target", "t", false, "Show dead symlink target")
	flag.StringSliceVarP(&ignores, "ignore", "i", ignores, "Ignore names that match these patterns")

	flag.Usage = func() {
		fmt.Printf(
			`%s - find dead symlinks in one or more dir trees

Usage: %s [options] dir [dir...]

Options:
`, Z, Z)
		flag.PrintDefaults()
		os.Stdout.Sync()
		os.Exit(0)
	}

	flag.Parse()
	if version {
		fmt.Printf("%s - %s [%s; %s]\n", Z, ProductVersion, RepoVersion, Buildtime)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		die("Insufficient args. Try %s --help", Z)
	}

	opt := walk.Options{
		FollowSymlinks: false,
		Type:           walk.SYMLINK,
		Excludes:       ignores,
	}

	out := make(chan Result, 1)
	var dead strings.Builder
	var wg sync.WaitGroup

	var sep = "\n"
	if zero {
		sep = "\000"
	}

	wg.Add(1)
	go func(ch chan Result) {
		if showTarget {
			for r := range ch {
				dead.WriteString(fmt.Sprintf("%s -> %s%s", r.Link, r.Target, sep))
			}
		} else {
			for r := range ch {
				dead.WriteString(fmt.Sprintf("%s%s", r.Link, sep))
			}
		}
		wg.Done()
	}(out)

	errs := walk.WalkFunc(args, &opt, func(res walk.Result) error {
		// we know nm is a symlink; we read the link and eval it
		nm := res.Path
		_, err := filepath.EvalSymlinks(nm)
		if err != nil {
			targ, err := os.Readlink(nm)
			if err != nil {
				return err
			}
			out <- Result{nm, targ}
		}
		return nil
	})

	if len(errs) > 0 {
		var s strings.Builder
		for _, v := range errs {
			s.WriteString(fmt.Sprintf("%w\n", v))
		}
		die("%s", s.String())
	}

	close(out)
	wg.Wait()
	if dead.Len() > 0 {
		fmt.Printf(dead.String())
	}
}

// die with error
func die(f string, v ...interface{}) {
	warn(f, v...)
	os.Exit(1)
}

func warn(f string, v ...interface{}) {
	z := fmt.Sprintf("%s: %s", os.Args[0], f)
	s := fmt.Sprintf(z, v...)
	if n := len(s); s[n-1] != '\n' {
		s += "\n"
	}

	os.Stderr.WriteString(s)
	os.Stderr.Sync()
}

// This will be filled in by "build"
var RepoVersion string = "UNDEFINED"
var Buildtime string = "UNDEFINED"
var ProductVersion string = "UNDEFINED"

// vim: ft=go:sw=4:ts=4:noexpandtab:tw=78:
