const { test } = require("node:test");
const assert = require("node:assert/strict");
const path = require("node:path");
const { analyze } = require("./analyzer.cjs");
const root = path.resolve(__dirname, "../../testdata/projects/typescript");
const config = {
  Modules: {
    world: {
      Language: "typescript",
      Paths: ["world"],
      Seams: { reader: { Symbols: [], Entrypoints: ["world/index.ts"] } },
    },
    npc: { Language: "typescript", Paths: ["npc"], Seams: {} },
  },
};
test("compiler resolves barrel, alias and type-only members", () => {
  const s = analyze({ version: 1, root, config });
  assert.equal(s.diagnostics.length, 0);
  assert.ok(
    s.seams[0].contractMembers.some((id) => id.endsWith("Reader.read")),
  );
  assert.ok(
    s.relations.some((r) => r.kind === "uses" && r.toId.endsWith("open")),
  );
  assert.ok(s.relations.every((r) => !r.bypassesSeam));
});
test("protocol mismatch is explicit", () =>
  assert.throws(() => analyze({ version: 99, root, config }), /Protocol/));

test("anonymous default classes preserve public members and structural facts", () => {
  const fs = require("node:fs"),
    os = require("node:os");
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), "seamgraph-default-"));
  try {
    fs.cpSync(root, temp, { recursive: true });
    fs.writeFileSync(
      path.join(temp, "world/internal.ts"),
      "export interface Reader {read(): string}\nexport default class { read(): string {return 'world'} }\n",
    );
    fs.writeFileSync(
      path.join(temp, "world/index.ts"),
      "export {default} from './internal'; export type {Reader} from './internal';\n",
    );
    fs.writeFileSync(
      path.join(temp, "npc/main.ts"),
      "import World from '@world'; export const value = new World().read();\n",
    );
    const s = analyze({ version: 1, root: temp, config });
    assert.equal(s.diagnostics.length, 0);
    assert.ok(
      s.seams[0].contractMembers.some((id) => id.endsWith("default.read")),
    );
    assert.ok(s.relations.some((r) => r.kind === "implements"));
    assert.equal(new Set(s.symbols.map((x) => x.id)).size, s.symbols.length);
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
  }
});

test("readonly, optional, accessor and constructor changes remain contract changes", () => {
  const fs = require("node:fs"),
    os = require("node:os");
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), "seamgraph-signatures-"));
  try {
    fs.cpSync(root, temp, { recursive: true });
    fs.writeFileSync(
      path.join(temp, "world/index.ts"),
      "export {default} from './internal';\n",
    );
    fs.writeFileSync(
      path.join(temp, "npc/main.ts"),
      "import type World from '@world'; export type Public = World;\n",
    );
    const source =
      "export default class { readonly value?: string; constructor(){} get current(): string {return 'world'} set current(value: string){} }\n";
    const signature = (code) => {
      fs.writeFileSync(path.join(temp, "world/internal.ts"), code);
      const s = analyze({ version: 1, root: temp, config });
      assert.equal(s.diagnostics.length, 0);
      return s.symbols.find((s) => s.id.endsWith("%23default"))
        .displaySignature;
    };
    const before = signature(source);
    for (const code of [
      source.replace("readonly ", ""),
      source.replace("value?:", "value!:"),
      source.replace(
        "set current(value: string)",
        "set current(value: string | number)",
      ),
      source.replace("constructor()", "private constructor()"),
    ])
      assert.notEqual(signature(code), before);
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
  }
});

test("one type shared by multiple seams has one unchanged signature", () => {
  const one = analyze({ version: 1, root, config });
  const multiple = structuredClone(config);
  multiple.Modules.world.Seams.second = structuredClone(
    multiple.Modules.world.Seams.reader,
  );
  const two = analyze({ version: 1, root, config: multiple });
  const signature = (s) =>
    s.symbols.find((x) => x.id.endsWith("%23Reader")).displaySignature;
  assert.equal(signature(one), signature(two));
  assert.equal(two.seams.length, 2);
});
