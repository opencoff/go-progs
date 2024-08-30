// hexlify.go - hexdump, base64 and so on
//
// Author: Sudhi Herle (sw@herle.net)
// License: GPLv2
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/opencoff/go-mmap"
	"github.com/opencoff/go-utils"
	flag "github.com/opencoff/pflag"
)

var Z string = path.Base(os.Args[0])

const _BUFSZ int = 65536

func main() {
	var version bool
	var count uint
	var out string

	flag.BoolVarP(&version, "version", "", false, "Show version info and quit")
	flag.UintVarP(&count, "count", "n", 0, "Read `N` bytes of each input (0 implies 'till EOF')")
	flag.StringVarP(&out, "outfile", "o", "-", "Write output to file `F`")

	flag.Usage = func() {
		fmt.Printf(
			`%s - dump input into b64, hex or 'C'

Usage: %s [options] mode [input]

Where mode is one of:

	b64, base64:	  output in base64 (standard encoding)
	hex, x:           output in "raw" hex
	hexdump, dump, d: mimic hexdump(1) output
	C, struct:        output C like array definition

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
		Die("Insufficient arguments. Try '%s --help'", Z)
	}

	var wr io.WriteCloser = os.Stdout

	if len(out) > 0 && out != "-" {
		wfd, err := utils.NewSafeFile(out, 0, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			Die("can't create %s: %s", out, err)
		}
		wr = wfd
		AtExit(wfd.Abort)
		defer wfd.Abort()
	}

	var mkdump func(wr io.Writer, fn string) dumper
	mode := strings.ToLower(args[0])
	switch mode {
	case "b64", "base64":
		mkdump = func(w io.Writer, fn string) dumper {
			return NewFlexDumper(w, fn, encB64)
		}

	case "c", "struct":
		mkdump = NewCdumper

	case "hex", "x":
		mkdump = func(w io.Writer, fn string) dumper {
			return NewFlexDumper(w, fn, encRawhex)
		}

	case "dump", "d", "hexdump":
		mkdump = NewHexDumper

	default:
		Die("unknown encoding type '%s'", mode)
	}

	hexlate := func(wr io.Writer, src io.Reader, fn string) {
		dd := mkdump(wr, fn)
		defer func(d dumper) {
			err := d.Close()
			if err != nil {
				Warn("%s", err)
			}
		}(dd)

		if fd, ok := src.(*os.File); ok && mmapable(fd) {
			if count > 0 {
				mm := mmap.New(fd)
				m, err := mm.Map(int64(count), 0, mmap.PROT_READ, 0)
				if err != nil {
					Warn("%s: %s", fd.Name(), err)
					return
				}
				defer m.Unmap()
				b := m.Bytes()

				if err = dd.Write(b); err != nil {
					Warn("%s", err)
				}
				return
			}

			_, err := mmap.Reader(fd, func(b []byte) error {
				return dd.Write(b)
			})

			if err != nil {
				Warn("%s", err)
			}

			return
		}

		if count > 0 {
			src = io.LimitReader(src, int64(count))
		}

		buf := make([]byte, _BUFSZ)
		for {
			m, err := src.Read(buf)
			if m == 0 || err == io.EOF {
				return
			}

			if err != nil {
				Warn("%s: %s", fn, err)
				return
			}

			if err = dd.Write(buf[:m]); err != nil {
				Warn("%s", err)
			}
		}
	}

	// Now process the input
	args = args[1:]
	if len(args) > 0 {
		fn := args[0]
		fd, err := os.Open(fn)
		if err != nil {
			Die("%s", err)
		}
		hexlate(wr, fd, fn)
		fd.Close()
	} else {
		hexlate(wr, os.Stdin, "<stdin>")
	}

	// without this - the output file will be deleted on exit.
	wr.Close()
}

// return true if an open file can be memory mapped
func mmapable(fd *os.File) bool {
	st, err := fd.Stat()
	if err != nil {
		return false
	}

	return st.Mode().IsRegular() && st.Size() > 0
}

type dumper interface {
	Write([]byte) error
	Close() error
}

type hexDumper struct {
	wr io.Writer
	fn string
	hd io.WriteCloser
}

func NewHexDumper(wr io.Writer, fn string) dumper {
	hd := hex.Dumper(wr)
	d := &hexDumper{
		wr: wr,
		fn: fn,
		hd: hd,
	}
	return d
}

func (d *hexDumper) Write(b []byte) error {
	return write(d.fn, d.hd, b)
}

func (d *hexDumper) Close() error {
	if err := d.hd.Close(); err != nil {
		return fmt.Errorf("%s: %s", d.fn, err)
	}
	return nil
}

type cDumper struct {
	wr      io.Writer
	fn      string
	bio     *bufio.Writer
	started bool
}

var _ dumper = &cDumper{}

func NewCdumper(wr io.Writer, fn string) dumper {
	bio := bufio.NewWriter(wr)
	d := &cDumper{
		wr:  wr,
		fn:  fn,
		bio: bio,
	}
	return d
}

func (d *cDumper) Write(b []byte) error {
	const linelen = 80
	const bpl = linelen / 5 // bytes per line

	bio := d.bio
	n := len(b)

	// handle the first byte separately
	if !d.started {
		s := fmt.Sprintf("{\n\t  %#2.2x", b[0])
		if _, err := bio.WriteString(s); err != nil {
			return fmt.Errorf("%s: %s", d.fn, err)
		}

		m := min(n, bpl)
		if err := d.writeLine(b[1:m]); err != nil {
			return err
		}
		n -= m
		b = b[m:]
		d.started = true
	}

	for n > 0 {
		m := min(n, bpl)
		if _, err := bio.WriteString("\n\t"); err != nil {
			return fmt.Errorf("%s: %s", d.fn, err)
		}
		if err := d.writeLine(b[:m]); err != nil {
			return err
		}

		n -= m
		b = b[m:]
	}
	if err := bio.Flush(); err != nil {
		return fmt.Errorf("%s: %s", d.fn, err)
	}
	return nil
}

func (d *cDumper) writeLine(b []byte) error {
	bio := d.bio
	for _, c := range b {
		s := fmt.Sprintf(", %#2.2x", c)
		if _, err := bio.WriteString(s); err != nil {
			return fmt.Errorf("%s: %s", d.fn, err)
		}
	}
	return nil
}

func (d *cDumper) Close() error {
	const s string = "\n}\n"
	b := []byte(s)
	return write(d.fn, d.wr, b)
}

type enctype int

const (
	encB64 enctype = iota
	encRawhex
)

// Dump b64 or raw-hex
type flexdump struct {
	wr  io.Writer
	fn  string
	buf []byte

	enc    func(dst, src []byte)
	enclen func(int) int
}

var _ dumper = &flexdump{}

func NewFlexDumper(wr io.Writer, fn string, ty enctype) dumper {
	buf := make([]byte, 3*_BUFSZ)
	d := &flexdump{
		wr:  wr,
		fn:  fn,
		buf: buf,
	}

	switch ty {
	case encB64:
		d.enc = base64.StdEncoding.Encode
		d.enclen = base64.StdEncoding.EncodedLen

	case encRawhex:
		d.enc = func(d, s []byte) { hex.Encode(d, s) }
		d.enclen = hex.EncodedLen

	default:
		panic("unknown encoding mode")
	}

	return d
}

func (d *flexdump) Write(b []byte) error {
	n := len(b)
	for n > 0 {
		m := min(n, _BUFSZ)
		z := d.enclen(m)
		d.enc(d.buf, b[:m])
		err := write(d.fn, d.wr, d.buf[:z])
		if err != nil {
			return err
		}
		n -= m
		b = b[m:]
	}
	return nil
}

func (d *flexdump) Close() error {
	fmt.Fprintf(d.wr, "\n")
	return nil
}

func write(fn string, wr io.Writer, b []byte) error {
	x, err := wr.Write(b)
	if err != nil {
		return fmt.Errorf("%s: %s", fn, err)
	}
	if len(b) != x {
		return fmt.Errorf("%s: partial write; exp %d, saw %d. Aborting ..", fn, len(b), x)
	}
	return nil
}

// This will be filled in by "build"
var RepoVersion string = "UNDEFINED"
var ProductVersion string = "UNDEFINED"

// vim: ft=go:sw=4:ts=4:noexpandtab:tw=78:
