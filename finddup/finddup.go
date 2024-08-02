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
	"sort"
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
			`%s - find duplicate files in one or more dirs.

Files that have the same strong-hash (blake3) are considered to be
identical. The names of the identical files are sorted on modification
time - with the most recent file at the top.

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

	dups := xsync.NewMapOf[string, *[]walk.Result]()
	err := walk.WalkFunc(args, &opt, func(res walk.Result) error {
		nm := res.Path
		cs, err := checksum(nm)
		if err != nil {
			return err
		}

		sum := fmt.Sprintf("%x", cs)
		empty := []walk.Result{}
		x, _ := dups.LoadOrStore(sum, &empty)
		*x = append(*x, res)
		return nil
	})

	if err != nil {
		Die("%s", err)
	}

	dups.Range(func(k string, pv *[]walk.Result) bool {
		v := *pv
		if len(v) < 2 {
			return true
		}

		sort.Sort(byMtime(v))

		fmt.Printf("\n# %s\n", k)
		if shell {
			fmt.Printf("# rm -f '%s'\n", v[0].Path)
			for _, r := range v[1:] {
				fmt.Printf("rm -f '%s'\n", r.Path)
			}
		} else {
			fmt.Printf("    %s\n", names(v))
		}

		return true
	})
}

func names(v []walk.Result) string {
	var b strings.Builder

	b.WriteString(v[0].Path)
	for _, r := range v[1:] {
		b.WriteString("\n    ")
		b.WriteString(r.Path)
	}
	return b.String()
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

type byMtime []walk.Result

func (r byMtime) Len() int {
	return len(r)
}

func (r byMtime) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r byMtime) Less(i, j int) bool {
	a, b := r[i].Stat, r[j].Stat

	x := a.ModTime().Compare(b.ModTime())

	// we want to keep the most recent mtime at the top.
	return x > 0
}

// This will be filled in by "build"
var RepoVersion string = "UNDEFINED"
var Buildtime string = "UNDEFINED"
var ProductVersion string = "UNDEFINED"

// vim: ft=go:sw=4:ts=4:noexpandtab:tw=78:
