import type { Database } from "@sqlite.org/sqlite-wasm";

// migrate はアプリ起動時に一度だけ呼ぶ。
// IF NOT EXISTS により冪等に実行できる。
export function migrate(db: Database): void {
  db.exec(`
    CREATE TABLE IF NOT EXISTS rooms (
      id   TEXT PRIMARY KEY,
      name TEXT NOT NULL
    );

    CREATE TABLE IF NOT EXISTS peers (
      id         TEXT PRIMARY KEY,
      name       TEXT NOT NULL,
      connected  INTEGER NOT NULL DEFAULT 1,
      joined_at  INTEGER NOT NULL
    );

    CREATE TABLE IF NOT EXISTS messages (
      id       TEXT PRIMARY KEY,
      room_id  TEXT NOT NULL REFERENCES rooms(id),
      peer_id  TEXT NOT NULL REFERENCES peers(id),
      body     TEXT NOT NULL,
      sent_at  INTEGER NOT NULL,
      read_at  INTEGER
    );

    CREATE INDEX IF NOT EXISTS idx_messages_room ON messages(room_id, sent_at);
  `);
}
