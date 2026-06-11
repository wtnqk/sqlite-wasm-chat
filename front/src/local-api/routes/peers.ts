import { Hono } from "hono";
import { getDb } from "../../db";

/**
 * チャット参加者（ピア）を表す型。
 * SQLite の peers テーブルの1行に対応する。
 */
export type Peer = {
  /** UUID などの一意な識別子 */
  id: string;
  /** 表示名 */
  name: string;
  /** 接続状態。1 = 接続中、0 = 切断済み */
  connected: number;
  /** 参加日時 (Unix ミリ秒) */
  joined_at: number;
};

/**
 * /peers ルートハンドラー。
 *
 * | メソッド | パス                  | 説明                         |
 * | -------- | --------------------- | ---------------------------- |
 * | GET      | /?connected=1         | ピア一覧を取得               |
 * | POST     | /                     | ピアを登録 (接続時)          |
 * | PUT      | /:id/disconnect       | ピアを切断済みにする         |
 */
const peers = new Hono()
  // GET /peers?connected=1
  .get("/", (c) => {
    const db = getDb();
    // クエリパラメータ connected=1 のときは接続中のピアだけ返す
    const onlyConnected = c.req.query("connected") === "1";
    let rows: Peer[];
    if (onlyConnected) {
      // @ts-expect-error -- @sqlite.org/sqlite-wasm の型定義が型引数なしで宣言されているが実装は受け取る
      rows = db.selectObjects<Peer>("SELECT * FROM peers WHERE connected = 1");
    } else {
      // @ts-expect-error -- @sqlite.org/sqlite-wasm の型定義が型引数なしで宣言されているが実装は受け取る
      rows = db.selectObjects<Peer>("SELECT * FROM peers");
    }
    return c.json(rows);
  })
  // POST /peers  ピアを登録 (接続時)
  .post("/", async (c) => {
    const { id, name, joined_at } = await c.req.json<Peer>();
    const db = getDb();
    // INSERT OR REPLACE で同じ id が来たときも安全に上書きする
    db.exec(
      "INSERT OR REPLACE INTO peers (id, name, connected, joined_at) VALUES (?,?,1,?)",
      { bind: [id, name, joined_at] },
    );
    return c.json({ ok: true }, 201);
  })
  // PUT /peers/:id/disconnect  切断時
  .put("/:id/disconnect", (c) => {
    const db = getDb();
    // connected を 0 にするだけでレコードは残す (メッセージ履歴でピア名を参照できるようにするため)
    db.exec("UPDATE peers SET connected = 0 WHERE id = ?", {
      bind: [c.req.param("id")],
    });
    return c.json({ ok: true });
  });

export { peers };
