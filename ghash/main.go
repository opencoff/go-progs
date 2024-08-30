// main.go -- Tool to generate & verify various hashes
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
	"io"
	"os"
	"path"
	"sync"

	"github.com/opencoff/go-utils"
	"github.com/opencoff/go-walk"
	flag "github.com/opencoff/pflag"

	"crypto/sha256"
	"crypto/sha512"
	"github.com/zeebo/blake3"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/sha3"
	"hash"
)

// ghash output file magic
const MAGIC = "#!ghash"

// basename of argv[0]
var Z string = path.Base(os.Args[0])

type otuple struct {
	nm  string
	sz  int64
	sum []byte
}

func main() {
	var ver, help, recurse, onefs, follow, force bool
	var verify, output, halgo string
	var listHashes bool

	mf := flag.NewFlagSet(Z, flag.ExitOnError)
	mf.BoolVarP(&ver, "version", "V", false, "Show version info and exit")
	mf.BoolVarP(&help, "help", "h", false, "Show help info exit")
	mf.BoolVarP(&recurse, "recurse", "r", false, "Recursively traverse directories")
	mf.BoolVarP(&onefs, "one-filesystem", "x", false, "Don't cross file system boundaries")
	mf.BoolVarP(&follow, "follow-symlinks", "L", false, "Follow symlinks")
	mf.BoolVarP(&listHashes, "list-hashes", "", false, "List supported hash algorithms")
	mf.BoolVarP(&force, "force-overwrite", "f", false, "Forcibly overwrite output file")
	mf.StringVarP(&halgo, "hash", "H", "sha256", "Use hash algorithm `H`")
	mf.StringVarP(&verify, "verify-from", "v", "", "Verify the hashes in file 'F' [stdin]")
	mf.StringVarP(&output, "output", "o", "", "Write hashes to file 'F' [stdout]")
	mf.Parse(os.Args[1:])

	if ver {
		fmt.Printf("%s - %s [%s]\n", Z, ProductVersion, RepoVersion)
		Exit(0)
	}

	if help {
		usage(0)
	}

	if listHashes {
		printHashes()
		Exit(0)
	}

	if len(verify) > 0 {
		exit := doVerify(verify)
		Exit(exit)
	}

	args := mf.Args()
	if len(args) < 1 {
		Die("Insufficient arguments. Try '%s -h'", Z)
	}

	h, ok := Hashes[halgo]
	if !ok {
		Die("Unknown hash algorithm '%s'. Try '%s --list-hashes'", halgo, Z)
	}

	var fd io.WriteCloser = os.Stdout

	if len(output) > 0 {
		var opt uint32
		if force {
			opt |= utils.OPT_OVERWRITE
		}
		fx, err := utils.NewSafeFile(output, opt, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			Die("%s", err)
		}
		fd = fx

		AtExit(fx.Abort)
		defer fx.Abort()
	}

	fmt.Fprintf(fd, "%s %s %s\n", MAGIC, halgo, ProductVersion)

	var wg sync.WaitGroup
	ch := make(chan otuple, 16)
	action := func(r walk.Result) error {
		sum, sz, err := hashFile(r.Path, h)
		if err != nil {
			return err
		}

		ch <- otuple{r.Path, sz, sum}
		return nil
	}

	wg.Add(1)
	go func(ch chan otuple, fd io.WriteCloser, wg *sync.WaitGroup) {
		defer wg.Done()
		for o := range ch {
			_, err := fmt.Fprintf(fd, "%x|%d|%s\n", o.sum, o.sz, o.nm)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return
			}
		}
		fd.Close()
	}(ch, fd, &wg)

	var err error

	switch recurse {
	case true:
		opt := walk.Options{
			FollowSymlinks: follow,
			OneFS:          onefs,
			Type:           walk.FILE,
		}

		err = walk.WalkFunc(args, &opt, action)

	case false:
		err = processArgs(args, follow, action)
	}

	close(ch)

	if err != nil {
		Warn("%s", err)
	}

	wg.Wait()
	if err != nil {
		Exit(1)
	}
	Exit(0)
}

func printHashes() {
	fmt.Printf("%s: Available hash algorithms:\n", Z)
	for k := range Hashes {
		fmt.Printf("   %s\n", k)
	}
}

var Hashes = map[string]func() hash.Hash{
	"sha256":   func() hash.Hash { return sha256.New() },
	"sha512":   func() hash.Hash { return sha512.New() },
	"sha3":     func() hash.Hash { return sha3.New512() },
	"sha3-256": func() hash.Hash { return sha3.New256() },
	"sha3-512": func() hash.Hash { return sha3.New512() },
	"blake2s":  func() hash.Hash { return keyedHashGen1(blake2s.New256) },

	"blake2b":     func() hash.Hash { return keyedHashGen1(blake2b.New512) },
	"blake2b-256": func() hash.Hash { return keyedHashGen1(blake2b.New256) },
	"blake2b-512": func() hash.Hash { return keyedHashGen1(blake2b.New512) },

	"blake3": func() hash.Hash { return keyedHashGen2(blake3.NewKeyed) },
}

func keyedHashGen1(hg func(key []byte) (hash.Hash, error)) hash.Hash {
	var zeroes [32]byte
	h, err := hg(zeroes[:])
	if err != nil {
		panic(fmt.Sprintf("%v: %s", hg, err))
	}
	return h
}

func keyedHashGen2(hg func(key []byte) (*blake3.Hasher, error)) hash.Hash {
	var zeroes [32]byte
	h, err := hg(zeroes[:])
	if err != nil {
		panic(fmt.Sprintf("%v: %s", hg, err))
	}
	return h
}

func usage(c int) {
	x := fmt.Sprintf(`%s is a tool to generate and verify various hashes on files

Usage: %s [options] file|dir [file|dir ..]

Options:
  -h, --help            Show help and exit
  -V, --version         Show version info and exit
  -r, --recurse	        Recursively traverse directories
  -x, --one-filesystem  Don't cross file system boundaries
  -L, --follow-symlinks Follow symbolic links
  -H, --hash=H		Use hash algorithm 'H' [sha256]
  --list-hashes		List supported hash algorithms
  -v, --verify-from=F   Verify the hashes in file 'F' [stdin]
  -o, --output=O        Write output hashes to file 'O' [stdout]
`, Z, Z)

	os.Stdout.Write([]byte(x))
	Exit(c)
}

// This will be filled in by "build"
var RepoVersion string = "UNDEFINED"
var ProductVersion string = "UNDEFINED"
