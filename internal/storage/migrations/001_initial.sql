-- +goose Up
CREATE TABLE app_settings (
    setting_key TEXT PRIMARY KEY NOT NULL CHECK (setting_key <> ''),
    setting_value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

-- +goose Down
DROP TABLE app_settings;
