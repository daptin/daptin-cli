BINARY=daptin-cli
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test e2e clean release

build:
	go build -mod vendor -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) .

test:
	go test -mod vendor -race ./...

e2e:
	@bash scripts/e2e.sh

clean:
	rm -f $(BINARY)
	rm -rf out/ dist/

release:
	@if [ -z "$(TAG)" ]; then echo "usage: make release TAG=v0.3.0"; exit 1; fi
	git tag $(TAG)
	git push origin master $(TAG)
	@echo "Release $(TAG) pushed. CI will build and publish."
