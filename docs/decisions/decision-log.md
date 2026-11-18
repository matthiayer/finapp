# Decision log

Record significant product and engineering decisions here so they remain
available outside chat. Add a new entry rather than silently changing a
previous decision. If a decision is superseded, link the replacement.

## Decisions

### DEC-001: Agree first-version functional baseline

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** Phase 2 defined the first-version user needs, functional
  requirements, and observable acceptance criteria.
- **Decision:** Use the requirements and acceptance criteria in the functional
  design as the agreed baseline. Keep bank-format validation dependent on
  sanitized samples; defer technical choices and explicitly deferred behavior
  to the relevant later design phases.
- **Alternatives considered:** Keep the requirements and acceptance criteria
  provisional and continue Phase 2.
- **Consequences:** Phase 2 is complete and screen/flow design can proceed.
  Deferred questions remain open for their designated phases.
- **Related documents:** [Functional design](../product/functional-design.md);
  [implementation plan](../implementation/implementation-plan.md).

### DEC-002: Agree first-version screen and dashboard proposal

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** Phase 3 reviewed the proposed navigation, dashboard, import
  history, and repeated-payee categorization details.
- **Decision:** Use the six proposed primary destinations. Keep recent import
  jobs on the Import landing screen. Use the four proposed dashboard charts,
  with category filtering applied to every chart. Show repeated-payee groups
  with relevant summary context and expandable transaction-level details.
- **Alternatives considered:** Limit category filtering to category charts,
  provide a separate import-history destination, or reduce the detail shown
  for repeated-payee groups.
- **Consequences:** Phase 3 is complete. Detailed visual styling and technical
  behavior remain for later phases.
- **Related documents:** [Experience design](../product/experience-design.md);
  [functional design](../product/functional-design.md).

### DEC-003: Package the app for Proxmox as a Go release

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The user has Proxmox VE with Home Assistant OS and wants a
  lightweight, mostly self-contained application package with few additional
  services.
- **Decision:** Run the app in a dedicated Debian stable LXC guest, separate
  from HAOS. Use one compiled Go executable with embedded web assets and a
  local SQLite database. GitHub Actions builds versioned release packages;
  the user pulls updates when ready rather than deploying every push. Serve
  the app only to devices on the private home network; remote access is out of
  scope. The current Proxmox setup has no port forwarding or tunnel, and the
  application deployment should preserve that.
- **Alternatives considered:** Install the application as a Home Assistant
  add-on, add a container runtime and deploy an OCI image, or use a framework
  requiring a separate application runtime.
- **Consequences:** No separate database service, Docker engine, HAOS
  integration, or self-hosted GitHub runner is needed. The guest pulls
  application releases outbound; database and backup data remain outside the
  release package. Parser libraries, Linux target architecture, migrations,
  and charting library remain to be selected.
- **Related documents:** [Technical design](../technical/technical-design.md);
  [local development and hosting](../operations/local-development.md).

### DEC-004: Identify possible duplicate transactions

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The import flow needs explicit criteria for identifying
  transactions that may already have been imported.
- **Decision:** Treat a transaction as a possible duplicate only when the
  account or accounts involved, amount, description, and transaction timestamp
  all match an existing transaction. Ask the user how to handle each candidate
  rather than silently skipping or replacing it.
- **Alternatives considered:** Defer duplicate detection or match using a
  subset of transaction details.
- **Consequences:** Implement the four-field match criteria in import
  duplicate detection; keep the user's skip, keep-both, or update choice
  separate from detection.
- **Related documents:** [Functional design](../product/functional-design.md);
  [experience design](../product/experience-design.md);
  [technical design](../technical/technical-design.md).

### DEC-005: Use the user as the exchange-rate source

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** Non-EUR transactions need EUR values for combined totals and
  trends, and the user wants to provide a rate when one is not otherwise
  available.
- **Decision:** Use a bank-provided EUR equivalent when present. Otherwise,
  ask the user to provide the exchange rate for the transaction date. The user
  is the rate source; no external historical-rate provider is planned.
- **Alternatives considered:** Select an external historical exchange-rate
  provider.
- **Consequences:** Do not integrate an external exchange-rate service for the
  first version; provide a way for the user to supply a transaction's rate
  when the bank export has no EUR equivalent.
- **Related documents:** [Functional design](../product/functional-design.md);
  [technical design](../technical/technical-design.md).

### DEC-006: Do not provide whole-import undo

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The import workflow previously included an option to undo an
  entire completed import job.
- **Decision:** Remove whole-import undo from the first-version scope. Keep
  import-job history and transaction links, and allow the user to remove
  selected transactions from a job with confirmation.
- **Alternatives considered:** Keep the whole-job undo action alongside
  selected-transaction removal.
- **Consequences:** Import results and history provide a way to view the job's
  transactions but no undo action. The effects of removing edited or linked
  transactions still need to be defined.
- **Related documents:** [Functional design](../product/functional-design.md);
  [experience design](../product/experience-design.md);
  [technical design](../technical/technical-design.md).

### DEC-007: Protect confirmed transfers when deleting transactions

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The user can remove selected transactions, but some may be
  manually edited or linked as confirmed internal transfers.
- **Decision:** Block removal of a transaction while it is linked to a
  confirmed transfer; the user must resolve that link first. Allow removal of
  a manually edited, unlinked transaction after clearly explaining that its
  edits will be deleted and asking for confirmation.
- **Alternatives considered:** Allow deletion of linked transactions directly,
  or block deletion of any manually edited transaction.
- **Consequences:** The transaction workflow must identify linked and edited
  records and provide the appropriate block or impact confirmation.
- **Related documents:** [Functional design](../product/functional-design.md);
  [experience design](../product/experience-design.md);
  [technical design](../technical/technical-design.md).

### DEC-008: Reuse entered exchange rates within an import preview

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The user supplies exchange rates when a bank export has no EUR
  equivalent.
- **Decision:** During import, ask once per source currency and transaction
  date, then apply that rate to matching transactions in the current import
  preview.
- **Alternatives considered:** Ask for a separate rate for every transaction
  or use one rate for an entire currency regardless of date.
- **Consequences:** The import preview must collect one rate per
  currency/date and apply it to matching transactions in that preview. No
  additional exchange-rate requirements need to be defined at this stage.
- **Related documents:** [Functional design](../product/functional-design.md);
  [experience design](../product/experience-design.md);
  [technical design](../technical/technical-design.md).

### DEC-009: Use Material Design 3 and prioritize laptop Chrome

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The interface needs a visual design baseline and an explicit
  first-version browser/device target.
- **Decision:** Follow Material Design 3 (Material You) guidelines. Optimize
  and verify required workflows in current Chrome on a laptop. Support for
  other browsers and phone layouts is nice to have, not a first-version
  acceptance requirement.
- **Alternatives considered:** Require broad browser and phone support from the
  first version.
- **Consequences:** Design and test the primary experience for laptop Chrome;
  keep other browser and phone support as a best-effort enhancement.
- **Related documents:** [Functional design](../product/functional-design.md);
  [experience design](../product/experience-design.md);
  [technical design](../technical/technical-design.md).

### DEC-010: Exclude confirmed internal transfers from income and spending

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** A movement of -5000 from account A and +5000 to account B is
  one internal transfer, not an expense and income.
- **Decision:** Keep both transactions in their account histories and use them
  for account balances. After the user confirms the pair as an internal
  transfer, exclude both sides from income, spending, saving, and category
  totals; no categorization is needed. Retain the existing user confirmation
  for suggested transfer pairs.
- **Alternatives considered:** Remove one or both transaction rows, or count
  the two sides as income and spending.
- **Consequences:** Transfer matching must preserve both ledger entries while
  excluding confirmed pairs from cash-flow and category reporting. This also
  clarifies the prior spreadsheet cleanup: do not delete the transfer entries.
- **Related documents:** [Functional design](../product/functional-design.md);
  [experience design](../product/experience-design.md);
  [technical design](../technical/technical-design.md).

### DEC-011: Do not handle transaction-history gaps

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The initial design included detecting missing transaction
  periods and prompting the user to resolve them.
- **Decision:** Do not detect or manage transaction-history gaps in the
  first-version app. Do not show gap alerts, incomplete-history markers,
  reminders, or gap-resolution actions.
- **Alternatives considered:** Detect gaps and let the user upload more files,
  defer reminders, or explicitly ignore each gap.
- **Consequences:** Calculate and show balances and totals from available
  imported data without a separate gap-detection or resolution workflow.
- **Related documents:** [Functional design](../product/functional-design.md);
  [experience design](../product/experience-design.md);
  [technical design](../technical/technical-design.md).

### DEC-012: Let the user choose the imported history range

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The first-version bank imports need a defined set of supported
  banks and a policy for how much transaction history to import.
- **Decision:** Support CSV imports from Triodos Belgium, ING Belgium, and VDK
  Belgium. The user chooses how much history to import by selecting files;
  the application does not impose a history-range limit.
- **Alternatives considered:** Impose a fixed history window such as five
  years.
- **Consequences:** Confirm each bank's CSV layout from sanitized samples;
  import eligible transactions contained in the files selected by the user.
- **Related documents:** [Functional design](../product/functional-design.md);
  [technical design](../technical/technical-design.md).

### DEC-013: Ignore VDK's active-filter indicator

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The VDK CSV account summary includes an indicator showing
  whether a filter was active when the export was created.
- **Decision:** Ignore the active-filter indicator. Import the transactions
  present in the selected file without special handling based on that value.
- **Alternatives considered:** Warn or block import when a filter was active.
- **Consequences:** No active-filter validation or warning is required in the
  VDK import flow.
- **Related documents:** [Functional design](../product/functional-design.md);
  [technical design](../technical/technical-design.md).

### DEC-014: Target Linux AMD64 for the Proxmox guest

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The Proxmox host reports an `x86_64` CPU architecture.
- **Decision:** Build and package the first-version Linux release for AMD64
  (`x86_64`).
- **Alternatives considered:** Target ARM64 or publish multiple architecture
  builds.
- **Consequences:** The initial release artifact and deployment guide target
  Linux AMD64.
- **Related documents:** [Technical design](../technical/technical-design.md);
  [local development and hosting](../operations/local-development.md).

### DEC-015: Select migration, charting, and browser-test tooling

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The Go application needs SQLite schema migrations, local
  dashboard charts, and automated browser verification.
- **Decision:** Use Goose with embedded SQL migration files; bundle Chart.js
  with the embedded web assets without a CDN dependency; use Go tests for
  application and parser behavior and Playwright with its Chrome channel for
  browser tests. Node.js is test-only and is not part of the deployed app
  runtime.
- **Alternatives considered:** Other Go migration tools, browser chart
  libraries, and browser automation frameworks.
- **Consequences:** Verify Goose with the selected SQLite driver and package
  Chart.js locally; configure Playwright/Chrome in the development and CI test
  environments.
- **Related documents:** [Technical design](../technical/technical-design.md);
  [implementation plan](../implementation/implementation-plan.md).

### DEC-016: Restrict the app to the home LAN on TCP port 8080

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The Proxmox host is at `192.168.68.67`, and the user confirmed
  the home network uses a `/22` prefix and approved port `8080`.
- **Decision:** Treat `192.168.68.0/22` as the allowed home LAN. Assign the
  app's LXC its own stable private address and serve the app on TCP port
  `8080`. Restrict access to that LAN; do not add router port forwarding or a
  tunnel.
- **Alternatives considered:** Use another port or expose the service beyond
  the private home network.
- **Consequences:** The specific LXC address and network-boundary approach
  are recorded in DEC-022.
- **Related documents:** [Technical design](../technical/technical-design.md);
  [local development and hosting](../operations/local-development.md).

### DEC-017: Use main-only development and sequential release labels

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The project needs a simple branching and release-numbering
  approach.
- **Decision:** Work on the `main` branch only, without feature branches.
  Number releases sequentially as `v1`, `v2`, `v3`, and onward.
- **Alternatives considered:** Feature branches and semantic version numbers
  with major, minor, and patch components.
- **Consequences:** Changes are made directly on `main`; release labels
  increment sequentially.
- **Related documents:** [Implementation plan](../implementation/implementation-plan.md);
  [technical design](../technical/technical-design.md).

### DEC-018: Do not provide release rollback

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** A release rollback procedure was listed as a deployment
  decision.
- **Decision:** Release rollback is not required.
- **Alternatives considered:** Preserve and reinstall the previous application
  release after an update failure.
- **Consequences:** The first-version release process will not include a
  rollback procedure. Database backup and restore remain separate functions.
- **Related documents:** [Implementation plan](../implementation/implementation-plan.md);
  [local development and hosting](../operations/local-development.md).

### DEC-019: Require no additional release-specific verification

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The delivery plan asked whether extra release-specific review
  or verification was required.
- **Decision:** No additional release-specific review or verification steps
  are required. Normal implementation tests and checks still apply.
- **Alternatives considered:** Add a separate release approval or smoke-test
  gate beyond the ordinary development checks.
- **Consequences:** The release process has no extra verification gate; normal
  tests and checks remain part of implementation.
- **Related documents:** [Implementation plan](../implementation/implementation-plan.md).

### DEC-020: Limit each import file to 1 MB

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The import file-size limit and performance target needed
  definition.
- **Decision:** Limit each selected bank CSV file to 1 MB (1,000,000 bytes).
  Apply the limit per file, not to the total size of a multi-file job. No
  fixed speed target is required.
- **Alternatives considered:** Apply a total job-size limit or define a
  quantitative import/response-time target.
- **Consequences:** Reject any individual file larger than 1,000,000 bytes
  before parsing and report the file-specific size limit. Validate
  performance qualitatively for correctness and predictable recovery.
- **Related documents:** [Functional design](../product/functional-design.md);
  [technical design](../technical/technical-design.md);
  [local development and hosting](../operations/local-development.md).

### DEC-021: Define an optional starter set and versioned JSON import

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The first-use experience needed defined starter categories,
  conservative categorization rules, and a portable import format.
- **Decision:** Provide a flat English starter category list and four
  optional description-keyword rules for salary, interest, refunds, and bank
  fees. Use a versioned UTF-8 JSON format for importing categories and
  direction/description rules. Let users choose categories and rules
  independently, preview existing matches, and confirm before importing or
  enabling selected items.
- **Alternatives considered:** Leave starter content and format undefined,
  or automatically apply a broader set of categorization rules.
- **Consequences:** Starter categorization remains opt-in and reviewable.
  Version 1 accepts only direction and description conditions in imported
  starter rules; malformed or unsupported files must not change app data.
- **Related documents:** [Starter-set specification](../product/starter-sets.md);
  [functional design](../product/functional-design.md);
  [experience design](../product/experience-design.md).

### DEC-022: Use a static app address and rely on the existing home network

- **Status:** Accepted; address availability verified
- **Date:** 2026-11-18
- **Context:** The app LXC needs a stable address, and the user confirmed that
  the existing network defines home-only access.
- **Decision:** Configure the app LXC with static address `192.168.68.71` on
  `192.168.68.0/22`. Rely on the existing network boundary for home-only
  access; do not add a separate host or guest firewall rule. Do not configure
  router port forwarding or a tunnel.
- **Alternatives considered:** Use a DHCP reservation or add a separate
  firewall rule to enforce the LAN-only boundary.
- **Consequences:** The user verified that `192.168.68.71` is available and
  good to use. Use the default gateway recorded in DEC-023 and verify during
  deployment that no external route exposes the service. Without application
  sign-in, devices allowed by the existing network boundary can access the
  app.
- **Related documents:** [Technical design](../technical/technical-design.md);
  [local development and hosting](../operations/local-development.md);
  [DEC-016](#dec-016-restrict-the-app-to-the-home-lan-on-tcp-port-8080).

### DEC-023: Set the app LXC's default gateway

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** The app LXC's static network configuration needs the home
  network's IPv4 default gateway.
- **Decision:** Use `192.168.68.1` as the app LXC's default gateway.
- **Alternatives considered:** Other gateway addresses on the home network.
- **Consequences:** Configure the LXC to route through `192.168.68.1`; the
  static address availability is confirmed in DEC-022.
- **Related documents:** [Technical design](../technical/technical-design.md);
  [local development and hosting](../operations/local-development.md);
  [DEC-022](#dec-022-use-a-static-app-address-and-rely-on-the-existing-home-network).

## Entry template

Copy this section for each decision:

### DEC-001: Short title

- **Status:** Proposed / Accepted / Superseded
- **Date:** YYYY-MM-DD
- **Context:** What question or constraint prompted this decision?
- **Decision:** What was chosen?
- **Alternatives considered:** What other reasonable options were considered?
- **Consequences:** What trade-offs, follow-up work, or risks result?
- **Related documents:** Links to relevant design sections
