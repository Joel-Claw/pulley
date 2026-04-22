FROM debian:bookworm-slim AS debian-test

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    golang-go \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/autopull
COPY . .

RUN go build -ldflags="-s -w" -o /usr/local/bin/autopull ./cmd/autopull

RUN useradd -m -s /bin/bash testuser

USER testuser
WORKDIR /home/testuser

COPY docker-test.sh /docker-test.sh

ENTRYPOINT ["/bin/bash", "/docker-test.sh"]