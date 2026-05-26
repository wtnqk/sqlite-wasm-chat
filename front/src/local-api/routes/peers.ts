import { Hono } from "hono";
import { getDb } from "../../db";

export type Peer = {
  id: string;
  name: string;
  connected: number;
  joined_at: number;
};

const peers = new Hono();

// GET /peers?connected=1
peers.get("/", (c) => {
  const db = getDb();
  const onlyConnected = c.req.query("connected") === "1";
  const rows = onlyConnected
    ? db.selectObjects<Peer>("SELECT * FROM peers WHERE connected = 1")
    : db.selectObjects<Peer>("SELECT * FROM peers");
  return c.json(rows);
});

// POST /peers  ピアを登録 (接続時)
peers.post("/", async (c) => {
  const { id, name, joined_at } = await c.req.json<Peer>();
  const db = getDb();
  db.exec(
    "INSERT OR REPLACE INTO peers (id, name, connected, joined_at) VALUES (?,?,1,?)",
    { bind: [id, name, joined_at] },
  );
  return c.json({ ok: true }, 201);
});

// PUT /peers/:id/disconnect  切断時
peers.put("/:id/disconnect", (c) => {
  const db = getDb();
  db.exec("UPDATE peers SET connected = 0 WHERE id = ?", {
    bind: [c.req.param("id")],
  });
  return c.json({ ok: true });
});

export { peers };
