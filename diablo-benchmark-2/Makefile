GOBIN := go
BUILDFLAGS := -v
PKG := "diablo-benchmark"
PKGFOLDERS := blockchains/... communication/... core/...

# GOPATH=$(PWD)/.go
# export GOPATH

default: diablo
	./diablo primary --verbose=trace --stat --env=accounts=algorand-accounts.yaml --env=contracts=teal-contracts 1 setup.yaml benchmark.yaml

all: lint diablo

reqs:
	GO111MODULE=off GO111MODULE=off go get -v golang.org/x/lint/golint
	$(GOBIN) mod download
	$(GOBIN) mod vendor

lint:
	@golint -set_exit_status $(PKGFOLDERS)

diablo:
	$(GOBIN) build $(BUILDFLAGS) -o $@

clean:
	-rm diablo

cleanall: clean
	-chmod -R 700 $(PWD)/.go
	-rm -rf $(PWD)/.go

.PHONY: default clean cleanall reqs diablo
