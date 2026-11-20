# Experience design

**Status:** Phase 3 screen and flow design agreed
**Last updated:** 2026-11-18

This document turns the agreed first-version behavior in the
[functional design](functional-design.md) into the agreed first screen and
flow design. Use Material Design 3 (Material You) guidelines for visual
components, typography, spacing, and responsive behavior.

## Experience principles

- Optimize the main workflows for current Chrome on a laptop. Other browsers
  and phone use are nice to have.
- Serve the app from Proxmox as a pure web application. Any PC or device on the
  private home network with a supported browser can use it; client devices
  require no app installation.
- Keep importing, categorizing, and reviewing financial summaries as distinct
  destinations.
- Make the current import job and account context visible wherever relevant.
- Show a preview before an irreversible or bulk action; clearly state its
  scope and provide a confirmation step.
- Surface ambiguous or conflicting data and let the user decide rather than
  silently guessing.
- Never ask the user to upload an unmodified bank export for design or testing.

## Navigation

Use the following six primary navigation destinations:

1. **Overview** — dashboard and time-range selection.
2. **Import** — start an import and review recent import jobs on the Import
   landing screen; import history is not a separate destination.
3. **Categorize** — review Uncategorized transactions and manage rules.
4. **Transactions** — search, filter, edit, and remove transactions.
5. **Accounts** — balances and balance corrections.
6. **Settings** — categories, starter sets, backup, and restore.

On narrow screens, collapse the navigation into a menu while keeping the
current destination and primary action visible.

## Screens

### Overview dashboard

- Put a shared time-range selector at the top: rolling 12 months by default,
  the agreed presets, and a custom range.
- Show headline totals for combined account balance, income, spending, and
  saving for the selected period.
- Show the effective date beside each account's latest balance and disclose
  when the combined balance uses dates that differ.
- Apply the selected category filter to transaction-based cash-flow charts.
  Keep the account-balance chart as actual account balances regardless of
  category selection, and disclose this exception beside the filters.
- Use these four visualizations:
  - Monthly income and spending columns with a saving line.
  - Account balance history, optionally with a combined balance.
  - Spending by category, including Uncategorized.
  - Income by category.
- **Empty state:** Explain that the dashboard fills after importing
  transactions; provide a clear **Import data** action.
- **Partial data:** Show available totals without a partial-month marker.

### Import files

- Start with a multi-file picker and a short explanation of supported account
  scope.
- Show selected filenames, file-level processing state, and an option to
  remove a file before analysis.
- During analysis, show progress and prevent duplicate submissions.
- If an account is recognized, show the proposed account for confirmation. If
  uncertain, require selection or creation of an account before continuing.
- Treat pending rows as excluded from financial history. Identify unsupported
  formats and validation errors by filename and row.
- **Invalid-job state:** If any selected file contains an invalid row, explain
  that the entire job will be rejected, show the file/row errors, and offer
  retry after the source file is corrected. Do not import any files in that
  job.

### Import review and confirmation

- Show one combined job summary: files and accounts, date range, row/valid/
  invalid counts, possible duplicate count, incoming and outgoing totals, and
  category totals assigned by active rules.
- Keep row-level validation issues attached to their source file and row.
- Present duplicate candidates with the existing and incoming values and
  actions to skip, keep both, or update the existing transaction. Allow a
  choice per match or for a selected group of similar matches.
- For a manually edited match, show changed fields and require a keep-edits or
  use-imported choice.
- If an imported transaction lacks a bank-provided EUR equivalent, request one
  exchange rate per source currency and transaction date. Apply it only to
  matching transactions in this import preview.
- Require the user to confirm the whole job before saving. Leaving or
  cancelling before confirmation saves nothing.
- Duplicate candidates require an exact match on the account or accounts
  involved, amount, description, and transaction timestamp.

### Import result and job history

- Show imported and skipped counts and any errors.
- Provide a **View transactions** action scoped to that job. Do not provide an
  undo-import action.
- In the import history list, show a recognizable job summary and allow the
  user to open its transactions.
- Require confirmation before removing selected transactions.

### Categorization inbox

- Make this a separate destination, initially focused on Uncategorized
  transactions.
- Default ordering groups or prioritizes repeated payees/senders by frequency.
- Show as much relevant context as possible for each repeated-payee group:
  payee/sender, occurrence count, date range, separate incoming/outgoing
  totals, and associated accounts. Let the user expand the group to inspect
  each transaction's booking date, amount, description, account, direction,
  category, and import job.
- Provide filters for payee and transaction description, amount sorting,
  selection, and a bulk category action.
- Show the selected transaction count and chosen category before applying a
  bulk change.
- Let the user save the current filter as a rule.
- **Empty state:** Confirm that no Uncategorized transactions need review and
  provide a route to transactions or rule management.

### Rule creation and rule management

- In rule creation, let the user define conditions on transaction fields,
  including payee and transaction description as separate matchable fields,
  and choose AND or OR for multiple conditions.
- Show matching existing transactions before save, including enough
  information to inspect why they matched.
- On save, apply the rule to reviewed matches and future matches. For each
  manually categorized match, ask whether to replace or keep the current
  category.
- In rule management, provide search and clear enabled/disabled states, with
  edit, enable/disable, and delete actions.
- When disabling or deleting a rule, ask whether existing transactions keep
  their current categories or return to Uncategorized.
- Do not silently choose between conflicting rules. Give the user a path to
  choose a category or edit the rules.

### Transactions

- Provide a searchable, sortable transaction list with filters for account,
  date range, category, direction, transfer status, and import job.
- Keep booking date as the primary displayed and sorted date; show value date
  separately when available.
- Provide a transaction detail/edit view for date, amount, description/payee,
  account, direction, and transfer status. Keep category assignment in the
  categorization workflow.
- Allow selected transactions to be deleted with a clear count and
  confirmation. A selected transaction linked to a confirmed transfer cannot
  be removed until the transfer link is resolved. For a manually edited,
  unlinked transaction, disclose that its edits will be deleted and require
  confirmation before removal.
- Show confirmed transfers distinctly from income and spending.
- Keep both transactions in their account histories and account balances, but
  exclude confirmed transfer pairs from income/spending, saving, and category
  totals. Do not require categorization for confirmed transfers.

### Accounts and balances

- List current/checking accounts with their latest known balance and effective
  date; show the combined balance and disclose different component dates.
- Provide an account detail view with balance history and a form to enter a
  known balance and effective date.

### Categories, setup, and data management

- During first use, offer built-in starter categories and rules and an option
  to import UTF-8 JSON starter-set files. Let the user select categories and
  rules independently, preview categories to add or reuse and each selected
  rule's existing transaction matches, and confirm before importing or
  enabling rules.
- Provide category list/create/edit/delete actions. Before deleting a category
  in use, require a destination for its transactions and disable affected
  rules, identifying those rules to the user.
- Put manual backup and restore under Settings. Explain that backups contain
  sensitive financial data and omit uploaded source files.
- Before restore, state that current app data will be replaced and require
  confirmation.
- Keep delete-all-data out of the first-version interface.

## Core flow maps

### Import

`Select files → detect/confirm or create accounts → validate files → review
combined preview and duplicate choices → confirm job → show result → view job
transactions`

If any file has invalid rows, show the row errors and stop before import; the
user corrects the source file and retries the job.

### Categorize and create a rule

`Open Categorize → review repeated Uncategorized payees → filter/select
transactions → bulk assign category or save filter as rule → inspect existing
matches → resolve manual-category prompts → save`

### Review finances and correct a balance

`Open Overview → choose time range → inspect totals/charts → open account
detail → enter known balance and effective date → review recalculated history`

## States to cover in detailed screen design

- First run, no accounts, no transactions, and no Uncategorized transactions.
- Loading and processing, including repeated-submit prevention.
- Unsupported file, unreadable file, invalid rows, and whole-job rejection.
- Account detection uncertain or account not yet created.
- Duplicate candidates and manually edited conflicts.
- Import success, partial skip counts, and selected-transaction removal
  confirmation.
- Deletion blocked for a confirmed transfer-linked transaction, and impact
  confirmation for a manually edited transaction.
- Multiple conflicting rules and manual-category replacement.
- Empty dashboard range and history with differing balance dates.
- Backup creation, sensitive-data warning, invalid backup, and restore
  confirmation.

## Decisions incorporated

- The category filter applies to transaction-based cash-flow charts; actual
  account-balance history remains unchanged because balances are not
  category-specific.
- The six proposed primary navigation destinations are accepted.
- Recent import jobs remain on the Import landing screen rather than having a
  separate destination.
- The four proposed dashboard visualizations are accepted as the first-version
  starting set.
- Repeated-payee groups show relevant summary context and expandable
  transaction-level details, as described in the Categorization inbox section.

## Deferred beyond Phase 3

- Detailed component-level styling and responsive implementation should
  follow Material Design 3; current Chrome on laptop is the required target.
  Other browsers and phone layouts are nice to have.
- Remaining implementation details such as exact data/config paths and backup
  file format.
