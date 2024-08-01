// process.go -- process files/dirs and compute hashes
//
// (c) 2023 Sudhi Herle <sudhi@herle.net>
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
	"path/filepath"
	"sync"
	"syscall"

	"github.com/opencoff/go-walk"
	"runtime"
)

const _parallelism int = 2

var nWorkers = runtime.NumCPU() * _parallelism

// iterate over the names
func processArgs(args []string, followSymlinks bool, apply func(r walk.Result) error) []error {
	nw := nWorkers
	if len(args) < nw {
		nw = len(args)
	}

	ch := make(chan walk.Result, nWorkers)
	errch := make(chan error, 1)

	// iterate in the background and feed the workers
	go func(ch chan walk.Result, errch chan error) {
		var sr symlinkResolver

		for _, nm := range args {
			var fi os.FileInfo
			var err error

			fi, err = os.Lstat(nm)
			if err != nil {
				errch <- fmt.Errorf("lstat %s: %w", nm, err)
				continue
			}

			if sr.isEntrySeen(nm, fi) {
				continue
			}

			m := fi.Mode()

			// if we're following symlinks, update fi & m
			if (m & os.ModeSymlink) > 0 {
				if !followSymlinks {
					errch <- fmt.Errorf("skipping symlink %s", nm)
					continue
				}

				nm, fi, err = sr.resolve(nm, fi)
				if err != nil {
					errch <- fmt.Errorf("%w", nm, err)
					continue
				}

				// a nil name means we can skip this entry
				if nm == "" {
					continue
				}

				m = fi.Mode()
			}

			switch {
			case m.IsDir():
				errch <- fmt.Errorf("skipping dir %s..", nm)

			case m.IsRegular():
				ch <- walk.Result{Path: nm, Stat: fi}

			default:
				errch <- fmt.Errorf("skipping non-file %s..", nm)
			}
		}
		close(ch)
	}(ch, errch)

	// now start workers and process entries
	var errs []error
	var wrkWait, errWait sync.WaitGroup

	errWait.Add(1)
	go func(ch chan error) {
		for err := range ch {
			errs = append(errs, err)
		}
		errWait.Done()
	}(errch)

	wrkWait.Add(nw)
	for i := 0; i < nw; i++ {
		go func(in chan walk.Result, errch chan error) {
			for r := range in {
				err := apply(r)
				if err != nil {
					errch <- err
				}
			}
			wrkWait.Done()
		}(ch, errch)
	}

	wrkWait.Wait()
	close(errch)
	errWait.Wait()

	return errs
}

type symlinkResolver struct {
	seen sync.Map
}

func (s *symlinkResolver) resolve(nm string, fi os.FileInfo) (string, os.FileInfo, error) {
	newnm, err := filepath.EvalSymlinks(nm)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", nm, err)
	}
	nm = newnm

	// we know this is no longer a symlink
	fi, err = os.Stat(nm)
	if err != nil {
		return "", nil, fmt.Errorf("stat %s: %w", nm, err)
	}

	// If we've seen this inode before, we are done.
	if s.isEntrySeen(nm, fi) || !fi.Mode().IsRegular() {
		return "", nil, nil
	}

	return nm, fi, nil
}

func (s *symlinkResolver) track(nm string, fi os.FileInfo, errch chan error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		errch <- fmt.Errorf("%s: can't read stat from FileInfo", nm)
		return
	}

	key := fmt.Sprintf("%d:%d:%d", st.Dev, st.Rdev, st.Ino)
	_, ok = s.seen.LoadOrStore(key, st)
}

// track this inode to detect loops; return true if we've seen it before
// false otherwise.
func (s *symlinkResolver) isEntrySeen(nm string, fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	key := fmt.Sprintf("%d:%d:%d", st.Dev, st.Rdev, st.Ino)
	x, ok := s.seen.LoadOrStore(key, st)
	if !ok {
		return false
	}

	// This can't fail because we checked it above before storing in the
	// sync.Map
	xt := x.(*syscall.Stat_t)

	return xt.Dev == st.Dev && xt.Rdev == st.Rdev && xt.Ino == st.Ino
}
