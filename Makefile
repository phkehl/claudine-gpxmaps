# gpxmaps build helpers.
#
# Targets:
#   make build         full binary (CLI + GUI); needs Fyne system deps (see deps-debian)
#   make build-nogui   CLI-only, CGO-free binary
#   make windows       CLI-only Windows .exe (cross-compiled, no C toolchain)
#   make test fmt vet   checks
#   make deps-debian   install the GUI build dependencies on Debian/Ubuntu
#   make clean

BIN     := gpxmaps
PKG     := .
GOFLAGS :=

.PHONY: build
build:
	CGO_ENABLED=1 go build $(GOFLAGS) -o $(BIN) $(PKG)

.PHONY: build-nogui
build-nogui:
	CGO_ENABLED=0 go build -tags nogui $(GOFLAGS) -o $(BIN) $(PKG)

.PHONY: windows
windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags nogui $(GOFLAGS) -o $(BIN).exe $(PKG)

.PHONY: test
test:
	go test ./...

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: vet
vet:
	go vet ./...
	go vet -tags nogui ./...

.PHONY: deps-debian
deps-debian:
	sudo apt-get update
	sudo apt-get install -y libgl1-mesa-dev xorg-dev

.PHONY: clean
clean:
	rm -f $(BIN) $(BIN).exe
