CREATE TABLE IF NOT EXISTS rate_limits (
  scope    TEXT NOT NULL,
  key      TEXT NOT NULL,
  count    INTEGER NOT NULL,
  reset_at TEXT NOT NULL,
  PRIMARY KEY (scope, key)
);
