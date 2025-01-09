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

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/walk"
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
		fmt.Printf("%s - %s [%s]\n", Z, ProductVersion, RepoVersion)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		Die("Insufficient args. Try %s --help", Z)
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

	err := walk.WalkFunc(args, opt, func(fi *fio.Info) error {
		// we know nm is a symlink; we read the link and eval it
		nm := fi.Path()
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

	if err != nil {
		Die("%s", err)
	}

	close(out)
	wg.Wait()
	if dead.Len() > 0 {
		fmt.Printf(dead.String())
	}
}

// This will be filled in by "build"
var RepoVersion string = "UNDEFINED"
var ProductVersion string = "UNDEFINED"

// vim: ft=go:sw=4:ts=4:noexpandtab:tw=78:
