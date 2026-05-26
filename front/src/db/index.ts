import sqlite3InitModule from "@sqlite.org/sqlite-wasm";
import type { Database } from "@sqlite.org/sqlite-wasm";
import { migrate } from "./migrate";

let db: Database | null = null;

// init は SQLite WASM を初期化してマイグレーションを実行する。
// アプリ起動時に一度だけ呼ぶ。以降は getDb() で取得する。
export async function init(): Promise<void> {
  const sqlite3 = await sqlite3InitModule({ print: () => {}, printErr: console.error });

  // OPFS が使えれば永続化、使えなければメモリ (Step 5 で切り替え予定)
  if ("opfs" in sqlite3) {
    db = new sqlite3.oo1.OpfsDb("/chat.db");
  } else {
    db = new sqlite3.oo1.DB("/chat.db", "c");
  }

  migrate(db);
}

// getDb は初期化済みの Database を返す。
// init() より前に呼ぶと例外になる。
export function getDb(): Database {
  if (!db) throw new Error("db: not initialized. call init() first");
  return db;
}
