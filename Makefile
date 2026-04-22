.PHONY: build install test clean

BINARY=autopull
VERSION?=0.1.0
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/autopull

install: build
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
	install -m 644 install/autopull.service /etc/systemd/system/autopull.service
	systemctl daemon-reload

test:
	go test -v ./cmd/autopull/

clean:
	rm -f $(BINARY)