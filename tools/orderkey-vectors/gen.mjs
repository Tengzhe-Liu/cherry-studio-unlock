/**
 * 用参考实现（fractional-indexing.js，取自 Cherry Studio 包内）生成测试向量。
 *
 * 运行：
 *     node tools/orderkey-vectors/gen.mjs
 *
 * 输出：internal/store/testdata/orderkey_vectors.json
 * Go 侧的 TestGenerateKeyBetweenMatchesReference 会把这份向量逐条比对，
 * 保证我们的移植与 Cherry Studio 自己生成的 order_key 完全一致。
 */

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { generateKeyBetween } from "./fractional-indexing.js";

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = path.resolve(here, "../../internal/store/testdata/orderkey_vectors.json");

const vectors = [];
const add = (a, b, note) => {
  vectors.push({ a, b, want: generateKeyBetween(a, b), note });
};

// 1) 空表 → 第一个键
add(null, null, "first key on an empty table");

// 2) 反复追加：模拟连续新建 Agent
let keys = [];
let max = null;
for (let i = 0; i < 40; i++) {
  const k = generateKeyBetween(max, null);
  add(max, null, `append #${i + 1}`);
  keys.push(k);
  max = k;
}

// 3) 在相邻两键之间插入（分列、末列等）
for (let i = 0; i + 1 < keys.length; i++) {
  add(keys[i], keys[i + 1], `between #${i + 1} and #${i + 2}`);
}

// 4) 跳到较远的两点之间
for (let i = 0; i + 5 < keys.length; i += 6) {
  add(keys[i], keys[i + 5], `between #${i + 1} and #${i + 6}`);
}

// 5) 插到最前面
add(null, keys[0], "insert before the first key");
add(null, keys[1], "insert before the second key");

// 6) 跨整数段边界（Zz → a0 那段）
add("Zz", null, "append after the last key of the Z-run");
add("Zy", "a0", "between the Z-run and the a-run");
add("a0", "a1", "between the first two a-run keys");
add("a9", "aA", "between digit-run and letter-run");
add("az", null, "append after az");
add("a3", null, "the real-world case: current max in a fresh install is a3");

fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, JSON.stringify(vectors, null, 2) + "\n", "utf8");
console.log(`wrote ${vectors.length} vectors -> ${outFile}`);
