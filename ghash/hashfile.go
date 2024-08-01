// hashfile.go -- hash a file and return its strong checksum
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

	"github.com/opencoff/go-mmap"
	"hash"
)

// hash a file and return the checksum, file-size and error
func hashFile(fn string, hgen func() hash.Hash) ([]byte, int64, error) {
	fd, err := os.Open(fn)
	if err != nil {
		return nil, 0, err
	}
	defer fd.Close()

	h := hgen()
	if h == nil {
		panic(fmt.Sprintf("nil hash!"))
	}

	sz, err := mmap.Reader(fd, func(b []byte) error {
		h.Write(b)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return h.Sum(nil)[:], sz, nil
}
