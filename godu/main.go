// main.go - parallel du(1)
//
// (c) 2016 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

package main

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/opencoff/go-utils"
	"github.com/opencoff/go-walk"
	flag "github.com/opencoff/pflag"
)

var Z string = path.Base(os.Args[0])
var Verbose bool

type result struct {
	name string
	size uint64
}

type bySize []result

func (r bySize) Len() int {
	return len(r)
}

func (r bySize) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// we're doing reverse sort.
func (r bySize) Less(i, j int) bool {
	return r[i].size > r[j].size
}

func main() {
	var version bool
	var human bool
	var kb bool
	var byts bool
	var total bool
	var symlinks bool
	var onefs bool
	var all bool
	var excludes []string

	flag.BoolVarP(&version, "version", "", false, "Show version info and quit")
	flag.BoolVarP(&Verbose, "verbose", "v", false, "Show verbose output")
	flag.BoolVarP(&symlinks, "follow-symlinks", "L", false, "Follow symlinks")
	flag.BoolVarP(&onefs, "single-filesystem", "x", false, "Don't cross mount points")
	flag.BoolVarP(&all, "all", "a", false, "Show all files & dirs")
	flag.BoolVarP(&human, "human-size", "h", false, "Show size in human readable form")
	flag.BoolVarP(&kb, "kilo-byte", "k", false, "Show size in kilo bytes")
	flag.BoolVarP(&byts, "byte", "b", false, "Show size in bytes")
	flag.BoolVarP(&total, "total", "t", false, "Show total size")
	flag.StringSliceVarP(&excludes, "exclude", "", nil, "Exclude names starting with `N`")

	flag.Usage = func() {
		fmt.Printf(
			`%s - disk utilization calculator (parallel edition)

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

	var size func(uint64) string

	if human {
		size = utils.HumanizeSize
	} else if kb {
		size = func(z uint64) string {
			z /= 1024
			return fmt.Sprintf("%d", z)
		}
	} else {
		size = func(z uint64) string {
			return fmt.Sprintf("%d", z)
		}
	}

	// sort the args in decreasing length so our prefix matching always
	// finds the longest match
	sort.Sort(byLen(args))

	opt := &walk.Options{
		FollowSymlinks: symlinks,
		OneFS:          onefs,
		Type:           walk.FILE,
		Excludes:       excludes,
	}

	// We know this function will be called from a single threaded
	// context; so we can use a regular map and not sync.Map
	linkmap := make(map[string]string)
	hardlinked := func(fi os.FileInfo, nm string) bool {
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return false
		}
		if st.Nlink == 1 {
			return false
		}

		key := fmt.Sprintf("%d:%d:%d", st.Dev, st.Rdev, st.Ino)
		if _, ok := linkmap[key]; ok {
			return true
		}

		linkmap[key] = nm
		return false
	}

	ch, ech := walk.Walk(args, opt)

	// harvest errors
	errs := make([]string, 0, 8)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for e := range ech {
			errs = append(errs, fmt.Sprintf("%s", e))
		}
		wg.Done()
	}()

	// now harvest results - we know we will only get files and their info.
	res := make([]result, 0, 1024)
	sizes := make(map[string]uint64)
	for r := range ch {
		// don't count hardlinks
		if hardlinked(r.Stat, r.Path) {
			continue
		}

		sz := uint64(r.Stat.Size())
		for i := range args {
			nm := args[i]
			if strings.HasPrefix(r.Path, nm) {
				sizes[nm] += sz
				break
			}
		}
		if all {
			res = append(res, result{r.Path, sz})
		}
	}

	wg.Wait()
	if len(errs) > 0 {
		die("%s", strings.Join(errs, "\n"))
	}

	if !all {
		for k, v := range sizes {
			res = append(res, result{k, v})
		}

	}

	var tot uint64
	sort.Sort(bySize(res))
	for i := range res {
		r := res[i]
		tot += r.size
		fmt.Printf("%12s %s\n", size(r.size), r.name)
	}
	if total {
		fmt.Printf("%12s TOTAL\n", size(tot))
	}
}

type byLen []string

func (b byLen) Len() int {
	return len(b)
}

// we're doing decreasing order of length
func (b byLen) Less(i, j int) bool {
	return len(b[i]) > len(b[j])
}

func (b byLen) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// This will be filled in by "build"
var RepoVersion string = "UNDEFINED"
var Buildtime string = "UNDEFINED"
var ProductVersion string = "UNDEFINED"
