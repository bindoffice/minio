PWD := $(shell pwd)
GOPATH := $(shell go env GOPATH)
LDFLAGS := $(shell go run buildscripts/gen-ldflags.go)

GOARCH := $(shell go env GOARCH)
GOOS := $(shell go env GOOS)

# v2.4.0+ is required for Go 1.25 export data format support.
GOLANGCI_VERSION ?= v2.4.0
GOLANGCI_LINT := $(GOPATH)/bin/golangci-lint

VERSION ?= $(shell git describe --tags)
TAG ?= "bindoffice/bindstore:$(VERSION)"

all: build

checks:
	@echo "Checking dependencies"
	@(env bash $(PWD)/buildscripts/checkdeps.sh)

getdeps:
	@mkdir -p ${GOPATH}/bin
	@if [ ! -x "$(GOLANGCI_LINT)" ] || ! $(GOLANGCI_LINT) version 2>/dev/null | grep -qE 'version 2\.'; then \
		echo "Installing golangci-lint $(GOLANGCI_VERSION)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin $(GOLANGCI_VERSION); \
	fi
	@which msgp 1>/dev/null || (echo "Installing msgp" && go get github.com/tinylib/msgp@v1.1.3)
	@which stringer 1>/dev/null || (echo "Installing stringer" && go get golang.org/x/tools/cmd/stringer)

crosscompile:
	@(env bash $(PWD)/buildscripts/cross-compile.sh)

verifiers: getdeps lint check-gen

check-gen:
	@go generate ./... >/dev/null
	@(! git diff --name-only | grep '_gen.go$$') || (echo "Non-committed changes in auto-generated code is detected, please commit them to proceed." && false)

lint:
	@echo "Running $@ check"
	@GO111MODULE=on $(GOLANGCI_LINT) cache clean
	@GO111MODULE=on $(GOLANGCI_LINT) run --config ./.golangci.yml

# Builds bindstore, runs the verifiers then runs the tests.
check: test
test: verifiers build
	@echo "Running unit tests"
	@GOGC=25 GO111MODULE=on CGO_ENABLED=0 go test -tags kqueue ./... 1>/dev/null

test-race: verifiers build
	@echo "Running unit tests under -race"
	@(env bash $(PWD)/buildscripts/race.sh)

# Verify bindstore binary
verify:
	@echo "Verifying build with race"
	@GO111MODULE=on CGO_ENABLED=1 go build -tags kqueue -trimpath --ldflags "$(LDFLAGS)" -o $(PWD)/bindstore 1>/dev/null
	@(env bash $(PWD)/buildscripts/verify-build.sh)

# Verify healing of disks with bindstore binary
verify-healing:
	@echo "Verify healing build with race"
	@GO111MODULE=on CGO_ENABLED=1 go build -race -tags kqueue -trimpath --ldflags "$(LDFLAGS)" -o $(PWD)/bindstore 1>/dev/null
	@(env bash $(PWD)/buildscripts/verify-healing.sh)

# Builds bindstore locally.
build: checks
	@echo "Building bindstore binary to './bindstore'"
	@GO111MODULE=on CGO_ENABLED=0 go build -tags kqueue -trimpath --ldflags "$(LDFLAGS)" -o $(PWD)/bindstore 1>/dev/null

hotfix-vars:
	$(eval LDFLAGS := $(shell MINIO_RELEASE="RELEASE" MINIO_HOTFIX="hotfix.$(shell git rev-parse --short HEAD)" go run buildscripts/gen-ldflags.go $(shell git describe --tags --abbrev=0 | \
    sed 's#RELEASE\.\([0-9]\+\)-\([0-9]\+\)-\([0-9]\+\)T\([0-9]\+\)-\([0-9]\+\)-\([0-9]\+\)Z#\1-\2-\3T\4:\5:\6Z#')))
	$(eval TAG := "bindoffice/bindstore:$(shell git describe --tags --abbrev=0).hotfix.$(shell git rev-parse --short HEAD)")
hotfix: hotfix-vars install

docker-hotfix: hotfix checks
	@echo "Building bindstore docker image '$(TAG)'"
	@docker build -t $(TAG) . -f Dockerfile.dev

docker: build checks
	@echo "Building bindstore docker image '$(TAG)'"
	@docker build -t $(TAG) . -f Dockerfile.dev

# Builds bindstore and installs it to $GOPATH/bin.
install: build
	@echo "Installing bindstore binary to '$(GOPATH)/bin/bindstore'"
	@mkdir -p $(GOPATH)/bin && cp -f $(PWD)/bindstore $(GOPATH)/bin/bindstore
	@echo "Installation successful. To learn more, try \"bindstore --help\"."

clean:
	@echo "Cleaning up all the generated files"
	@find . -name '*.test' | xargs rm -fv
	@find . -name '*~' | xargs rm -fv
	@rm -rvf bindstore
	@rm -rvf build
	@rm -rvf release
	@rm -rvf .verify*
