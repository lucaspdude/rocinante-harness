CREATE TABLE IF NOT EXISTS ssh_keys (
  id          TEXT PRIMARY KEY,
  label       TEXT NOT NULL,
  provider    TEXT NOT NULL,
  public_key  TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ssh_servers (
  id          TEXT PRIMARY KEY,
  label       TEXT NOT NULL,
  host        TEXT NOT NULL,
  port        INTEGER NOT NULL,
  username    TEXT NOT NULL,
  key_id      TEXT NOT NULL REFERENCES ssh_keys(id),
  created_at  TEXT NOT NULL
);
