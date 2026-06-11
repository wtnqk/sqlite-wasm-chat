// SQLite の WASM ビルドを動的に読み込む関数。
// ブラウザ上で SQLite を動かすために WebAssembly バイナリを fetch して初期化する。
import sqlite3InitModule from "@sqlite.org/sqlite-wasm";
// Database 型はランタイムには不要だが、TypeScript の型チェックに使う（型のみのインポート）。
import type { Database } from "@sqlite.org/sqlite-wasm";
import { migrate } from "./migrate";

// モジュールスコープの変数として DB インスタンスを保持する。
// null = まだ初期化されていない状態。init() が完了すると代入される。
let db: Database | null = null;

// init は SQLite WASM を初期化してマイグレーションを実行する。
// アプリ起動時に一度だけ呼ぶ。以降は getDb() で取得する。
export async function init(): Promise<void> {
  // WASM バイナリのロードは非同期なので await で完了を待つ。
  // print を空関数にすることで SQLite 内部のログ出力を抑制している。
  // @ts-expect-error -- @sqlite.org/sqlite-wasm の型定義が引数なしで宣言されているが実装は受け取る
  const sqlite3 = await sqlite3InitModule({ print: () => {}, printErr: console.error });

  // OPFS (Origin Private File System) はブラウザが提供するファイルシステム API。
  // 対応ブラウザ (Chrome など) では DB をファイルとして永続化できる。
  // 非対応ブラウザではメモリ上の DB にフォールバックする（ページを閉じると消える）。
  if ("opfs" in sqlite3) {
    db = new sqlite3.oo1.OpfsDb("/chat.db");
  } else {
    db = new sqlite3.oo1.DB("/chat.db", "c"); // "c" = 存在しなければ作成
  }

  // テーブル定義などのスキーマを適用する。冪等に設計されているため何度呼んでも安全。
  migrate(db);
}

// getDb は初期化済みの Database を返す。
// init() より前に呼ぶと例外になる。
export function getDb(): Database {
  if (!db) throw new Error("db: not initialized. call init() first");
  return db;
}
