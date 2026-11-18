# Technical design

**Status:** Phase 4 complete — architecture, stack, and hosting decisions
agreed; implementation details deferred
**Last updated:** 2026-11-18

This document defines how to implement the agreed [functional
design](../product/functional-design.md) and [experience
design](../product/experience-design.md). Choices labeled **Proposed** are
recommendations, not accepted decisions. Record accepted choices in the
[decision log](../decisions/decision-log.md).

## Constraints and quality attributes

- **Product scope:** One primary user in the first version; later household
  access is out of scope.
- **Hosting:** Proxmox VE is available, with an existing Home Assistant OS
  installation. Run the application in a separate Debian stable LXC guest
  beside HAOS. Serve it as a web application to PCs and devices on the private
  home network; remote access is out of scope. Clients use a supported browser
  without installing an app or signing in.
- **Network details:** The Proxmox host is `192.168.68.67` on
  `192.168.68.0/22`. The app guest's static address is
  `192.168.68.71`; the user verified it is available and safe to use. Use
  `192.168.68.1` as the default gateway. Serve HTTP on TCP port `8080` and
  allow access only from `192.168.68.0/22`; do not configure router port
  forwarding or a tunnel.
- **Network configuration:** The current setup has no port forwarding or
  tunnel configured. The application deployment should preserve this
  home-network-only access model.
- **Deployment preference:** Lightweight, largely self-contained, easy to
  maintain, and distributable from GitHub as one application package. GitHub
  builds releases; the user pulls an update when ready. Avoid adding separate
  services unless they provide clear value.
- **Data:** Sensitive financial records are retained until explicitly deleted.
  Original bank-export files are not included in backups and should not be
  retained after parsing.
- **Imports:** Support multiple files per job and reject the entire job if any
  row is invalid. CSV is the agreed bank-export format; user-provided schemas
  for Triodos, ING, and VDK are documented below. Possible duplicates match
  only when the account or accounts involved, amount, description, and
  transaction timestamp all match. The user chooses import history by
  selecting files; the app imposes no history-range limit. Limit each selected
  file to 1 MB (1,000,000 bytes); the cap is per file, not per job.
- **Triodos CSV variants:** Support comma and semicolon delimiters and
  headerless older exports. The user-described quoted ten-field variant is:
  booking date (`dd-mm-yyyy`), own account number, signed amount with comma
  decimal separator, counterparty account number, counterparty BIC/SWIFT,
  counterparty name, counterparty address, transaction code, description, and
  resulting account balance. The user's file uses a comma delimiter. For this
  variant, parse one date and a signed amount; do not assume a separate value
  date or debit/credit columns. Other variants may differ; support the
  documented delimiter/header options.
- **ING CSV variant:** The user-described file is semicolon-delimited with a
  header row and fields for own account number/name, counterparty account,
  transaction number, booking/value dates, amount, currency, description,
  transaction details, and message. Dates use `dd/mm/yyyy`; amounts use a
  comma decimal separator and may have a leading minus sign. Keep booking and
  value dates distinct, and use the import preview for any ambiguous
  transaction direction.
- **VDK CSV variant:** The user-described file is semicolon-delimited with
  double-quoted fields. Parse its six key/value account-summary records,
  followed by a blank line and an 18-field transaction header. The summary
  contains account identity/type, balance, balance timestamp, and an active
  filter indicator; the described balance uses a comma decimal separator and
  the balance timestamp uses `dd/mm/yyyy hh:mm:ss`. Transaction execution and
  value dates use `dd/mm/yyyy`; amounts may have a leading minus sign and use
  a comma decimal separator. The transaction fields include statement/reference
  identifiers, transaction type, counterparty details, message, amount,
  post-movement balance, exchange rate, and fees. Ignore the active-filter
  indicator; import the transactions present in the selected file without
  special handling.
- **Currency:** Account balances are EUR. For non-EUR transactions, use a
  bank-provided EUR equivalent when available; otherwise ask the user to
  specify one rate per source currency and transaction date for the current
  import preview. Apply it to matching transactions in that preview. The user
  is the rate source; no external historical-rate provider is planned.
- **Quality priorities:** Data correctness, predictable recovery, easy local
  operation, and maintainability are more important than serving many
  concurrent users.
- **Performance:** No fixed speed target is required for the first version.
- **Data volume:** Depends on the files selected by the user. Each file is
  capped at 1 MB; total job size may exceed that when multiple files are
  selected. Expected transaction counts remain to be confirmed during
  implementation.

## Architecture

### Agreed application shape

Run the app in its own Debian stable LXC guest on Proxmox, separate from the
Home Assistant OS installation. Do not make the app a Home Assistant add-on:
its data, lifecycle, and recovery should remain independent of HAOS.

Use one compiled Go executable with the web assets embedded. It serves the
interface and handles import parsing, validation, categorization rules,
calculations, persistence, and backup/restore. PCs and other devices connect
over the private home network using a standard web browser; no client app is
installed. Store application data in a local SQLite file outside the
executable's release directory. Run the executable as a systemd service
inside the guest.

This keeps deployment to one application package and one guest. It needs no
separate database service, web-server package, Python/Node.js runtime, Docker
engine, or Home Assistant integration. SQLite keeps user data independent of
application releases.

### Stack alternatives

| Option | What it means for this app | Benefits | Trade-offs |
| --- | --- | --- | --- |
| **Go + SQLite** (selected) | One compiled Go executable serves the web app and embeds its static assets; SQLite is a local data file. | Small deployment footprint; no app runtime or separate database service; a versioned executable can be packaged by GitHub Actions and pulled into the guest. | Bank-format parsing libraries and migration behavior must be validated; the UI still needs browser-side JavaScript for charts. |
| **Python + Django + SQLite** | A Python web application renders pages and handles business logic; a Python runtime and dependencies must be installed or bundled. | Mature web framework, built-in forms/security/migrations, and a broad data/file-parsing ecosystem. | A less self-contained release unless packaged in an additional container image; that adds a container runtime or packaging layer inside the guest. |
| **TypeScript + Next.js + SQLite** | One Node.js application serves the web interface and server-side behavior; TypeScript can be shared across browser and server code. | Strong fit for a highly interactive dashboard; one primary language across most application code. | Requires a Node.js runtime or container image and more frontend/build tooling than a single compiled executable. |

The selected Go package runs as a service in a separate Debian stable Proxmox
LXC guest. No separate database or application container runtime is planned.

### GitHub packaging and deployment

- GitHub Actions builds a versioned Linux AMD64 (`x86_64`) executable package
  for the Proxmox guest.
- The user pulls the selected release into the guest and restarts the service;
  persistent database and backup paths stay outside the release package.
- The guest makes outbound requests to GitHub. Do not open inbound access from
  GitHub to the home network and do not install a self-hosted GitHub Actions
  runner in the guest.
- Every push is not automatically deployed. This keeps updates controlled and
  avoids an extra updater service or runner.

### Component boundaries

1. **Web interface:** Pages and forms for importing, transactions,
   categorization, accounts, dashboard, and settings.
2. **Application services:** Import-job validation and commit, duplicate
   decisions, categorization/rule evaluation, balance calculations, transfer
   confirmation, and backup/restore.
3. **Bank-format adapters:** Parse one supported format at a time into a
   shared internal transaction representation. Keep format-specific parsing
   separate from business rules.
4. **Persistence:** Relational records for accounts, transactions, import
   jobs, categories, rules, confirmed transfers, and app settings.
5. **Local files:** Temporary import staging and user-created backup files.
   Do not persist original bank exports as application data.

## Technology choices

| Concern | Recommendation | Status |
| --- | --- | --- |
| Language and framework | Go; one compiled web application | Accepted |
| User interface | Embedded web assets with lightweight browser JavaScript | Accepted |
| Dashboard charts | Chart.js, bundled with the embedded web assets; no CDN dependency | Accepted |
| Database | SQLite in the guest's persistent data directory | Accepted |
| Database migrations | Goose with embedded SQL migration files | Accepted |
| Build and package tooling | Go modules; GitHub Actions builds versioned Linux release artifacts | Accepted |
| Test tooling | Go test for application and parser tests; Playwright using its Chrome channel for browser tests (Node.js is test-only) | Accepted |
| Hosting | Dedicated Debian stable Proxmox LXC guest, separate from HAOS | Accepted |
| Updates | GitHub Release package, pulled by the user when ready | Accepted |
| Import formats | Add and test a parser adapter for each documented bank CSV layout | CSV agreed; schemas documented, parser implementation pending |

## Data and imports

### Persistence model

Use normalized relational entities for:

- **Account:** account name/label, bank, currency, and balance-anchor details.
- **Transaction:** booking date, optional value date, source amount/currency,
  EUR amount where available, user-supplied rate details when needed,
  description/payee, direction, category, transfer status, and import-job link.
- **Import job:** creation time, source filenames as metadata only, row
  counts, status, and result summary.
- **Category and rule:** editable categorization setup and rule conditions.
- **Transfer pair:** the two linked transactions and user confirmation state.
- **Settings:** user-controlled app preferences required by the agreed scope.

Exact fields and constraints will be finalized alongside the first parser and
data model. Do not retain source file contents in transaction records.

### Import processing

1. Receive the selected files and write only to a restricted temporary
   location if streaming parsing requires disk.
2. Parse into a staging representation and validate every row.
3. Show file- and row-specific errors. If any row is invalid, reject the whole
   job and persist none of its transactions.
4. Present account mapping, duplicate candidates, manual-edit conflicts, and
   the combined preview for user decisions.
5. On confirmation, apply the selected decisions in one database transaction
   and record the import-job association for every imported row.
6. Remove staged file contents after completion, cancellation, or failure.

Apply the agreed duplicate criteria to identify possible matches: the account
or accounts involved, amount, description, and transaction timestamp must all
match. Keep detection separate from the user's choice to skip, keep both, or
update an existing transaction.

Starter-set files are UTF-8 JSON with a versioned schema. Validate the full
file before changing persistent data; preview categories to add or reuse and
existing transactions matched by each selected rule. Require confirmation
before adding categories or enabling selected rules. The first-version
format and built-in content are defined in the
[starter-set specification](../product/starter-sets.md).

### Backup and restore

- Backups are manually initiated, include parsed financial and
  categorization data, and omit original bank-export files.
- Warn that backup files contain sensitive financial information.
- Restore must validate the backup before replacing data and must require
  confirmation.
- Use a consistent database snapshot/backup mechanism; do not copy a live
  SQLite database file in a way that could produce an inconsistent backup.
- Selected-transaction deletion must preserve import-job provenance and
  require confirmation. Do not implement whole-import undo.

## Security and privacy

Bank and transaction data is sensitive. The first version has no sign-in by
product decision, so anyone able to reach the application on the trusted home
network may be able to access the data. Any supported client browser on that
network uses the same server-hosted web app; the persistent application data
is held by the Proxmox guest rather than a separate client installation.

### Proposed controls

- Bind only to the intended private-network interface; do not expose the app
  through router port forwarding, a public endpoint, remote tunnel, or other
  external route. The current setup has no port forwarding or tunnel; do not
  add one for the application.
- Rely on the existing network boundary to provide home-only access, as
  confirmed by the user; no additional host or guest firewall rule is planned.
  Verify during deployment that the boundary remains in place and no external
  route exposes the service.
- A private IP address alone is not an authentication mechanism.
- Use framework-standard CSRF protection and validate uploaded content by
  format, size, and parser behavior; do not trust filenames or client-provided
  content types.
- Keep original imports in temporary storage only and exclude financial data
  and transaction descriptions from logs and diagnostics.
- Keep database, temporary uploads, and backups outside version control.
- Application-managed encryption is not required for the database or backup
  files. Backups may be ordinary files; warn that they contain sensitive
  financial data. Host and network access controls remain relevant.
- Run the sample anonymizer locally with no network transfer; make it write a
  separate sanitized copy and flag data it cannot safely replace.

## Verification

- **Unit tests:** Format adapters, validation, rule matching, currency
  conversion, balance calculations, and transfer suggestions.
- **Integration tests:** All-or-nothing imports, duplicate choices, selected
  transaction deletion and provenance, including blocked deletion of confirmed
  transfer-linked transactions, backup/restore, and database migrations.
- **Browser tests:** First-use, import preview/rejection/success, categorizing,
  dashboard filters, account balances, and backup/restore confirmations.
- **Fixtures:** Synthetic bank-shaped files for regular tests. Use sanitized
  real samples only to verify actual bank layouts; never commit original bank
  files or unreviewed sanitized samples.
- **Quality checks:** Add type/style/build commands after the stack is agreed.
  Verify all required workflows in current Chrome on a laptop. Testing other
  browsers and phone layouts is nice to have.

## Deferred until implementation details are available

- Exact database schema and migration files.
- Expected transaction counts within the 1 MB-per-file limit.
- Exact Playwright test commands and CI wiring.
- Verify the existing network boundary and absence of external routes during
  deployment.
