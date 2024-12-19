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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/opencoff/go-fio"
	"runtime"
)

const _parallelism int = 2

var nWorkers = runtime.NumCPU() * _parallelism

// iterate over the names
func processArgs(args []string, followSymlinks bool, apply func(*fio.Info) error) error {
	nw := nWorkers
	if len(args) < nw {
		nw = len(args)
	}

	ch := make(chan *fio.Info, nWorkers)
	errch := make(chan error, 1)

	// iterate in the background and feed the workers
	go func(ch chan *fio.Info, errch chan error) {
		var sr symlinkResolver

		for _, nm := range args {
			fi, err := fio.Lstat(nm)
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
				ch <- fi

			default:
				errch <- fmt.Errorf("skipping non-file %s..", nm)
			}
		}
		close(ch)
	}(ch, errch)

	// now start workers and process entries
	var wrkWait, errWait sync.WaitGroup
	var err error

	errWait.Add(1)
	go func(e *error, ch chan error) {
		var errs []error
		for err := range ch {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			*e = errors.Join(errs...)
		}
		errWait.Done()
	}(&err, errch)

	wrkWait.Add(nw)
	for i := 0; i < nw; i++ {
		go func(in chan *fio.Info, errch chan error) {
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

	return err
}

type symlinkResolver struct {
	seen sync.Map
}

func (s *symlinkResolver) resolve(nm string, fi *fio.Info) (string, *fio.Info, error) {
	newnm, err := filepath.EvalSymlinks(nm)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", nm, err)
	}
	nm = newnm

	// we know this is no longer a symlink
	fi, err = fio.Stat(nm)
	if err != nil {
		return "", nil, fmt.Errorf("stat %s: %w", nm, err)
	}

	// If we've seen this inode before, we are done.
	if s.isEntrySeen(nm, fi) || !fi.Mode().IsRegular() {
		return "", nil, nil
	}

	return nm, fi, nil
}

func (s *symlinkResolver) track(nm string, fi *fio.Info, errch chan error) {
	key := fmt.Sprintf("%d:%d:%d", fi.Dev, fi.Rdev, fi.Ino)
	s.seen.LoadOrStore(key, fi)
}

// track this inode to detect loops; return true if we've seen it before
// false otherwise.
func (s *symlinkResolver) isEntrySeen(nm string, fi *fio.Info) bool {
	key := fmt.Sprintf("%d:%d:%d", fi.Dev, fi.Rdev, fi.Ino)
	x, ok := s.seen.LoadOrStore(key, fi)
	if !ok {
		return false
	}

	xt := x.(*fio.Info)

	return xt.Dev == fi.Dev && fi.Rdev == fi.Rdev && fi.Ino == fi.Ino
}
