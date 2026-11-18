# Local development and hosting

**Status:** Installation/build/activation guide outlined; exact commands pending
the app foundation
**Last updated:** 2026-11-18

This is the step-by-step guide for building, installing, activating, updating,
and safely operating the application. Fill in and verify exact commands as
the app foundation and release process are implemented.

## Requirements

- Operating system and version: Debian stable in a dedicated Proxmox LXC guest
- Release architecture: Linux AMD64 (`x86_64`)
- Runtime and version: Compiled Go application; Go toolchain needed to build,
  not to run a release binary
- Required local services: systemd; SQLite is embedded as a local database
  file
- Build and release instructions: To add
- Installation and activation instructions: To add

## Build, install, and activate

Document and verify these steps in order:

1. Prepare the Debian stable LXC guest in Proxmox, including required
   resources, network settings, and verification of the existing home-only
   access boundary.
2. Build and test the application locally, or obtain the versioned release
   package produced by GitHub Actions.
3. Install the selected release in the guest and create the application
   service account and persistent data directories.
4. Configure the application using documented, non-secret defaults.
5. Install and enable the systemd service, then start the application.
6. Verify service health and access the app from an allowed laptop on the
   private home network using current Chrome. No client-side app installation
   or sign-in should be required. Other browsers and phone layouts are
   nice-to-have.

Exact commands and paths remain pending implementation.

## Configure

- Copy or create local configuration using documented, non-secret defaults.
- Required environment variable names and descriptions: To document.
- Keep credentials and real financial data out of source control.
- Local configuration file and ignore rules: To document.

## Run and stop

Include both the local development workflow and the deployed service workflow.
Clearly distinguish building/testing a source checkout from installing a
released application package.

```text
Build and test locally: To document
Install or update a release package: To document
Apply database migrations safely: To document
Activate or restart the systemd service: To document
Open and verify the app: To document
Stop or disable the service: To document
```

## Update and recovery

- Document how to back up the database before an update or migration.
- Document how to verify the installed version and health after updating.
- Document how to recover from a failed update or restore application data.

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
- Local database and uploaded-file locations: To document.
- Backup, restore, and test-data reset steps: To document.

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
