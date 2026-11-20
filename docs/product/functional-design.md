# Functional design

**Status:** Phase 2 requirements and Phase 3 experience design agreed
**Last updated:** 2026-11-18

Describe what the app should help people do. Keep this document focused on
user needs and observable behavior; put technology choices in the
[technical design](../technical/technical-design.md).

## Problem and intended users

- **Problem to solve:** The user has accounts at multiple banks but lacks one
  place to understand balances, spending, and overall financial activity. They
  are concerned that unnoticed spending could make it difficult to save.
- **Current workflow:** The user maintains one Excel file containing a large
  list of bank history. They manually remove duplicates related to movements
  between accounts, add a category to each transaction, then build graphs and
  overviews using pivot tables.
- **Desired improvement:** Make that workflow easier, reduce repetitive manual
  work, and turn it into a sustainable web application.
- **Highest-priority pain point:** Manually assigning a category to every
  transaction. The first version should make categorization especially quick
  and easy to review.
- **Initial intended user:** The user themselves.
- **Possible future users:** Their household; multi-user behavior is not yet in
  the first-version scope.
- **Where and how users expect to use the app:** A pure web application hosted
  on Proxmox and accessed through a supported browser from any PC or device on
  the private home network, with the network boundary defined in the
  [technical design](../technical/technical-design.md). Client devices should
  not need an installed app or sign-in.

## Goals and non-goals

### Goals

- Bring information from multiple bank accounts into one app. Manual import is
  acceptable for the first version if needed.
- The first version covers current/checking accounts only.
- Provide three distinct areas: importing information, viewing dashboards, and
  categorizing transactions. Categorization is a separate module within the
  app.
- Prioritize reducing the time and effort needed to categorize transactions.
- Categorization is meaningfully easier when active rules handle repeated
  transactions automatically and the remaining new or unmatched transactions
  are quick to review and categorize.
- Show an overview of finances, including balances and history, and make
  multi-year financial trends easier to understand.
- Help the user understand spending and income so activity that may affect
  saving is easier to notice.

### Non-goals

- Automatic outlier detection or alerts are not part of the first version.
- Recurring-payment or transaction-pattern analysis is not part of the first
  version; the focus is on financial trends over time.
- Category-specific trend views are not part of the first version; category
  breakdowns remain available in the overview.
- Importing the user's existing Excel workbook is not in the first-version
  scope; initial transaction history will come from bank-export files.
- Savings and credit-card accounts are not in the first-version scope.
- Direct bank connections and household/multiple-user access are not in the
  first-version scope.

## User journeys

For each journey, describe the starting point, the user's actions, the outcome,
and important error or recovery paths.

### First use and setup

- Initial user: the account owner.
- During first-run configuration, offer starter categories and starter rules as
  optional imports that the user can choose independently. Enable imported
  starter rules immediately after preview and confirmation. Provide built-in
  starter sets and allow importing UTF-8 JSON starter-set files. The contents
  and format are defined in the [starter-set specification](starter-sets.md).

### Importing or updating account data

- Support bank-export CSV files from Triodos Belgium, ING Belgium, and VDK
  Belgium. Each bank defines its own CSV layout; confirm the field mapping
  from bank documentation or user-provided field descriptions, then verify the
  parser with synthetic fixtures and sanitized samples when needed.
- For Triodos CSVs, account for comma or semicolon delimiters and the
  possibility that older exports have no column-header row. Map transaction
  and value dates, own and counterparty account details, amount and direction,
  transaction type/description, and post-transaction balance where present.
  Amount direction may be encoded by a signed amount or separate debit and
  credit columns; confirm the actual variant from sanitized samples.
- A user-described Triodos variant has quoted fields in this order: booking
  date (`dd-mm-yyyy`), own account number, signed amount with comma decimal
  separator, counterparty account number, counterparty BIC/SWIFT, counterparty
  name, counterparty address, transaction code (for example `SCT` or `IN`),
  description, and resulting account balance. The described variant has one
  date field and a signed amount; do not assume a separate value date or
  debit/credit columns in this variant. The user's Triodos file uses a comma
  delimiter.
- The user-described ING CSV has a semicolon delimiter and a header row with
  these fields in order: `Rekeningnummer`, `Naam van de rekening`, `Rekening
  tegenpartij`, `Omzetnummer`, `Boekingsdatum`, `Valutadatum`, `Bedrag`,
  `Munteenheid`, `Omschrijving`, `Detail van de omzet`, and `Bericht`. Booking
  and value dates use `dd/mm/yyyy`; amounts use a comma decimal separator and
  may have a leading minus sign.
- The user-described VDK CSV is semicolon-delimited with quoted fields. It
  starts with six key/value summary records for account number, account name,
  account type, balance, balance timestamp, and whether a filter is active;
  after a blank line comes the transaction header. Its fields, in order, are
  execution date, value date, statement year, statement number, VDK reference,
  transaction type, counterparty account, counterparty BIC/SWIFT, counterparty
  name, address, postal code, city, country, message, amount, balance after
  movement, exchange rate, and fees. The account summary may provide the
  account balance and its effective timestamp; the described balance uses a
  comma decimal separator and its timestamp format is `dd/mm/yyyy hh:mm:ss`.
  Transaction dates use `dd/mm/yyyy`; amounts may have a leading minus sign
  and use a comma decimal separator. The example has the active-filter
  indicator set to `Neen`; the application ignores this indicator and imports
  the transactions present in the selected file without special handling.
- Import posted/settled exports only. Pending-row detection/filtering is
  deferred because the currently supported Triodos, ING, and VDK layouts do
  not include an explicit pending-status field; do not infer pending status
  from free text.
- Determine whether a transaction is incoming or outgoing from bank data when
  clear; ask the user to resolve ambiguous direction. Direction is separate
  from category.
- Let the user resolve an ambiguous transaction direction in the import
  preview before confirming the job.
- Use the booking date as the primary date for transaction lists and trends.
  Retain the value date separately when it is present in the bank export.
- The user expects bank-export files to contain no reversal transactions; a
  reversal-handling workflow is not part of the first-version scope.
- Let the user select multiple bank-export files from different accounts in
  one import job.
- Limit each uploaded file to 1 MB (1,000,000 bytes). The limit applies per
  file, so a multi-file job may exceed 1 MB in total when each file is within
  the limit.
- Let the user choose how much history to import by selecting the bank-export
  file or files. The application does not impose a history-range limit.
- Detect the bank and account from an import file when possible; ask the user
  to confirm or select an account if the match is uncertain.
- If an import belongs to an account not yet in the app, let the user create
  that account during the import flow.
- Before saving, the app should show a preview of the selected file and let the
  user confirm the import.
- Summarize the categories assigned by active rules in the import preview;
  transaction-level categorization review happens in the categorization module.
- Include the source bank/account, transaction date range, total row count with
  valid and invalid counts, possible duplicate count, and total incoming and
  outgoing amounts in the preview.
- For a multi-file job, show one combined summary rather than a separate
  aggregate summary per file/account; identify the included files/accounts and
  keep validation errors tied to the relevant file and row.
- If an import may contain transactions already imported, the app should show
  the possible duplicates and ask the user how to handle them rather than
  silently adding or discarding them.
- Treat an imported transaction as a possible duplicate only when the account
  or accounts involved, amount, description, and transaction timestamp all
  match an existing transaction.
- For a possible duplicate, offer to keep the existing transaction and skip
  the imported row, import it anyway and keep both, or replace/update the
  existing transaction with the imported data. Let the user decide per match
  or apply a choice to a group of similar matches.
- If an imported transaction matches one the user has manually edited, show
  the differing values and ask whether to keep the edits or use the imported
  values.
- If any file in a multi-file job contains invalid rows, report the invalid
  rows and reject the entire job without importing any of its files. The user
  can correct the source file and retry the job.
- After an import, let the user review the batch and remove selected
  transactions. Do not provide an action to undo an entire import batch.
- Keep each imported transaction linked to its import job so the user can
  filter transactions by the job that imported them.
- When a job completes, show a result summary of imported and skipped
  transactions and any errors, with an action to view the job's transactions.
- Deferred until sanitized samples are available: verify the bank-specific CSV
  layouts and actual history availability.

#### Public bank-export research

Research checked on 2026-11-16. These public help pages do not confirm what is
available for the user's specific accounts; verify export options in the bank's
own service before finalizing format support.

Bank-specific parser behavior must be validated against synthetic bank-shaped
fixtures and, when needed to confirm a real export layout, a sanitized sample.
Do not upload or commit an unmodified bank export. The browser-only
[CSV sanitizer](../../tools/sanitize-csv.html) can create a separate copy on
the user's Chromebook without network access. It redacts every data value by
default; review column names and any values explicitly kept unchanged before
sharing. Keep original exports and sanitized samples out of version control.

| Bank | Publicly documented information | Publicly documented history/export details |
| --- | --- | --- |
| ING Belgium | Public format details were not confirmed. The user described a CSV with a semicolon delimiter and header row: `Rekeningnummer`, `Naam van de rekening`, `Rekening tegenpartij`, `Omzetnummer`, `Boekingsdatum`, `Valutadatum`, `Bedrag`, `Munteenheid`, `Omschrijving`, `Detail van de omzet`, and `Bericht`. Dates use `dd/mm/yyyy`; amounts use a comma decimal separator and may have a leading minus sign. | Not a product requirement |
| Triodos Belgium | [Internet Banking export FAQ](https://www.triodos.be/nl/meest-gestelde-vragen/kan-ik-gegevens-vanuit-internet-banking-naar-mijn-boekhoudpakket-exporteren?id=31b679c364da) lists Excel, CSV, and CODA for exporting account data to accounting software. A [separate FAQ](https://www.triodos.be/nl/meest-gestelde-vragen/hoe-kan-ik-mijn-rekeninguittreksels-raadplegen?id=e6f9b40c28bd) documents PDF statements. User-provided format details: CSVs may use comma or semicolon delimiters; older exports may omit column headers. The user's file uses a comma delimiter. The user described a quoted, ten-field variant with booking date (`dd-mm-yyyy`), own account number, signed amount with comma decimal separator, counterparty account number, counterparty BIC/SWIFT, counterparty name, counterparty address, transaction code, description, and resulting balance. This variant has one date field and no separate debit/credit columns. No real transaction values are needed to define this mapping. | Not stated publicly |
| VDK Belgium | The [account movements FAQ](https://www.vdk.be/nl/veelgestelde-vragen/ik-wil-een-overzicht-van-de-bewegingen-op-mijn-rekeningen-waar-vind-ik-dat) says processed movements can be viewed up to ten years back. The public pages found do not confirm a structured transaction-export format. A separate [statement FAQ](https://www.vdk.be/nl/veelgestelde-vragen/hoelang-blijven-mijn-elektronische-rekeninguittreksels-beschikbaar) says electronic statements remain in history for two years. The user described a semicolon-delimited quoted CSV with a six-record account summary, a blank line, and an 18-field transaction header. The summary includes account identity/type, balance and timestamp, and an active-filter indicator, which the app ignores; it imports the transactions present in the selected file without special handling based on that indicator. Transaction dates use `dd/mm/yyyy`; amounts may have a leading minus sign and use a comma decimal separator. The described account-balance timestamp uses `dd/mm/yyyy hh:mm:ss`, and balance uses a comma decimal separator. The example has the filter indicator set to `Neen`. | Exportable history range not confirmed |

### Creating and viewing dashboards

#### Dashboard graph proposal (accepted as the first-version starting set)

The user has said the dashboard is not the priority yet, but accepted this
graph set as the first-version starting point. The detailed visual treatment
can be iterated later.

1. **Monthly cash flow:** grouped columns for incoming money and spending, with
   a line for saving (income minus spending). Exclude confirmed internal
   transfers from these totals. Default to the rolling 12 months; allow the
   agreed time-range presets and custom dates.
2. **Account balance history:** a line per current account, with an optional
   combined balance. Show the effective date for latest balances and disclose
   when a combined total combines balances from different dates.
3. **Spending by category:** horizontal bars for the selected period, including
   Uncategorized, so the largest spending groups are easy to identify.
4. **Income by category:** a category breakdown for the selected period, so
   groups such as Salary can be understood without requiring automatic salary
   detection.

Selecting a category filters every dashboard chart. Category-specific monthly
trend charts are not included. Keep the dashboard predefined; this proposal
does not add custom widgets, saved views, or report export.

- The first version should show combined balances across accounts, income and
  spending over time, transaction history, and spending by group.
- Show each account's latest known balance with its date. The combined balance
  should use each account's latest known balance and make clear that those
  balances may be from different dates.
- If an import does not provide an account balance, ask the user to enter the
  current balance and record when it applies. Use that balance and the imported
  transactions to calculate historical balances for the covered period.
- Let the user update an account's known balance and effective date later;
  recalculate historical balances from available transactions.
- The default dashboard time window should be the rolling last 12 months, with
  the ability to change the selected time window.
- Show total incoming money and spending by group. Income may have groups such
  as Salary, so the user can see salary separately when they categorize it.
- The user defines saving as income minus spending.
- Exclude confirmed internal transfers from the income and spending totals used
  to calculate saving.
- Account balances supported in the first version must already be in EUR.
- Convert non-EUR transaction amounts to EUR for combined totals and trends.
- For non-EUR transactions, use the bank-provided EUR equivalent when
  available; otherwise ask the user to specify the exchange rate for the
  transaction date. During import, ask once per source currency and transaction
  date and apply that rate to matching transactions in the current import
  preview. The user is the source of the rate; no external historical rate
  provider is planned.
- Show multi-year trends in spending, saving, earning, and account balances as
  monthly totals.
- If history only covers part of a month, show the total for the available
  transactions without a special partial-month marker.
- Let the user choose a common preset range or a custom start and end date.
- Offer these preset ranges: 3 months, 6 months, 12 months, year to date,
  previous calendar year, 3 years, 5 years, and all available history. The
  default remains the rolling last 12 months.
- Use a predefined dashboard layout in the first version. Let the user filter
  by time range and category, but do not support adding or rearranging widgets
  or saving filter combinations. Dashboard charts and summaries are viewed in
  the app, not exported as reports.
- Internal transfers should be shown separately from income and spending.
- The app should suggest likely pairs of outgoing and incoming transactions
  as transfers when they are in opposite directions, have the same amount, are
  within three days of each other, and belong to accounts the user imported.
  All accounts imported in the first version belong to the user. Ask the user
  to confirm each suggested match.
- Keep both sides of a confirmed transfer in their account histories and use
  them when calculating the respective account balances. Do not treat either
  side as income or spending, include them in saving or category totals, or
  require them to be categorized.

### Categorizing transactions

- Categorization should be a separate module in the app, not just an action
  embedded in the dashboard.
- Categories are flexible and are not restricted to income or spending
  transactions.
- Let the user import a starter set of categories.
- The app should use rules to suggest or assign categories to income and
  spending transactions.
- Rules may match transaction direction or amount (for example, positive
  amounts), source, recipient/payee, or text in the description. For example,
  the user can create a Salary category and a rule matching positive amounts
  from a particular sender.
- For a rule with multiple conditions, let the user choose whether all
  conditions must match (AND) or any condition may match (OR).
- Transactions that no rule can categorize should be available in an
  Uncategorized review list. Uncategorized is a category of its own, so these
  transactions remain included in the applicable income or spending totals and
  appear separately in category breakdowns.
- Confirmed internal transfers do not require a category and should not appear
  in the Uncategorized review list.
- Make it easy to filter uncategorized transactions and save a useful filter as
  a reusable categorization rule. Prioritize a clear, efficient review
  experience so repeated or straightforward cases are easy to process; this is
  the highest-priority pain point in the current workflow.
- Make repeated payees or senders and frequently occurring transactions easy
  to spot, and allow sorting by transaction amount.
- Sort the Uncategorized review list by most frequent/repeated payees first by
  default; let the user change filters and sorting.
- Let the user select and categorize multiple matching transactions together.
- During rule creation, preview the existing transactions that match the rule.
  Saving the rule applies it to the previewed existing transactions and
  automatically categorizes future matching transactions.
- If an existing match has a category the user assigned manually, show it in
  the preview and ask whether to replace that category before applying the rule
  to that transaction.
- Provide a place to review and manage existing rules, not only create rules
  from transaction filters. The user should be able to view and search rules,
  edit them, enable or disable them, and delete them. When disabling or
  deleting a rule, ask whether to keep its existing categorizations or return
  affected transactions to Uncategorized.
- The user should remain in control: they can override the group for an
  individual transaction and create categories and rules.
- Provide category management so the user can view, create, edit, and delete
  categories. If a category being deleted is assigned to transactions, ask
  whether to move those transactions to Uncategorized or another category.
- When deleting a category assigned by rules, disable the affected rules before
  deleting the category.
- When multiple rules conflict, the app should not silently choose a winner.
  Ask the user to resolve the conflict, including by editing the rules or
  choosing a category for the affected transaction. Do not add a separate
  persistent rule-priority system in the first version.
- A rule should not silently replace a manual transaction override.
- Editing a rule affects future matches only; transactions already categorized
  retain their current categories.
- The categorization module should be designed to allow future extensions;
  specific future features are not in the first-version scope yet.

## Interaction principles

- Prefer asking the user to make a decision over taking an unconfigured
  automated action. Saving a rule after previewing its matching transactions
  is the user's explicit decision to apply it to those and future matches.
- Where appropriate, repeated decision prompts may offer a "Don't ask me
  again" option.

## Functional requirements

Write requirements so they can be verified. Use identifiers such as `FR-01`
when a requirement is agreed.

The criteria below are agreed as the first-version behavior baseline. They
define observable behavior, not implementation details. Criteria depending
on bank export samples or technical choices are explicitly deferred.

| ID | Requirement | Priority | Acceptance criteria | Status |
| --- | --- | --- | --- | --- |
| FR-01 | Import bank-export CSV transaction data from Triodos Belgium, ING Belgium, and VDK Belgium. Let the user choose the amount of history by selecting the files to import; impose no application history-range limit. | First version | For each bank, synthetic fixtures verify the documented or user-described layouts. Use sanitized samples when needed to confirm actual file variants. Import all eligible transactions in the selected file or files. The user controls the imported history range by selecting files; the application imposes no time-range limit. | Agreed |
| FR-32 | Let the user include multiple bank-export files from different accounts in one import job. | First version | User can select at least two files for one job; preview and confirmation apply to the job as a whole; each transaction remains associated with the right account and job. | Agreed |
| FR-44 | Limit each uploaded bank CSV file to 1 MB (1,000,000 bytes). | First version | Files up to and including 1,000,000 bytes can be selected for analysis; a larger file is rejected before parsing with a clear file-specific message. The limit applies per file, not to the combined job size. | Agreed |
| FR-30 | Detect the bank/account for an import when possible; ask the user to confirm or select an account when uncertain, and allow creating a new account during import. | First version | Recognized account details are shown for confirmation; unrecognized/ambiguous account mapping requires the user to select or create an account before confirming import; no file is silently assigned to an uncertain account. | Agreed |
| FR-33 | Let the user choose whether multiple conditions in a categorization rule combine with AND or OR. | First version | A rule with multiple conditions can be saved with either logic; test transactions matching all, only one, or no conditions receive the result implied by the selected logic. | Agreed |
| FR-35 | Use posted/settled transactions for financial history and ignore pending transactions in imports. | Deferred | The first version accepts exports containing posted/settled transactions only. Pending-row detection/filtering is deferred until a supported bank layout with an explicit status field is identified. | Deferred by user; 2026-11-20 |
| FR-41 | Use the booking date for transaction lists and trends; retain the value date separately when provided. | First version | When an input row has both dates, lists and monthly totals use the booking date and the value date remains available as a separate detail. | Agreed |
| FR-42 | Determine transaction direction from bank data when clear; let the user resolve ambiguous direction in the import preview; keep direction separate from category. | First version | Clear bank direction is mapped to incoming/outgoing; ambiguous direction requires a user choice in preview; the chosen direction affects totals independently of category. | Agreed |
| FR-38 | Keep imported financial data until the user explicitly deletes it. | First version | Imported data remains available after navigation and restart; no time-based retention removes it; an individual deletion occurs only after the user initiates and confirms it. Whole-app deletion remains out of scope. | Agreed |
| FR-39 | Support current/checking accounts only in the first version. | First version | Current/checking accounts can be created and imported; unsupported account types are not silently treated as checking accounts and are identified to the user. | Agreed |
| FR-37 | After an import job completes, show imported/skipped counts and errors, with an action to view the job's transactions. | First version | A completed job summary reports the job's imported/skipped rows and errors and provides a working action to view its transactions; a rejected job reports that zero rows were imported. No whole-job undo action is provided. | Agreed |
| FR-02 | Preview an import and require user confirmation before saving; show included files/accounts and combined job-level date range, row/valid/invalid counts, possible duplicate count, incoming/outgoing totals, and category totals. Keep validation errors tied to the relevant file and row. | First version | Before confirmation, preview shows all selected files/accounts and the specified combined totals; category totals summarize active-rule assignments. Row-level validation errors identify file and row. Cancelling or closing before confirmation saves nothing. | Agreed |
| FR-03 | Identify possible duplicates when the account or accounts involved, amount, description, and transaction timestamp all match; ask the user whether to skip the imported row, keep both, or update the existing transaction; allow a choice per match or for a group of similar matches. | First version | A row is presented as a possible duplicate only when all four matching fields are equal. For each candidate, skip, keep both, and update-existing choices are available; a per-match choice affects that candidate only, while a group choice affects only the selected similar candidates. | Agreed |
| FR-31 | When an import matches a manually edited transaction, show differing values and ask whether to keep the edits or use the imported values. | First version | For a matched manually edited transaction with differing fields, the preview identifies the differences and requires a keep-edits or use-imported choice before changing those fields. | Agreed |
| FR-04 | If any file in a multi-file import job has an invalid row, report the errors and reject the entire job without importing any files. | First version | If any row in any selected file is invalid, the job imports zero rows, identifies each invalid file/row and reason, and permits retry after the source file is corrected. | Agreed |
| FR-24 | Let the user remove selected transactions from an import batch; do not provide functionality to undo an entire import batch. | First version | Removing selected transactions requires confirmation and leaves other transactions in that job and transactions from other jobs unchanged. A selected transaction linked to a confirmed transfer cannot be removed until the transfer link is resolved. A manually edited, unlinked transaction can be removed after clearly disclosing that its edits will be deleted and asking for confirmation. | Agreed |
| FR-25 | Let the user edit transaction date, amount, description/payee, account, income/spending direction, and transfer status; delete selected transactions with confirmation; and manually back up and restore parsed financial data and categorization setup, without original uploaded files. Category assignment is separate. | First version | Each listed field can be changed and its saved value appears in the transaction; category changes remain in categorization. Manual backup includes parsed transactions, accounts, balances, import-job history, categories, rules, and settings, omits uploaded source files, and warns about sensitive contents. Restore replaces current data only after confirmation; no password protection or Excel export is required. | Agreed |
| FR-29 | Keep imported transactions linked to their import job and allow filtering transactions by import job. | First version | Selecting an import job filter shows only transactions attributed to that job; clearing the filter restores the unfiltered list. | Agreed |
| FR-05 | Show each account's latest known balance and date, plus a combined balance based on those latest known balances. | First version | For each account, display its latest known balance and effective date; the combined figure equals the sum of those latest balances and indicates when component dates differ. | Agreed |
| FR-06 | Show monthly spending by group, total incoming money, and saving (income minus spending), excluding confirmed internal transfers; allow income such as salary to be viewed by category. | First version | For a selected month, spending equals outgoing posted transactions and income equals incoming posted transactions, excluding confirmed internal transfers. Saving equals income minus spending. Amounts by category reconcile to their respective totals, including Uncategorized. | Agreed |
| FR-28 | Treat Uncategorized as a category; include its transactions in income/spending totals according to direction and show Uncategorized separately in category breakdowns. | First version | An uncategorized incoming/outgoing transaction contributes to the corresponding total and appears under Uncategorized in the category breakdown; assigning a category moves it out of that breakdown without changing direction or amount. | Agreed |
| FR-07 | Default the dashboard to the rolling last 12 months; offer 3 months, 6 months, 12 months, year to date, previous calendar year, 3 years, 5 years, all available history, and a custom date range. | First version | Initial dashboard range is the rolling 12 months. Each listed preset and a valid custom date range updates the displayed period; data outside available history is not fabricated. | Agreed |
| FR-34 | Provide a predefined dashboard with time-range and category filters; do not allow adding/rearranging widgets, saving filter combinations, or exporting dashboard reports in the first version. | First version | Dashboard uses a fixed layout; time-range selection updates displayed data and category selection filters all transaction-based cash-flow charts. The account-balance chart always shows actual balances and is not changed by category selection; disclose this exception beside the filters. No widget add/reorder, saved filter combination, or dashboard report export is offered. | Agreed |
| FR-23 | Support EUR account balances only. Convert non-EUR transaction amounts to EUR for combined totals and trends, using a bank-provided EUR equivalent when available; otherwise ask the user to specify the exchange rate for the transaction date. | First version | EUR amounts are unchanged; use a bank-provided EUR equivalent when present; otherwise request one user-specified rate per source currency and transaction date and apply it to matching transactions in that import preview. Account balances in non-EUR are not accepted; no external historical rate provider is planned. | Agreed |
| FR-08 | Assign income and spending transactions to categories using rules based on transaction details, while allowing per-transaction overrides and user-created categories and rules. | First version | A saved rule can match transaction direction/amount, source, payee, or description and assign its category to matching income or spending transactions. A user can assign a category manually to one transaction. | Agreed |
| FR-09 | Do not silently resolve conflicting matching rules or replace a manual transaction override; ask the user to choose a category or edit the rules. | First version | When multiple rules match with different categories, the app does not apply an arbitrary winner; it asks the user to choose a category or edit rules. A manual category is not silently replaced. | Agreed |
| FR-10 | Editing a rule affects future matches only; existing transactions retain their current categories. | First version | After editing/saving a rule, previously categorized transactions keep their categories; transactions matched after the edit use the updated rule. | Agreed |
| FR-11 | Suggest transfers between accounts imported by the user when transactions have equal amounts in opposite directions within three days; require user confirmation and show confirmed transfers separately from income and spending. | First version | Transactions from imported accounts with equal absolute amounts, opposite directions, and dates no more than three days apart are offered as a candidate; no candidate is treated as a transfer until confirmed. Both transactions remain in their account histories and balance calculations. Confirmed pairs are excluded from income, spending, saving, and category totals and require no categorization. | Agreed |
| FR-12 | Show changes in account balances over time. | First version | For a selected period, monthly balance values are shown from available balance and transaction history; changing the period updates the values without inventing unavailable history. | Agreed |
| FR-40 | For a month with only partially available transaction history, show totals for the available transactions without a special partial-month marker. | First version | When imported history begins or ends mid-month, that month's charts show totals from available transactions and add no partial-month marker. The app does not detect or manage transaction-history gaps. | Agreed |
| FR-13 | Host the app on Proxmox as a browser-based web application, accessible from the user's private home network without client installation or sign-in. | First version | Current Chrome on a laptop can access the app from the private home network without installing client software or signing in; the app is served by the Proxmox host and is not exposed outside the agreed private network. Other browsers and phone layouts are nice to have. | Agreed |
| FR-14 | Let the user enter or update an account balance and its effective date; calculate historical balances from that point using available imported transactions. | First version | Saving a known balance with an effective date recalculates the account's history using available transactions; changing the anchor balance/date updates the derived balances. | Agreed |
| FR-16 | Provide a separate categorization module with an Uncategorized review list sorted by most frequent/repeated payees by default; surface repeated payees/senders, allow filtering, amount sorting, bulk categorization, and saving a filter as a rule. | First version | Categorization is reachable as a separate module. Uncategorized entries default to most frequent/repeated payees; user can filter/sort by amount, select multiple entries, assign a category in one action, and save a filter as a rule. | Agreed |
| FR-17 | Preview existing transactions matching a new rule; when saved, apply it to those and future matches, but ask individually before replacing a manually assigned category. | First version | Rule creation shows existing matches before save. Saving applies the rule to reviewed matches and future matches; each manually categorized match requires an explicit replace/keep decision. | Agreed |
| FR-18 | Provide a place in the categorization module to view/search, edit, enable/disable, and delete categorization rules. | First version | User can view/search rules and edit, enable, disable, or delete one. Disabling/deleting asks whether to keep existing categorizations or return affected transactions to Uncategorized. | Agreed |
| FR-19 | Let the user view, create, edit, and delete categories; when deleting a category in use, ask whether to move its transactions to Uncategorized or another category. | First version | User can view/create/edit/delete categories. Deleting an assigned category requires choosing Uncategorized or another category for affected transactions; no transaction is left referencing a deleted category. | Agreed |
| FR-43 | When deleting a category used by rules, disable the affected rules before deleting the category. | First version | Before deletion completes, every rule assigning the category is disabled; the user is told which rules were affected. Existing transaction handling follows FR-19 and FR-18. | Agreed |
| FR-20 | Allow categories to be used flexibly for income or spending transactions. | First version | The same user-created category can be assigned to an incoming or outgoing transaction; direction and category remain independently stored/displayed. | Agreed |
| FR-21 | During first-run configuration, offer built-in starter categories and rules and allow importing UTF-8 JSON starter-set files; let the user select categories and rules independently and enable selected starter rules after preview and confirmation. | First version | First-run setup offers the built-in sets and JSON file import; the user can select categories and rules independently and choose rules individually. Before confirmation, show categories to add/reuse and existing transactions matched by selected rules. Only confirmed items are imported; selected rules are enabled. See the [starter-set specification](starter-sets.md). | Agreed |
| FR-22 | Prefer asking for user decisions over unconfigured automation and, where appropriate, offer a "Don't ask me again" option. | First version | The app does not silently take a decision that requires user input under the agreed flows. Where a repeated prompt offers "Don't ask me again," the choice is stored and suppresses that same prompt thereafter; exact prompts and scope are finalized in experience design. | Agreed |
| FR-26 | Provide an English interface optimized for Chrome on laptops. Support for other browsers and phone use is nice to have. | First version | All first-version workflows are usable in English in current Chrome on a laptop. Other browsers and phone layouts are not required for acceptance, but should work where practical. | Agreed |
## Data and user controls

- Let the user edit transaction details and delete selected transactions, with
  a clear confirmation before deletion.
- Editable transaction details include date, amount, description/payee,
  account, income/spending direction, and transfer status. Category assignment
  remains a separate categorization action.
- Let the user back up all app data and restore from a backup. A separate
  human-readable export for use in Excel is not required in the first version.
- Data export/backup should include the underlying financial data and
  categorization setup (including categories and rules); dashboards are
  generated from that data and are not exported as reports.
- Include parsed transactions, accounts, balances, import-job history,
  categories, rules, and app settings in a backup, but omit the original
  uploaded bank-export files.
- Backups are initiated manually in the first version; scheduled automatic
  backups are out of scope.
- An ordinary backup file is acceptable; built-in password protection is not
  required. Clearly warn that the file contains sensitive financial data.
- Restoring a backup replaces the current app data only after the user confirms
  the replacement.
- Deleting all app data is not part of the first-version scope.
- Keep imported financial data until the user explicitly deletes it.

## Quality expectations

Define user-visible expectations for accessibility, usability, performance,
reliability, and supported browsers/devices.

- Interface language: English.
- Optimize the first version for laptop browsers; provide basic phone-browser
  support as a nice-to-have. Current Chrome on a laptop is the required target;
  other browsers and tablet support are nice-to-have.
- No fixed import or page-response speed target is required for the first
  version; prioritize data correctness and predictable recovery.
