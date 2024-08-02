// finddup.go - find duplicate files in one or more dirs and below
//
// Author: Sudhi Herle (sw@herle.net)
// License: GPLv2
package main

import (
	"fmt"
	"hash"
	"os"
	"path"
	"strings"

	"github.com/opencoff/go-mmap"
	"github.com/opencoff/go-walk"
	flag "github.com/opencoff/pflag"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/zeebo/blake3"
)

var Z string = path.Base(os.Args[0])

type csum struct {
	name string
	sum  string
	err  error
}

func main() {
	var version, shell, follow bool
	var ignores []string = []string{".git", ".hg"}

	flag.BoolVarP(&version, "version", "", false, "Show version info and quit")
	flag.BoolVarP(&follow, "follow-symlinks", "L", false, "Follow symlinks")
	flag.BoolVarP(&shell, "shell", "s", false, "Generate shell commands")
	flag.StringSliceVarP(&ignores, "ignore", "i", ignores, "Ignore names that match these patterns")

	flag.Usage = func() {
		fmt.Printf(
			`%s - find duplicate files in one or more dirs

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
		Die("Insufficient args. Try %s --help", Z)
	}

	opt := walk.Options{
		FollowSymlinks: follow,
		Type:           walk.FILE,
		Excludes:       ignores,
	}

	dups := xsync.NewMapOf[string, *[]string]()
	err := walk.WalkFunc(args, &opt, func(res walk.Result) error {
		nm := res.Path
		cs, err := checksum(nm)
		if err != nil {
			return err
		}

		sum := fmt.Sprintf("%x", cs)
		empty := []string{}
		x, _ := dups.LoadOrStore(sum, &empty)
		*x = append(*x, nm)
		return nil
	})

	if err != nil {
		Die("%s", err)
	}

	dups.Range(func(k string, pv *[]string) bool {
		v := *pv
		if len(v) < 2 {
			return true
		}

		fmt.Printf("\n# %s\n", k)
		if shell {
			fmt.Printf("# rm -f '%s'\n", v[0])
			for i := 1; i < len(v); i++ {
				fmt.Printf("rm -f '%s'\n", v[i])
			}
		} else {
			fmt.Printf("    %s\n", strings.Join(v, "\n    "))
		}

		return true
	})
}

// create a new cryptographic hash func
func hasher() hash.Hash {
	var zeroes [32]byte

	h, err := blake3.NewKeyed(zeroes[:])
	if err != nil {
		panic(fmt.Sprintf("blake3: %s", err))
	}
	return h
}

// fast checksum using mmap
func checksum(fn string) ([]byte, error) {
	fd, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", fn, err)
	}

	defer fd.Close()

	h := hasher()
	_, err = mmap.Reader(fd, func(buf []byte) error {
		h.Write(buf)
		return nil
	})

	return h.Sum(nil)[:], err
}

// This will be filled in by "build"
var RepoVersion string = "UNDEFINED"
var Buildtime string = "UNDEFINED"
var ProductVersion string = "UNDEFINED"

// vim: ft=go:sw=4:ts=4:noexpandtab:tw=78:
