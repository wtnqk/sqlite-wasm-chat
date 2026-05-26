import { hc } from "hono/client";
import { app } from "../local-api";
import type { AppType } from "../local-api";

// localClient は Hono local app を直接呼ぶクライアント。
// fetch をオーバーライドして app.fetch() に差し替えることで
// ネットワークを介さず SQLite に到達する。
export const localClient = hc<AppType>("http://local", {
  fetch: (input, init) => app.fetch(new Request(input, init)),
});
