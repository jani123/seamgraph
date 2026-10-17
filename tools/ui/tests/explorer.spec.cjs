const { test, expect, chromium: playwright } = require("@playwright/test");
const chromium = require("@sparticuz/chromium");
const fs = require("node:fs"),
  path = require("node:path"),
  os = require("node:os"),
  { spawn, execFileSync } = require("node:child_process");
let root, server, browser, context, page;
const repo = path.resolve(__dirname, "../../.."),
  url = "http://127.0.0.1:18765";
test.beforeAll(async () => {
  root = fs.mkdtempSync(path.join(os.tmpdir(), "seamgraph-browser-"));
  fs.cpSync(path.join(repo, "testdata/projects/go"), root, { recursive: true });
  execFileSync(
    "go",
    ["build", "-o", path.join(repo, ".build/seamgraph"), "./cmd/seamgraph"],
    { cwd: repo },
  );
  server = spawn(
    path.join(repo, ".build/seamgraph"),
    ["serve", "--root", root, "--addr", "127.0.0.1:18765"],
    { cwd: repo, stdio: "pipe" },
  );
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(url)).ok) break;
    } catch {}
    await new Promise((r) => setTimeout(r, 50));
  }
  const nixLibs = fs.existsSync("/nix/store")
    ? fs
        .readdirSync("/nix/store")
        .filter((n) => /-(nspr|nss|expat)-/.test(n) && !n.endsWith(".drv"))
        .map((n) => path.join("/nix/store", n, "lib"))
        .filter((p) => fs.existsSync(p))
    : [];
  let executablePath = await chromium.executablePath();
  if (nixLibs.length) {
    const glibc = fs
      .readdirSync("/nix/store")
      .filter(
        (n) =>
          /-glibc-[0-9]/.test(n) &&
          fs.existsSync(path.join("/nix/store", n, "lib/ld-linux-x86-64.so.2")),
      )
      .sort((a, b) =>
        a
          .slice(a.indexOf("-glibc-"))
          .localeCompare(b.slice(b.indexOf("-glibc-")), undefined, {
            numeric: true,
          }),
      )
      .at(-1);
    const lib = path.join("/nix/store", glibc, "lib");
    const patch = fs
      .readdirSync("/nix/store")
      .filter((n) => n.includes("-patchelf-"))
      .map((n) => path.join("/nix/store", n, "bin/patchelf"))
      .find((p) => fs.existsSync(p));
    if (!patch) throw Error("Nix browser tests need patchelf");
    execFileSync(patch, [
      "--set-interpreter",
      lib + "/ld-linux-x86-64.so.2",
      "--set-rpath",
      [lib, ...nixLibs, "/tmp"].join(":"),
      executablePath,
    ]);
  }

  browser = await playwright.launch({
    executablePath,
    args: chromium.args.filter(
      (a) =>
        ![
          "--disable-web-security",
          "--allow-running-insecure-content",
        ].includes(a),
    ),
    env: {
      ...process.env,
      FONTCONFIG_PATH: "/tmp/fonts",
      FONTCONFIG_FILE: "/tmp/fonts/fonts.conf",
      LD_LIBRARY_PATH: [...nixLibs, process.env.LD_LIBRARY_PATH || ""].join(
        ":",
      ),
    },
    headless: true,
  });
  context = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
    permissions: ["clipboard-read", "clipboard-write"],
  });
  page = await context.newPage();
});
test.afterAll(async () => {
  await browser?.close();
  server?.kill();
  fs.rmSync(root, { recursive: true, force: true });
});
test("offline graph, selection, copy IDs, seam consumers, and confirmed editing", async () => {
  const external = [],
    errors = [];
  page.on("request", (r) => {
    if (!r.url().startsWith(url)) external.push(r.url());
  });
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto(url);
  await expect(page.locator("#summary")).toContainText("2 modules");
  await expect(page.locator("#status")).not.toContainText("Error");
  await page.waitForFunction(
    () => document.querySelector("#graph")._cyreg?.cy?.nodes().length === 2,
  );
  // Real canvas click at a rendered node position.
  const position = await page.evaluate(() => {
    const el = document.querySelector("#graph"),
      n = el._cyreg.cy.getElementById("module:world"),
      p = n.renderedPosition(),
      r = el.getBoundingClientRect();
    return { x: r.x + p.x, y: r.y + p.y };
  });
  await page.mouse.click(position.x, position.y);
  await expect(page.locator("#details")).toContainText("seam:world.reader");
  await expect(page.locator("#details")).toContainText("module:npc");
  await expect(page.locator("#details")).toContainText(
    "Declared contract requirement verification",
  );
  await page.locator('[data-copy="seam:world.reader"]').click();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe("seam:world.reader");
  await page.locator('[data-expand-module="module:world"]').click();
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          document
            .querySelector("#graph")
            ._cyreg.cy.getElementById("seam:world.reader").length,
      ),
    )
    .toBe(1);
  await page.locator('[data-expand-module="module:world"]').click();
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          document
            .querySelector("#graph")
            ._cyreg.cy.getElementById("seam:world.reader").length,
      ),
    )
    .toBe(0);
  const inspect = execFileSync(
    path.join(repo, ".build/seamgraph"),
    ["inspect", "seam:world.reader", "--root", root, "--json"],
    { encoding: "utf8" },
  );
  expect(JSON.parse(inspect).entity.id).toBe("seam:world.reader");
  await page.getByRole("button", { name: "About evidence" }).click();
  await expect(
    page.getByText("Edges show observed dependencies.", { exact: false }),
  ).toBeVisible();
  await page.evaluate(() =>
    document.querySelector("#graph")._cyreg.cy.edges().first().emit("tap"),
  );
  await expect(page.locator("#details")).toContainText("Observed relationship");
  const edgeCopy = page.locator('#details [data-copy^="edge:"]').first();
  const edgeId = await edgeCopy.getAttribute("data-copy");
  await edgeCopy.click();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe(edgeId);
  const cfg = path.join(root, "seamgraph.yaml"),
    before = fs.readFileSync(cfg, "utf8");
  await page.locator('input[name="to"]').fill("module:world");
  await page.getByRole("button", { name: "Preview YAML change" }).click();
  await expect(page.locator("#proposal")).toContainText("Exact YAML diff");
  expect(fs.readFileSync(cfg, "utf8")).toBe(before);
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.locator("#proposal")).toBeEmpty();
  expect(fs.readFileSync(cfg, "utf8")).toBe(before);
  await page.getByRole("button", { name: "Preview YAML change" }).click();
  await expect(page.locator("#proposal")).toContainText("Exact YAML diff");
  await page.getByRole("button", { name: "Confirm and write rule" }).click();
  await expect(page.locator("#proposal")).toContainText("Rule saved");
  expect(fs.readFileSync(cfg, "utf8")).toContain("to: world");
  expect(fs.readFileSync(cfg + ".bak", "utf8")).toBe(before);
  await page.screenshot({
    path: path.join(repo, ".build/explorer.png"),
    fullPage: true,
  });
  expect(external).toEqual([]);
  expect(errors).toEqual([]);
});

test("100 modules and 500 edges remain filterable", async () => {
  const modules = Array.from({ length: 100 }, (_, i) => ({
    id: "module:m" + i,
    displayName: "m" + i,
    sourceRoots: ["src/m" + i],
    seams: [],
  }));
  const relations = [];
  for (let i = 0; i < 100; i++)
    for (let j = 1; j <= 5; j++) {
      const from = modules[i].id,
        to = modules[(i + j) % 100].id;
      relations.push({
        id: "edge:" + i + "-" + j,
        fromId: from,
        toId: to,
        kind: "imports",
        evidence: [],
      });
    }
  await page.route("**/api/model", (route) =>
    route.fulfill({
      json: {
        snapshot: {
          schemaVersion: 1,
          modules,
          seams: [],
          symbols: [],
          relations,
          diagnostics: [],
        },
        check: { violations: [], incomplete: false },
      },
    }),
  );
  const started = Date.now();
  await page.goto(url);
  await expect(page.locator("#summary")).toContainText("100 modules");
  await page.waitForFunction(
    () => document.querySelector("#graph")._cyreg?.cy?.edges().length === 500,
  );
  expect(Date.now() - started).toBeLessThan(10000);
  await page.locator("#search").fill("module:m42");
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          document.querySelector("#graph")._cyreg.cy.nodes(":visible").length,
      ),
    )
    .toBe(1);
  await page.screenshot({
    path: path.join(repo, ".build/explorer-large.png"),
    fullPage: true,
  });
  await page.unroute("**/api/model");
});

test("stale confirmation and invalid rule proposals fail visibly", async () => {
  await page.goto(url);
  await expect(page.locator("#summary")).toContainText("2 modules");
  await page.waitForFunction(
    () => document.querySelector("#graph")._cyreg?.cy?.edges().length > 0,
  );
  await page.evaluate(() =>
    document.querySelector("#graph")._cyreg.cy.edges().first().emit("tap"),
  );
  await expect(page.locator("#details")).toContainText("Observed relationship");
  await page.locator('input[name="to"]').fill("module:absent");
  await page.getByRole("button", { name: "Preview YAML change" }).click();
  await expect(page.locator("#status")).toContainText("unknown provider");
  await page.locator('input[name="to"]').fill("seam:world.reader");
  await page.locator('select[name="effect"]').selectOption("deny");
  // Remove the existing allow for this pair in the fixture, then propose a fresh
  // valid rule. The edit is local to this disposable project.
  const cfg = path.join(root, "seamgraph.yaml");
  let data = fs.readFileSync(cfg, "utf8");
  data = data.replace(/  allow:[\s\S]*$/, "");
  fs.writeFileSync(cfg, data);
  await page.getByRole("button", { name: "Preview YAML change" }).click();
  await expect(page.locator("#proposal")).toContainText("Exact YAML diff");
  fs.appendFileSync(cfg, "# concurrent external edit\n");
  await page.getByRole("button", { name: "Confirm and write rule" }).click();
  await expect(page.locator("#status")).toContainText(
    "configuration changed since preview",
  );
  expect(fs.readFileSync(cfg, "utf8")).toContain("# concurrent external edit");
  expect(fs.readFileSync(cfg, "utf8")).not.toContain("deny:");
});

for (const language of ["typescript", "python"])
  test(`${language} browser IDs resolve through CLI`, async () => {
    fs.rmSync(root, { recursive: true, force: true });
    fs.cpSync(path.join(repo, "testdata/projects", language), root, {
      recursive: true,
    });
    await page.goto(url);
    await expect(page.locator("#summary")).toContainText("2 modules");
    await page.waitForFunction(
      () => document.querySelector("#graph")._cyreg?.cy?.nodes().length === 2,
    );
    await page.evaluate(() =>
      document
        .querySelector("#graph")
        ._cyreg.cy.getElementById("module:world")
        .emit("tap"),
    );
    await expect(page.locator("#details")).toContainText("seam:world.reader");
    await page.locator('[data-copy="seam:world.reader"]').click();
    await expect
      .poll(() => page.evaluate(() => navigator.clipboard.readText()))
      .toBe("seam:world.reader");
    const inspect = JSON.parse(
      execFileSync(
        path.join(repo, ".build/seamgraph"),
        ["inspect", "seam:world.reader", "--root", root, "--json"],
        { encoding: "utf8" },
      ),
    );
    expect(inspect.entity.id).toBe("seam:world.reader");
    expect(inspect.relations.length).toBeGreaterThan(0);
  });

test("Git comparison overlays regressions and removed IDs remain inspectable", async () => {
  fs.rmSync(root, { recursive: true, force: true });
  fs.cpSync(path.join(repo, "testdata/projects/go"), root, { recursive: true });
  const git = (args) => execFileSync("git", ["-C", root, ...args]);
  git(["init", "-q"]);
  git(["add", "."]);
  git([
    "-c",
    "user.name=Test",
    "-c",
    "user.email=test@example.invalid",
    "commit",
    "-qm",
    "base",
  ]);
  const world = path.join(root, "world/world.go");
  fs.writeFileSync(
    world,
    fs
      .readFileSync(world, "utf8")
      .replace("func Open()", "func Open(options ...string)"),
  );
  const config = path.join(root, "seamgraph.yaml");
  fs.writeFileSync(
    config,
    fs.readFileSync(config, "utf8").replaceAll("reader", "public"),
  );
  fs.writeFileSync(
    path.join(root, "npc/bad.go"),
    'package npc\nimport w "example.test/architecture/world"\nvar Forbidden w.Concrete\n',
  );
  git(["add", "."]);
  git([
    "-c",
    "user.name=Test",
    "-c",
    "user.email=test@example.invalid",
    "commit",
    "-qm",
    "architecture changes",
  ]);
  const before = execFileSync(
    path.join(repo, ".build/seamgraph"),
    [
      "inspect",
      "seam:world.reader",
      "--revision",
      "HEAD~1",
      "--root",
      root,
      "--json",
    ],
    { encoding: "utf8" },
  );
  expect(JSON.parse(before).entity.id).toBe("seam:world.reader");
  await page.goto(url);
  await expect(page.locator("#summary")).toContainText("2 modules");
  await page.getByRole("button", { name: "Compare", exact: true }).click();
  await expect(page.locator("#changes")).toContainText("added violation");
  await expect(page.locator("#changes")).toContainText("removed seam");
  await expect(page.locator("#changes")).toContainText("modified symbol");
  await expect
    .poll(() =>
      page.evaluate(
        () => document.querySelector("#graph")._cyreg.cy.edges(".added").length,
      ),
    )
    .toBeGreaterThan(0);
});


test("analysis errors appear inside the graph instead of an empty frame", async () => {
  await page.route("**/api/model", route => route.fulfill({
    status: 500, contentType: "text/plain",
    body: "open /example/seamgraph.yaml: no such file or directory",
  }));
  try {
    await page.goto(url);
    await expect(page.locator("#graph [role=alert]")).toContainText("Could not load the architecture");
    await expect(page.locator("#graph")).toContainText("/example/seamgraph.yaml");
  } finally { await page.unroute("**/api/model"); }
});

test("a missing renderer explains the failure inside the graph", async () => {
  await page.route("**/assets/cytoscape.min.js", route => route.abort());
  try {
    await page.goto(url);
    await expect(page.locator("#graph [role=alert]")).toContainText("graph renderer failed to load");
  } finally { await page.unroute("**/assets/cytoscape.min.js"); }
});

test("explorer fills desktop and mobile viewports and refits after resize", async () => {
  try {
    await page.goto(url);
    await page.waitForFunction(() => document.querySelector("#graph")._cyreg?.cy?.nodes().length === 2);
    for (const viewport of [{width:1920,height:1080}, {width:1024,height:768}, {width:390,height:844}]) {
      await page.setViewportSize(viewport);
      await expect.poll(() => page.evaluate(() => {
        const graph = document.querySelector("#graph"), rect = graph.getBoundingClientRect();
        const cy = graph._cyreg.cy;
        return Math.abs(cy.width() - rect.width) < 2 && Math.abs(cy.height() - rect.height) < 2;
      })).toBe(true);
      const geometry = await page.evaluate(() => {
        const graph = document.querySelector("#graph").getBoundingClientRect();
        const details = document.querySelector("#details").getBoundingClientRect();
        return {graph:graph.toJSON(),details:details.toJSON(),width:document.documentElement.scrollWidth,height:document.documentElement.scrollHeight};
      });
      expect(geometry.width).toBeLessThanOrEqual(viewport.width);
      expect(geometry.height).toBeLessThanOrEqual(viewport.height);
      expect(geometry.graph.height).toBeGreaterThan(200);
      expect(geometry.details.bottom).toBeLessThan(viewport.height);
      if (viewport.width > 767) {
        expect(geometry.graph.width).toBeGreaterThan(viewport.width * .5);
        expect(geometry.details.x).toBeGreaterThan(geometry.graph.x);
      } else {
        expect(geometry.details.y).toBeGreaterThan(geometry.graph.y);
      }
      await page.screenshot({path:path.join(repo, `.build/explorer-${viewport.width}.png`)});
    }
  } finally { await page.setViewportSize({width:1440,height:1000}); }
});
