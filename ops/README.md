# Self-hosting Illarin

This directory contains a reference deployment for running Illarin on one Linux
host with Docker Compose. It is not tied to a hosting provider. Adapt the edge
proxy, DNS, registry, monitoring, and backup destination to suit the environment
you operate.

> [!WARNING]
> The production stack is an operator starting point, not a managed service.
> Review every example value, protect the host, and prove that a backup can be
> restored before serving real data.

## Architecture

Traffic should reach a TLS-terminating reverse proxy before it reaches Illarin:

```text
Internet -> DNS and TLS proxy -> Illarin gateway -> web and API -> PostgreSQL
```

The Compose stack runs PostgreSQL, the Go API, the Next.js site, an internal
nginx gateway, and a Datadog agent. Uploaded blobs remain on the host. nginx may
serve a blob only after the API authorizes it with `X-Accel-Redirect`.

The included deployment has these current integration requirements:

- a container registry that holds `illarin-api` and `illarin-web` images tagged
  with full Git commit SHAs;
- Microsoft Graph application credentials and one Microsoft 365 sender mailbox
  for account email;
- a Datadog API key for the monitoring service in `compose.prod.yaml`;
- an off-host, restic-compatible repository for production backups.

Discord sign-in is optional. NPMPlus is also optional: `compose.npmplus.yaml`
only joins the gateway to an existing NPMPlus network when `NPMPLUS_NETWORK` is
set. Another reverse proxy can forward to the configured gateway address and
port instead.

## Prerequisites

- a Linux host with Docker Engine, the Docker Compose plugin, `flock`, and SSH;
- a DNS name and a TLS-terminating reverse proxy;
- a GitHub fork or another way to build and publish both application images;
- Microsoft 365 and Datadog credentials for the integrations above;
- enough persistent storage for PostgreSQL, uploads, image replacement, and the
  configured free-space reserve.

## Configure the host

Copy `ops/production.env.example` to `/etc/illarin/production.env`, set its mode
to `600`, and replace every placeholder. Keep secret files outside the checkout:

```bash
sudo install -d -m 0750 /etc/illarin/secrets
sudo install -m 0600 microsoft-365-client-secret \
  /etc/illarin/secrets/microsoft-365-client-secret
sudo install -m 0600 restic-password \
  /etc/illarin/secrets/restic-password
```

`ILLARIN_IMAGE_REGISTRY` is the namespace before the image name. For a GitHub
fork owned by `example`, use `ghcr.io/example`; the workflows publish
`ghcr.io/example/illarin-api:<commit>` and
`ghcr.io/example/illarin-web:<commit>`.

Generate `LINKING_HMAC_KEY` as 32 random bytes encoded as unpadded base64url.
Use a separate, randomly generated PostgreSQL password and update both
`POSTGRES_PASSWORD` and `DATABASE_URL` with the same value.

The interactive helper covers the reference GitHub, Microsoft 365, Datadog,
SSH, and NPMPlus path:

```bash
make production-setup
```

It writes local setup material under `ops/`; those files are ignored by Git.
The helper is optional. Operators may configure the same values and secrets with
their own provisioning system.

## Publish and deploy

GitHub Actions runs CI on every branch. A successful push to the repository's
default branch publishes immutable images. The manual production workflow only
deploys when it is run from that default branch.

Configure the `production` GitHub environment with:

| Name | Kind | Purpose |
| --- | --- | --- |
| `PRODUCTION_URL` | variable | Public site URL shown by GitHub |
| `PRODUCTION_HOST` | secret | Hostname or address used by SSH |
| `PRODUCTION_USER` | secret | Deployment user |
| `PRODUCTION_SSH_PORT` | secret | SSH port |
| `PRODUCTION_SSH_KEY` | secret | Private Ed25519 deployment key |
| `PRODUCTION_HOST_KEYS` | secret | Verified `known_hosts` entry |

The deployment workflow copies only the control files, selects images by the
full commit SHA, applies forward database migrations, waits for health checks,
and runs smoke checks. Application rollback returns to the previous images; it
does not reverse a database migration.

For a manual deployment from an installed release directory:

```bash
make prod-config-check
make prod-deploy VERSION=<full-lowercase-commit-sha>
make prod-smoke
```

Useful operating commands are listed by `make help`. In particular:

```bash
make prod-status
make prod-logs SERVICE=api
make prod-restart SERVICE=api
make prod-rollback
```

## Backups and recovery

The backup job writes a PostgreSQL dump first, then uploads that dump and the
immutable blob directory to restic while blob deletion is locked. Derivatives
are disposable and are not backed up. Retention defaults to 30 daily snapshots.

Configure `RESTIC_REPOSITORY`, the standard `AWS_*` credentials required by an
S3-compatible destination, and the `restic-password` secret. Then initialize,
check, and exercise the repository before enabling scheduled backups:

```bash
make prod-backup-init
make prod-backup
make prod-backup-check
```

Set `BACKUPS_ENABLED=true` only after those commands succeed. Restore a snapshot
into an isolated directory and scratch PostgreSQL instance, apply its
`database.dump` with `pg_restore`, and verify database rows against the restored
blob sizes and hashes. Do not make the first restore attempt during an outage or
write a rehearsal over production data.

Monitor backup failures and age, disk space, container health, certificate
expiry, and Microsoft application-secret expiry. A healthy deployment without
a tested off-host restore is not recoverable.
