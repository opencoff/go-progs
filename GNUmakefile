
progs = ifaddr finddup deadlinks hexlify ghash godu
arch := $(shell ./build --print-arch)
bindir = ./bin/$(arch)

# Always use a command line provided install-dir
ifneq ($(INSTALLDIR),)
    tooldir = $(INSTALLDIR)
else
    tooldir = $(HOME)/bin/$(arch)
endif

gofmt = $(addsuffix .gofmt, $(progs))

.PHONY: clean all $(tooldir) $(progs) gofmt $(gofmt)

all: $(progs)

install: $(progs) $(tooldir)
	for p in $(progs); do \
		cp $(bindir)/$$p $(tooldir)/ ; \
	done

$(progs):
	./build -s ./$@

$(tooldir):
	-mkdir -p $(tooldir)

clean:
	-rm -rf ./bin

fmt gofmt: $(gofmt)

$(gofmt):
	(cd $(basename $@) && gofmt -w -s *.go)
