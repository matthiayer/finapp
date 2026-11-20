-- +goose Up
CREATE TABLE accounts (
    id INTEGER PRIMARY KEY,
    bank TEXT NOT NULL,
    account_number TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    account_type TEXT NOT NULL DEFAULT 'current'
        CHECK (account_type = 'current'),
    currency TEXT NOT NULL DEFAULT 'EUR',
    balance_anchor_amount TEXT,
    balance_anchor_date TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (bank, account_number)
) STRICT;

CREATE TABLE categories (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    is_system INTEGER NOT NULL DEFAULT 0 CHECK (is_system IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

INSERT INTO categories (name, is_system) VALUES ('Uncategorized', 1);

CREATE TABLE import_jobs (
    id INTEGER PRIMARY KEY,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    source_files_json TEXT NOT NULL,
    file_count INTEGER NOT NULL CHECK (file_count >= 1),
    row_count INTEGER NOT NULL CHECK (row_count >= 0),
    imported_count INTEGER NOT NULL CHECK (imported_count >= 0),
    skipped_count INTEGER NOT NULL CHECK (skipped_count >= 0)
) STRICT;

CREATE TABLE transfers (
    id INTEGER PRIMARY KEY,
    confirmed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE transactions (
    id INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    booking_date TEXT NOT NULL,
    value_date TEXT,
    amount TEXT NOT NULL,
    currency TEXT NOT NULL,
    eur_amount TEXT NOT NULL,
    exchange_rate TEXT,
    fees TEXT,
    description TEXT NOT NULL DEFAULT '',
    payee TEXT NOT NULL DEFAULT '',
    counterparty_account TEXT NOT NULL DEFAULT '',
    reference TEXT NOT NULL DEFAULT '',
    balance_after TEXT,
    direction TEXT NOT NULL CHECK (direction IN ('incoming', 'outgoing', 'unknown')),
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    import_job_id INTEGER NOT NULL REFERENCES import_jobs(id) ON DELETE RESTRICT,
    source_file_name TEXT NOT NULL,
    source_row INTEGER NOT NULL CHECK (source_row >= 1),
    pending INTEGER NOT NULL DEFAULT 0 CHECK (pending IN (0, 1)),
    is_manually_edited INTEGER NOT NULL DEFAULT 0 CHECK (is_manually_edited IN (0, 1)),
    is_manual_category INTEGER NOT NULL DEFAULT 0 CHECK (is_manual_category IN (0, 1)),
    transfer_id INTEGER REFERENCES transfers(id) ON DELETE RESTRICT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE transaction_imports (
    transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    import_job_id INTEGER NOT NULL REFERENCES import_jobs(id) ON DELETE RESTRICT,
    source_file_name TEXT NOT NULL,
    source_row INTEGER NOT NULL CHECK (source_row >= 1),
    action TEXT NOT NULL CHECK (action IN ('inserted', 'updated', 'kept_both')),
    PRIMARY KEY (transaction_id, import_job_id, source_file_name, source_row)
) STRICT;

CREATE INDEX transactions_account_date_idx
    ON transactions (account_id, booking_date);
CREATE INDEX transactions_job_idx ON transactions (import_job_id);
CREATE INDEX transaction_imports_job_idx ON transaction_imports (import_job_id);
CREATE INDEX transactions_category_date_idx
    ON transactions (category_id, booking_date);
CREATE INDEX transactions_duplicate_idx
    ON transactions (account_id, amount, description, booking_date, currency);

-- +goose Down
DROP INDEX transactions_duplicate_idx;
DROP INDEX transactions_category_date_idx;
DROP INDEX transaction_imports_job_idx;
DROP INDEX transactions_job_idx;
DROP INDEX transactions_account_date_idx;
DROP TABLE transaction_imports;
DROP TABLE transactions;
DROP TABLE transfers;
DROP TABLE import_jobs;
DROP TABLE categories;
DROP TABLE accounts;
