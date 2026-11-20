# Local development and hosting

**Status:** Installation and maintenance procedures documented; execution on
the target LXC remains to be verified
**Last updated:** 2026-11-20

This is the step-by-step guide for building, installing, activating, updating,
and safely operating Finapp. The deployed service runs as the `finapp` system
user, listens only on the configured home-LAN address, and stores its SQLite
database outside the release package. The browser has no sign-in; preserve the
existing home-only network boundary.

## Requirements

- Operating system and version: Debian stable in a dedicated Proxmox LXC guest
- Release architecture: Linux AMD64 (`x86_64`)
- Runtime and version: Compiled Go application; Go toolchain needed to build,
  not to run a release binary
- Required local services: systemd; SQLite is embedded as a local database
  file
- Build and release instructions: `make check`; GitHub Actions creates a
  Linux AMD64 package for sequential `v1`, `v2`, ... tags on `main` using the
  [release workflow](../../.github/workflows/release.yml)
- Installation and activation: use the included
  [systemd unit](../../deploy/finapp.service) and
  [environment example](../../deploy/finapp.env.example) from the GitHub
  release package

## Prepare and publish a release

The release workflow runs for sequential tags such as `v1`, `v2`, and `v3`.
It only publishes a tag whose commit is already on `main`. The release asset is
`finapp-<version>-linux-amd64.tar.gz`; it contains `finapp`, `finapp.service`,
and `finapp.env.example`. No Go or Node.js runtime is needed in the guest.

On the development machine, update `main`, run the checks, choose the next
unused sequential version, and push that tag. Commit the intended release
changes to `main` first and ensure the worktree is clean:

```sh
set -eu
git switch main
git pull --ff-only origin main
git status --short
test -z "$(git status --porcelain)"
make check
make test-browser
git tag vN
git push origin vN
```

Replace `vN` with the next unused tag (for example, `v1`). Wait for the
**Release** workflow to complete, then confirm the matching Linux AMD64 asset
appears on the repository's GitHub Releases page. The current workflow does
not publish a checksum or signature; download the asset only from that
repository's release page.

## Prepare the Debian LXC

Use the dedicated Debian stable guest in Proxmox; do not install Finapp into
the Home Assistant OS guest. Configure the guest's network in Proxmox using
the already verified static address `192.168.68.71/22` and gateway
`192.168.68.1`. Preserve the existing home-only network policy for
`192.168.68.0/22`; do not add router port forwarding, a tunnel, or public
access. From a console in the guest, verify the address and route:

```sh
ip -4 address
ip route
```

The guest must have systemd, outbound HTTPS access to GitHub for release
downloads, and enough persistent storage for the database and backups. Log in
as `root` or open a root shell for the administrative commands below.

Install the small set of OS tools used to download and inspect releases and
backups:

```sh
apt-get update
apt-get install --yes ca-certificates curl tar sqlite3
```

## First installation

Choose a version that is already published on GitHub Releases. Set the version
in the following commands to that exact tag:

```sh
set -eu
VERSION=vN
PACKAGE="finapp-${VERSION}-linux-amd64.tar.gz"
DOWNLOAD="/tmp/${PACKAGE}"
STAGE="/tmp/finapp-${VERSION}"

curl --fail --location --show-error \
  "https://github.com/matthiayer/finapp/releases/download/${VERSION}/${PACKAGE}" \
  --output "${DOWNLOAD}"
mkdir -m 0700 "${STAGE}"
tar -xzf "${DOWNLOAD}" -C "${STAGE}"
test -x "${STAGE}/finapp"
test -f "${STAGE}/finapp.service"
test -f "${STAGE}/finapp.env.example"
```

Replace `vN` with the published version before running the block. The archive
should contain exactly those three files at its root. If download or archive
validation fails, do not install it.

Create the service account and persistent/configuration directories, install
the release files, and create the environment file only if one does not
already exist:

```sh
set -eu
VERSION=vN
STAGE="/tmp/finapp-${VERSION}"
getent group finapp >/dev/null || groupadd --system finapp
getent passwd finapp >/dev/null || \
  useradd --system --gid finapp --home-dir /var/lib/finapp \
    --no-create-home --shell /usr/sbin/nologin finapp
install -d --owner=finapp --group=finapp --mode=0700 /var/lib/finapp
install -d --owner=root --group=root --mode=0755 /etc/finapp
install -d --owner=root --group=root --mode=0755 /usr/local/share/finapp
install --owner=root --group=root --mode=0755 \
  "${STAGE}/finapp" /usr/local/bin/finapp
install --owner=root --group=root --mode=0644 \
  "${STAGE}/finapp.service" /etc/systemd/system/finapp.service
if test ! -e /etc/finapp/finapp.env; then
  install --owner=root --group=root --mode=0600 \
    "${STAGE}/finapp.env.example" /etc/finapp/finapp.env
fi
chown root:root /etc/finapp/finapp.env
chmod 0600 /etc/finapp/finapp.env
```

Inspect `/etc/finapp/finapp.env` and ensure it contains the deployment values:

```text
FINAPP_LISTEN_ADDR=192.168.68.71:8080
FINAPP_DB_PATH=/var/lib/finapp/finapp.db
```

The environment file is root-owned and mode `0600`. The systemd unit runs as
the unprivileged `finapp` user, creates `/var/lib/finapp` with mode `0700`,
and restricts writable paths to that directory. Enable and start the service:

```sh
systemctl daemon-reload
systemctl enable --now finapp.service
systemctl status --no-pager finapp.service
journalctl --unit=finapp.service --lines=50 --no-pager
```

The first successful start creates the database and applies embedded Goose
migrations. Verify health from the guest:

```sh
curl --fail --show-error http://192.168.68.71:8080/healthz
```

It should return `ok`. From a laptop on the home network, open
`http://192.168.68.71:8080`, confirm the dashboard loads, and verify that
`http://192.168.68.71:8080/healthz` returns `ok`. Confirm access remains
limited to the home network; do not expose the service to the public internet.
The app has no sign-in, so this network boundary protects all financial data.

After these checks pass, record the installed release version:

```sh
VERSION=vN
printf '%s\n' "${VERSION}" > /usr/local/share/finapp/version
chmod 0644 /usr/local/share/finapp/version
cat /usr/local/share/finapp/version
```

## Build and run locally

From the repository root:

```sh
make check
make run
```

For browser acceptance testing, install the JavaScript test dependencies and
Playwright's Chromium once, then run:

```sh
npm ci
npx playwright install --with-deps chromium
make test-browser
```

The browser suite uses synthetic bank-shaped CSV data, starts an isolated app
instance, and stores its temporary database outside the repository.

The development server binds to `127.0.0.1:8080` by default and creates
`data/finapp.db`. Override settings with `FINAPP_LISTEN_ADDR` and
`FINAPP_DB_PATH`. Use `make build` to create `build/finapp`.

The `/healthz` endpoint checks both the HTTP process and SQLite connection.
The app embeds its HTML, CSS, JavaScript, and Chart.js assets. The current
implementation includes bank import preview/confirmation, import-job history,
transactions, dashboard, and starter category/rule setup. Broader
categorization, transaction editing, transfer management, and in-app backup
and restore are not yet implemented.

## Configure

- Copy or create local configuration using documented, non-secret defaults.
- `FINAPP_LISTEN_ADDR`: host and port to bind; defaults to
  `127.0.0.1:8080` for local development.
- `FINAPP_DB_PATH`: SQLite file path; defaults to `data/finapp.db` for local
  development.
- Deployment values are `192.168.68.71:8080` and
  `/var/lib/finapp/finapp.db`, as shown in `deploy/finapp.env.example`.
- Deployment configuration: `/etc/finapp/finapp.env`, based on
  `deploy/finapp.env.example`.
- Keep credentials and real financial data out of source control.
- Local database and build outputs are ignored by Git.

## Stop and start

- Stop without disabling startup: `systemctl stop finapp.service`
- Start: `systemctl start finapp.service`
- Restart: `systemctl restart finapp.service`
- Stop and disable startup at boot: `systemctl disable --now finapp.service`
- Enable startup and start now: `systemctl enable --now finapp.service`

Run the selected command in the guest as root. To inspect status and recent
logs:

```sh
systemctl status --no-pager finapp.service
journalctl --unit=finapp.service --lines=100 --no-pager
```

## Back up the database

The current release does not provide an in-app backup or restore interface.
Use this offline file-level procedure as an administrator. The SQLite database
contains parsed financial data and categorization setup; treat every backup as
sensitive. Original uploaded CSV files are not stored in the database.

Create a root-only backup directory once:

```sh
install -d --owner=root --group=root --mode=0700 /var/backups/finapp
```

Before an application update or migration, stop the service and copy the
database while SQLite is closed. Run the commands one at a time; if a backup
command fails, restart the service after investigating:

```sh
systemctl stop finapp.service
BACKUP="/var/backups/finapp/finapp-$(date -u +%Y%m%dT%H%M%SZ).db"
install --owner=root --group=root --mode=0600 \
  /var/lib/finapp/finapp.db "${BACKUP}"
sqlite3 -readonly "${BACKUP}" 'PRAGMA integrity_check;'
systemctl start finapp.service
```

The integrity check must print `ok`. If it does not, do not use that backup for
an update or restore; the original database has not been changed by this
backup procedure. Verify the service is active and `/healthz` responds after
the backup. Copy backups to a separate trusted storage location for protection
from guest or host failure. Keep the copy private; the application does not
encrypt backup files.

## Update an installed release

Use the first-install download and extraction procedure above with the new
published version. Keep the extracted archive and files until the service is
verified. Back up the database first, then stop the service. Do not overwrite
the environment file: it contains the host-specific listener and database path.

Install the new executable through a temporary filename and rename it into
place on the same filesystem. Update the service unit if the release includes
a changed one, then record the version. Set `VERSION` and `STAGE` to the new
release's values as in the first-install procedure:

```sh
set -eu
VERSION=vN
STAGE="/tmp/finapp-${VERSION}"
systemctl stop finapp.service
install --owner=root --group=root --mode=0755 \
  "${STAGE}/finapp" /usr/local/bin/finapp.new
mv -- /usr/local/bin/finapp.new /usr/local/bin/finapp
install --owner=root --group=root --mode=0644 \
  "${STAGE}/finapp.service" /etc/systemd/system/finapp.service
systemctl daemon-reload
systemctl start finapp.service
systemctl status --no-pager finapp.service
curl --fail --show-error http://192.168.68.71:8080/healthz
printf '%s\n' "${VERSION}" > /usr/local/share/finapp/version
chmod 0644 /usr/local/share/finapp/version
```

The app applies embedded database migrations on startup. Confirm health returns
`ok`, check the logs for migration or startup errors, and open the app from a
home-network laptop before removing the staged release files.

## Recovery

If the service fails to start after an update, do not repeatedly restart it.
Inspect `systemctl status` and `journalctl` first. Confirm the environment
file, listener address, permissions, and installed files. If the new binary
was not installed completely, reinstall the same downloaded release package
using the installation steps above. The accepted release policy does not
include downgrading to a previous release.

If the database needs to be restored, set `RESTORE` to the exact backup file.
The commands validate it before replacing the current database, create and
check a separate pre-restore copy, and leave the service stopped if a command
fails:

```sh
set -eu
RESTORE="/var/backups/finapp/finapp-YYYYMMDDTHHMMSSZ.db"
test -f "${RESTORE}"
INTEGRITY="$(sqlite3 -readonly "${RESTORE}" 'PRAGMA integrity_check;')"
printf '%s\n' "${INTEGRITY}"
test "${INTEGRITY}" = "ok"
systemctl stop finapp.service
PRE_RESTORE="/var/backups/finapp/pre-restore-$(date -u +%Y%m%dT%H%M%SZ).db"
install --owner=root --group=root --mode=0600 \
  /var/lib/finapp/finapp.db "${PRE_RESTORE}"
INTEGRITY="$(sqlite3 -readonly "${PRE_RESTORE}" 'PRAGMA integrity_check;')"
printf '%s\n' "${INTEGRITY}"
test "${INTEGRITY}" = "ok"
install --owner=finapp --group=finapp --mode=0600 \
  "${RESTORE}" /var/lib/finapp/finapp.db.restore
mv -- /var/lib/finapp/finapp.db.restore /var/lib/finapp/finapp.db
systemctl start finapp.service
systemctl status --no-pager finapp.service
curl --fail --show-error http://192.168.68.71:8080/healthz
```

On startup, the installed application applies any newer embedded migrations
to the restored database. Verify health, inspect the logs, and check the app
from a home-network browser. Keep the pre-restore database backup until the
restored data has been checked. If restore or migration fails, stop the service
and preserve both database files for diagnosis; do not delete or overwrite
them.

## Local access and data safety

- Maximum bank CSV upload: 1 MB (1,000,000 bytes) per file; multiple files in
  one import job may exceed 1 MB total.
- Proxmox host address: `192.168.68.67`
- Home LAN CIDR: `192.168.68.0/22`
- App guest: static address `192.168.68.71` (verified available). Default
  gateway: `192.168.68.1`.
- App port: TCP `8080`
- Access is for PCs and devices on the private home network through a supported
  browser; no client installation or sign-in is required. Remote access is
  out of scope. Restrict access to `192.168.68.0/22`. The current setup has no
  port forwarding or tunnel; do not add one for the application. The existing
  network provides the home-only boundary, so no additional firewall rule is
  planned. Verify that the network boundary remains in place and no external
  route exposes the service.
- Do not expose the app or its data to the public internet without a separate
  security review and an agreed deployment design.
- Local development database: `data/finapp.db`; deployed database:
  `/var/lib/finapp/finapp.db`. The app creates database directories with
  owner-only permissions where possible.
- Uploaded CSV contents are parsed in memory and are not written to disk.
  Preview rows remain in memory for up to 20 minutes or until confirmation.
- Each file has a 1,000,000-byte cap; the limit is per file, not per job.
- SQLite is CGo-free; application-created database directories/files use
  restrictive `0700`/`0600` permissions where the host permits them.
- Chart.js 4.5.0 and its license files are embedded locally; the app does not
  depend on a public chart CDN or Node.js runtime.
- In-app backup/restore is not implemented. Use the documented offline
  database backup procedure; store copies with restrictive permissions and
  keep at least one copy outside the Proxmox guest.

## Sanitizing bank CSV samples on a Chromebook

Use the browser-only [CSV sanitizer](../../tools/sanitize-csv.html) without
uploading original bank exports to the remote workspace:

1. Download the HTML file itself to the Chromebook and open it locally in
   Chrome. Do not select or copy the original CSV into a Codespace or other
   remote workspace.
2. Select the local CSV. Choose its delimiter (the provided headers use
   semicolons), encoding, and whether the file contains a header row. The
   safer default is **No header**: this treats the first record as data and
   redacts it. Select **File has a header row** only when the CSV really has
   column names; those names are preserved unchanged.
3. Analyze the file. The tool shows column names and row counts, not
   transaction values. All cell values are redacted by default; only keep a
   column unchanged when its values are safe to share.
4. Create the separate sanitized download and inspect it locally before
   sharing. Column names are preserved, so review those too. Dates and amounts
   may reveal private information; keep them redacted unless you have a
   specific need and have reviewed the risk.

The page uses no external scripts or network requests. It keeps file contents
in the browser tab and creates the sanitized output as a local download.

## Troubleshooting

Document common setup, startup, migration, and import errors with safe
resolution steps. Do not include secrets or real account data in logs or
examples.
