APP := taillaunch
PKG := ./cmd/taillaunch

.PHONY: test test-stub build fmt

fmt:
	gofmt -w cmd internal

test:
	go test ./...

test-stub:
	go test -tags stub ./...

build:
	go build -trimpath -o $(APP) $(PKG)
