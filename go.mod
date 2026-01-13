module go-progs

go 1.25.5

//replace github.com/opencoff/go-fio v0.5.15 => ../go-fio

require (
	github.com/opencoff/go-fio v0.5.16
	github.com/opencoff/go-mmap v0.1.7
	github.com/opencoff/go-utils v1.0.6
	github.com/opencoff/pflag v1.0.7
	github.com/puzpuzpuz/xsync/v3 v3.5.1
	github.com/zeebo/blake3 v0.2.4
	golang.org/x/crypto v0.47.0
)

require (
	github.com/klauspost/cpuid/v2 v2.0.12 // indirect
	github.com/pkg/xattr v0.4.10 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/term v0.39.0 // indirect
)
