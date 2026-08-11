import { copyFileSync, existsSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";
const root = join(dirname(fileURLToPath(import.meta.url)), "..", "src", "routes");
const sk = join(root, "_sk");
for (const [s,d] of [["layout.svelte","+layout.svelte"],["page.svelte","+page.svelte"]]) {
  const f=join(sk,s), t=join(root,d);
  if (existsSync(f)) { copyFileSync(f,t); console.log("created", d); }
}
console.log("done");
