[![GoDoc](https://godoc.org/github.com/opencoff/go-progs?status.svg)](https://godoc.org/github.com/opencoff/go-progs)

# README for go-progs


## What is this?
This is a collection of `golang` programs I wrote for my daily use. 
Most of them walk the file system and do interesting things - the FS
walk being done in parallel/concurrent manner. These tools use the
concurrent FS walker library [go-walk](https://github.com/opencoff/go-walk).

## Brief Description of the tools

* `godu` -- an opinionated and simpler reimplementation of du(1).
* `ghash` -- walk a file system and generate strong checksums of
  every file. Supports a variety of cryptographic checksums. 
* `finddup` -- finds duplicate files in a file system tree; files
  with the same strong checksum are considered identical.
* `deadlinks` -- walk the file system and find dead symlinks.
* `hexlify` -- print the contents of files/stdin in a variety of
  encoded formats (hex, base64, hexdump(1) style etc.)
* `ifaddr` - prints the network interfaces and their IP addresses.


All the tools have their own "help" accessible via the `-h` or
`--help` command line option.

## How do I build it?
You'll need GNUmake 4.0 or later and a golang 1.21 or later:

    git clone https://github.com/opencoff/go-progs
    cd go-progs
    gmake

The binaries will be in `./bin/$HOSTOS-$ARCH/`
where `$HOSTOS` is the host OS where you are building (e.g., openbsd)
and `$ARCH` is the CPU architecture (e.g., amd64).

If you *do not* have GNUmake, you can still build each program;
eg.,:

    ./build -s ./godu

## Licensing Terms
The code and tools in this repository is licensed under the terms of the
GNU Public License v2.0 (strictly v2.0). If you need a commercial
license or a different license, please get in touch with me.

See the file ``LICENSE`` for the full terms of the license.
