# Containerized Terminal Casino SSH server.
#
# Build:  docker build -f deploy/Dockerfile -t terminal-casino .
# Run:    docker run --rm -p 23234:23234 -v casino-hostkey:/data terminal-casino
#         then: ssh -p 23234 localhost
#
# For a clean `ssh <host>` on port 22, map -p 22:23234 (the host's own sshd must
# not already own port 22 — see deploy/README.md), or run on a host where 22 is free.

# --- build stage ---
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/casino-ssh ./cmd/casino-ssh

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/casino-ssh /casino-ssh
# Host key persists on the mounted volume so it survives restarts.
VOLUME /data
EXPOSE 23234
ENTRYPOINT ["/casino-ssh", "-addr", ":23234", "-host-key", "/data/casino_ed25519"]
