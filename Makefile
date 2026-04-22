.PHONY: build install test clean

BINARY=pulley
VERSION?=0.1.0
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/pulley

install: build
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
	install -m 644 install/pulley.service /etc/systemd/system/pulley.service
	systemctl daemon-reload

test:
	go test -v ./cmd/pulley/

clean:
	rm -f $(BINARY)