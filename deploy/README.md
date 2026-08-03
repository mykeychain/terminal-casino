# Deploying Terminal Casino

How to run the casino locally and how to host it on a free-tier **GCP e2-micro** so
people can reach it with `ssh terminal-casino.<yourdomain>`.

---

## 1. Run locally (single terminal)

Play in your own terminal — lobby → game, no networking:

```
go run ./cmd/casino
```

## 2. Run the SSH server locally

Serve the lobby over SSH on your own machine, then connect from another terminal:

```
go run ./cmd/casino-ssh          # listens on :23234
ssh -p 23234 localhost           # lands in the lobby
```

Flags (each also settable via a `CASINO_SSH_*` env var): `-addr` (listen address,
env `CASINO_SSH_ADDR`, default `:23234`), `-host-key` (path, generated on first run,
default `.ssh/casino_ed25519`), `-idle-timeout` (reap idle sessions, default `15m`,
`0` disables), and `-max-sessions` (concurrent-session cap with a "full" message,
default `50`, `0` = unlimited). Connects/disconnects are logged with the live session
count and duration.
Ctrl+C stops it gracefully.

---

## 3. Host on a GCP e2-micro (free tier)

End goal: `ssh terminal-casino.<yourdomain>` (or a `~/.ssh/config` alias) drops a
player straight into the lobby, on the default SSH port 22.

### 3a. Create the instance + a static IP

e2-micro is always-free in `us-west1`, `us-central1`, or `us-east1` (1 per month).

```
# reserve a static external IP (keeps the address stable for DNS)
gcloud compute addresses create casino-ip --region=us-central1

gcloud compute instances create casino \
  --zone=us-central1-a \
  --machine-type=e2-micro \
  --image-family=debian-12 --image-project=debian-cloud \
  --address=casino-ip
```

Note the external IP: `gcloud compute instances describe casino --zone=us-central1-a --format='get(networkInterfaces[0].accessConfigs[0].natIP)'`.

### 3b. Free port 22 for the game (move admin SSH aside)

The box's own `sshd` owns port 22, which we want for the game. Move admin SSH to
2222 **with a safety net** so you can't lock yourself out:

```
# on the instance (via `gcloud compute ssh casino` or the console)
sudo sed -i 's/^#\?Port .*/Port 22\nPort 2222/' /etc/ssh/sshd_config
sudo systemctl restart ssh          # now listening on BOTH 22 and 2222
```

Open 2222 to yourself and verify it works **before** dropping 22:

```
# from your laptop
gcloud compute firewall-rules create allow-admin-ssh \
  --allow=tcp:2222 --source-ranges=YOUR_IP/32 --network=default
ssh -p 2222 <you>@<instance-ip>     # confirm this connects
```

Once 2222 works, make admin SSH use *only* 2222 so 22 is free:

```
sudo sed -i '/^Port 22$/d' /etc/ssh/sshd_config
sudo systemctl restart ssh
```

(If anything goes wrong, GCP's **Serial console** / **browser SSH** can still get you
in.) From now on you administer the box with `ssh -p 2222`, and `gcloud compute ssh`
with `--ssh-flag="-p 2222"`.

### 3c. Install the binary + service

Build a Linux binary (locally, then copy — e2 is x86-64):

```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o casino-ssh ./cmd/casino-ssh
gcloud compute scp casino-ssh deploy/casino-ssh.service casino:~ --zone=us-central1-a --ssh-flag="-p 2222"
```

On the instance:

```
sudo useradd --system --no-create-home --shell /usr/sbin/nologin casino
sudo mkdir -p /opt/terminal-casino && sudo mv ~/casino-ssh /opt/terminal-casino/
sudo mv ~/casino-ssh.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now casino-ssh
journalctl -u casino-ssh -f            # watch it start / see connections
```

The unit binds `:22` via `CAP_NET_BIND_SERVICE` (no full root) and persists the host
key under `/var/lib/terminal-casino/`.

### 3d. Firewall for the game

Port 22 is already open on GCP's default network (`default-allow-ssh`, `0.0.0.0/0`) —
that now reaches the game. If you deleted that rule, re-add `tcp:22` from `0.0.0.0/0`.

### 3e. Connect

```
ssh player@<instance-ip>               # any username works; access is anonymous
```

**Clean name — two options:**
- **DNS (public):** add an `A` record `terminal-casino.<yourdomain>` → the static IP.
  Then anyone runs `ssh terminal-casino.<yourdomain>`.
- **Local alias (just you):** in `~/.ssh/config`:
  ```
  Host terminal-casino
      HostName <instance-ip-or-domain>
      User player
      Port 22
  ```
  Then `ssh terminal-casino`.

---

## 4. Updating

```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o casino-ssh ./cmd/casino-ssh
gcloud compute scp casino-ssh casino:~ --zone=us-central1-a --ssh-flag="-p 2222"
sudo mv ~/casino-ssh /opt/terminal-casino/ && sudo systemctl restart casino-ssh
```

## 5. Container option

See `deploy/Dockerfile`. Build/run:

```
docker build -f deploy/Dockerfile -t terminal-casino .
docker run --rm -p 23234:23234 -v casino-hostkey:/data terminal-casino
```

Map `-p 22:23234` for the default port (the host must not already use 22).

---

## Security notes

- **Access is anonymous** — anyone who reaches the port can play. That's fine for
  fake-money blackjack; just know it's open. No shell escape: connections only run the
  TUI (the `activeterm` middleware also rejects non-interactive sessions).
- Keep the **admin SSH** port (2222) restricted to your own IP in the firewall.
- The **host key persists** (`/var/lib/terminal-casino/`), so returning players don't
  get "host key changed" warnings.
- **Abuse limits are built in.** Idle sessions are reaped after 15m and concurrent
  sessions are capped at 50 by default (over-cap connections get a "casino is full"
  message). Tune both with `-idle-timeout` / `-max-sessions` (or the `CASINO_SSH_*`
  env vars) — see the flags in §2. Connects/disconnects are logged with the live
  session count and duration (`journalctl -u casino-ssh`).
