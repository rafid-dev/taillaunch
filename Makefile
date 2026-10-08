APP := taillaunch
PKG := ./cmd/taillaunch
GUI_APP := taillaunch-gui
GUI_PKG := ./cmd/taillaunch-gui

.PHONY: test test-stub build build-gui fmt

fmt:
	gofmt -w cmd internal

test:
	go test ./...

test-stub:
	go test -tags stub ./...

build:
	go build -trimpath -o $(APP) $(PKG)

build-gui:
	go build -trimpath -o $(GUI_APP) $(GUI_PKG)
