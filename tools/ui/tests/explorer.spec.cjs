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
  server?.kill();
  try {
    // Sparticuz Chromium can hang on browser.close() while pages remain open.
    // Close every page explicitly, then its context, before stopping Chromium.
    for (const browserContext of browser?.contexts() ?? []) {
      for (const browserPage of browserContext.pages()) {
        await browserPage.close();
      }
      await browserContext.close();
    }
  } finally {
    try {
      await browser?.close();
    } finally {
      if (root) fs.rmSync(root, { recursive: true, force: true });
    }
  }
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
  await expect(page.getByRole("link", { name: "Help", exact:true })).toHaveAttribute("href","/help");
  await page.evaluate(() => { document.querySelector("#graph")._cyreg.cy.edges().first().emit("tap"); });
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
  await page.evaluate(() => { document.querySelector("#graph")._cyreg.cy.edges().first().emit("tap"); });
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
    await page.evaluate(() => { document
        .querySelector("#graph")
        ._cyreg.cy.getElementById("module:world")
        .emit("tap"); });
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
  await page.getByLabel("Comparison scope").selectOption("revisions");
  await page.getByRole("button", { name: "Compare", exact: true }).click();
  await expect(page.locator("#changes")).toContainText("added violation");
  await expect(page.locator("#changes")).toContainText("removed seam");
  await expect(page.locator("#changes")).toContainText("modified symbol");
  await expect
    .poll(() =>
      page.evaluate(
        () => document.querySelector("#graph")._cyreg.cy.edges(".changed").length,
      ),
    )
    .toBeGreaterThan(0);
  await expect.poll(()=>page.evaluate(()=>{
    const cy=document.querySelector('#graph')._cyreg.cy,world=cy.getElementById('module:world');
    return world.data('label').includes('Seam changed') && world.height() >= world.data('label').split('\n').length*14;
  })).toBe(true);
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

test("dependency layers, reference views, seam edges and focus preserve the full graph", async () => {
  const snapshot = JSON.parse(fs.readFileSync(path.join(repo, "testdata/ir/go.json")));
  snapshot.modules.push({id:"module:bootstrap",displayName:"bootstrap",sourceRoots:["bootstrap"],seams:["seam:bootstrap.construction"]},
    {id:"module:isolated",displayName:"isolated",sourceRoots:["isolated"],seams:[]});
  snapshot.symbols.push({id:"symbol:bootstrap:New",moduleId:"module:bootstrap",kind:"function",displaySignature:"New()",visibility:"public"},
    {id:"symbol:world:Value",moduleId:"module:world",kind:"field",displaySignature:"Value string",visibility:"public"});
  snapshot.seams.push({id:"seam:bootstrap.construction",moduleId:"module:bootstrap",contractMembers:["symbol:bootstrap:New"]});
  const evidence = snapshot.relations[0].evidence;
  snapshot.relations.push({id:"data",fromId:"module:npc",toId:"symbol:world:Value",kind:"uses",evidence},
    {id:"construction",fromId:"module:npc",toId:"symbol:bootstrap:New",kind:"uses",evidence},
    {id:"wiring",fromId:"module:bootstrap",toId:snapshot.symbols[0].id,kind:"uses",evidence},
    {id:"import",fromId:"module:npc",toId:"module:world",kind:"imports",evidence});
  await page.route("**/api/model", route => route.fulfill({json:{snapshot,check:{violations:[],incomplete:false}}}));
  // Keep this projection fixture self-contained: earlier editing tests change
  // the real project, and this test exercises graph focus rather than scanning.
  const moduleRoute = /\/module\?/;
  await page.route(moduleRoute, route => {
    const id = new URL(route.request().url()).searchParams.get("id");
    return route.fulfill({status:id==="module:world"?200:404,contentType:"text/html",
      body:'<code>module:world</code><button data-expand-module="module:world">Expand seams in graph</button>'});
  });
  try {
    await page.goto(url);
    await expect(page.locator("#summary")).toContainText("4 modules");
    const visibleNodes = () => page.evaluate(() => document.querySelector("#graph")._cyreg.cy.nodes(":visible").map(n => n.id()).sort());
    await expect.poll(visibleNodes).toEqual(["module:npc","module:world"]);
    expect(await page.evaluate(() => {
      const cy = document.querySelector("#graph")._cyreg.cy;
      return cy.getElementById("module:npc").position("x") < cy.getElementById("module:world").position("x");
    })).toBe(true);
    await page.locator("#dependency-view").selectOption("construction");
    await expect.poll(visibleNodes).toEqual(["module:bootstrap","module:npc","module:world"]);
    await page.locator("#dependency-view").selectOption("all");
    await expect.poll(visibleNodes).toHaveLength(4);
    await page.evaluate(() => { document.querySelector("#graph")._cyreg.cy.getElementById("module:world").emit("tap"); });
    await expect.poll(visibleNodes).toHaveLength(3);
    await page.locator('[data-expand-module="module:world"]').click();
    await expect.poll(() => page.evaluate(() => document.querySelector("#graph")._cyreg.cy.edges().some(e => e.data("target") === "seam:world.reader"))).toBe(true);
    await expect(page.locator(".htmx-request")).toHaveCount(0);
    const detailsResponse = page.waitForResponse(response => {
      const request = new URL(response.url());
      return request.pathname === "/module" && request.searchParams.get("id") === "module:world";
    });
    await page.evaluate(() => { document.querySelector("#graph")._cyreg.cy.getElementById("seam:world.reader").emit("tap"); });
    await detailsResponse;
    await expect(page.locator("#details")).toContainText("module:world");
    await expect.poll(visibleNodes).toEqual(["module:bootstrap","module:npc","module:world","seam:world.reader"]);
    await page.locator("#reset-focus").click();
    await expect.poll(visibleNodes).toHaveLength(5);
    await page.locator("#dependency-view").selectOption("data");
    await expect.poll(() => page.evaluate(() => document.querySelector("#graph")._cyreg.cy.edges(":visible").map(e => e.id()))).toEqual(["module:npc→module:world"]);
  } finally {
    if (!page.isClosed()) {
      await page.unroute("**/api/model");
      await page.unroute(moduleRoute);
    }
  }
});

test("hover highlights incoming and outgoing arrows and restores their styles on leave", async () => {
  const snapshot = JSON.parse(fs.readFileSync(path.join(repo, "testdata/ir/go.json")));
  snapshot.modules.push({id:"module:extra",displayName:"extra",sourceRoots:["extra"],seams:[]},
    {id:"module:other",displayName:"other",sourceRoots:["other"],seams:[]});
  const evidence = snapshot.relations[0].evidence;
  snapshot.relations.push({id:"outgoing",fromId:"module:world",toId:"module:extra",kind:"imports",evidence},
    {id:"unrelated",fromId:"module:extra",toId:"module:other",kind:"imports",evidence});
  await page.route("**/api/model", route => route.fulfill({json:{snapshot,check:{violations:[{relationId:snapshot.relations[0].id}],incomplete:false}}}));
  try {
    await page.goto(url);
    await expect(page.locator("#summary")).toContainText("4 modules");
    await page.locator("#dependency-view").selectOption("all");
    const position = await page.evaluate(() => {
      const graph = document.querySelector("#graph"), cy = graph._cyreg.cy;
      const p = cy.getElementById("module:world").renderedPosition(), rect = graph.getBoundingClientRect();
      return {x:rect.x+p.x,y:rect.y+p.y};
    });
    await page.mouse.move(position.x,position.y);
    await expect.poll(() => page.evaluate(() => {
      const cy = document.querySelector("#graph")._cyreg.cy;
      return cy.edges(".hover-connected").map(e => e.id()).sort();
    })).toEqual(["module:npc→module:world","module:world→module:extra"]);
    expect(await page.evaluate(() => {
      const cy = document.querySelector("#graph")._cyreg.cy;
      return {
        width:cy.getElementById("module:world→module:extra").style("width"),
        color:cy.getElementById("module:world→module:extra").style("target-arrow-color"),
        violation:cy.getElementById("module:npc→module:world").style("target-arrow-color"),
        unrelated:cy.getElementById("module:extra→module:other").style("width"),
      };
    })).toEqual({width:"3.5px",color:"rgb(103,232,249)",violation:"rgb(251,113,133)",unrelated:"1.5px"});
    await page.mouse.move(0,0);
    await expect.poll(() => page.evaluate(() => document.querySelector("#graph")._cyreg.cy.edges(".hover-connected").length)).toBe(0);
    expect(await page.evaluate(() => document.querySelector("#graph")._cyreg.cy.getElementById("module:world→module:extra").style("width"))).toBe("1.5px");
    const edgePoint=await page.evaluate(()=>{
      const graph=document.querySelector('#graph'),edge=graph._cyreg.cy.getElementById('module:world→module:extra');
      const points=[edge.renderedSourceEndpoint(),...(edge.renderedSegmentPoints()||[]),edge.renderedTargetEndpoint()];
      const segments=points.slice(1).map((b,i)=>({a:points[i],b,length:Math.hypot(b.x-points[i].x,b.y-points[i].y)})).sort((a,b)=>b.length-a.length);
      const {a,b}=segments[0],rect=graph.getBoundingClientRect();
      return{x:rect.x+(a.x+b.x)/2,y:rect.y+(a.y+b.y)/2};
    });
    await page.mouse.move(edgePoint.x,edgePoint.y);
    await expect.poll(()=>page.evaluate(()=>document.querySelector('#graph')._cyreg.cy.edges('.hover-connected').map(e=>e.id()))).toEqual(['module:world→module:extra']);
    expect(await page.evaluate(()=>document.querySelector('#graph')._cyreg.cy.getElementById('module:world→module:extra').style('width'))).toBe('3.5px');
    await page.mouse.move(0,0);
    await expect.poll(()=>page.evaluate(()=>document.querySelector('#graph')._cyreg.cy.edges('.hover-connected').length)).toBe(0);
  } finally {
    await page.unroute("**/api/model");
  }
});

test("neighbor ordering removes crossings and stays stable while hovering or refitting", async () => {
  const modules = ['a','b','c','d'].map(name => ({id:'module:'+name,displayName:name,sourceRoots:[name],seams:[]}));
  const symbols = ['c','d'].map(name => ({id:'symbol:'+name+':Run',moduleId:'module:'+name,kind:'function',displaySignature:'Run()',visibility:'public'}));
  const relations = [['a','d'],['b','c']].map(([from,to]) => ({id:from+'→'+to,fromId:'module:'+from,toId:'symbol:'+to+':Run',kind:'uses',evidence:[]}));
  const snapshot = {schemaVersion:1,modules,seams:[],symbols,relations,diagnostics:[]};
  await page.route('**/api/model',route => route.fulfill({json:{snapshot,check:{violations:[],incomplete:false}}}));
  try {
    await page.goto(url);
    await expect(page.locator('#summary')).toContainText('4 modules');
    const coordinates = () => page.evaluate(() => document.querySelector('#graph')._cyreg.cy.nodes().map(n => ({id:n.id(),...n.position()})));
    const initial = await coordinates(), byId = Object.fromEntries(initial.map(p => [p.id,p]));
    expect(byId['module:a'].x).toBeLessThan(byId['module:d'].x);
    expect((byId['module:a'].y-byId['module:b'].y)*(byId['module:d'].y-byId['module:c'].y)).toBeGreaterThan(0);
    await page.evaluate(() => { document.querySelector('#graph')._cyreg.cy.getElementById('module:a').emit('mouseover'); });
    expect(await coordinates()).toEqual(initial);
    await page.locator('#dependency-view').selectOption('all');
    expect(await coordinates()).toEqual(initial);
    await page.getByRole('link',{name:'Architecture',exact:true}).click();
    expect(await coordinates()).toEqual(initial);
  } finally { await page.unroute('**/api/model'); }
});

test("orthogonal arrows avoid module boxes, remain clickable, and follow dragged nodes", async () => {
  const snapshot=JSON.parse(fs.readFileSync(path.join(repo,'testdata/ir/go.json')));
  snapshot.modules.push({id:'module:middle',displayName:'middle',sourceRoots:['middle'],seams:[]},
    {id:'module:bootstrap',displayName:'bootstrap',sourceRoots:['bootstrap'],seams:[]});
  snapshot.relations.push({id:'bootstrap-middle',fromId:'module:bootstrap',toId:'module:middle',kind:'imports',evidence:[]},
    {id:'bootstrap-world',fromId:'module:bootstrap',toId:'module:world',kind:'imports',evidence:[]});
  snapshot.symbols.push({id:'symbol:middle:Run',moduleId:'module:middle',kind:'function',displaySignature:'Run()',visibility:'public'});
  snapshot.relations.push({id:'npc-middle',fromId:'module:npc',toId:'symbol:middle:Run',kind:'uses',evidence:[]},
    {id:'middle-world',fromId:'module:middle',toId:snapshot.symbols[0].id,kind:'uses',evidence:[]});
  await page.route('**/api/model',route=>route.fulfill({json:{snapshot,check:{violations:[],incomplete:false}}}));
  const geometry=()=>page.evaluate(()=>{
    const cy=document.querySelector('#graph')._cyreg.cy;
    let diagonal=0,intersections=0;
    for(const edge of cy.edges(':visible')){
      const points=[edge.sourceEndpoint(),...(edge.segmentPoints()||[]),edge.targetEndpoint()];
      for(let i=1;i<points.length;i++){
        const a=points[i-1],b=points[i];
        if(Math.abs(a.x-b.x)>1e-5&&Math.abs(a.y-b.y)>1e-5)diagonal++;
        for(const node of cy.nodes(':visible')){
          if(node.id()===edge.data('source')||node.id()===edge.data('target'))continue;
          const box=node.boundingBox({includeLabels:false});
          if(Math.abs(a.y-b.y)<1e-5&&a.y>box.y1&&a.y<box.y2&&Math.max(Math.min(a.x,b.x),box.x1)<Math.min(Math.max(a.x,b.x),box.x2))intersections++;
          if(Math.abs(a.x-b.x)<1e-5&&a.x>box.x1&&a.x<box.x2&&Math.max(Math.min(a.y,b.y),box.y1)<Math.min(Math.max(a.y,b.y),box.y2))intersections++;
        }
      }
    }
    return{diagonal,intersections};
  });
  try{
    await page.goto(url);
    await expect(page.locator('#summary')).toContainText('4 modules');
    await expect.poll(geometry).toEqual({diagonal:0,intersections:0});
    const click=await page.evaluate(()=>{
      const graph=document.querySelector('#graph'),edge=graph._cyreg.cy.getElementById('module:npc→module:world');
      const points=[edge.renderedSourceEndpoint(),...edge.renderedSegmentPoints(),edge.renderedTargetEndpoint()];
      const segments=points.slice(1).map((b,i)=>({a:points[i],b,length:Math.hypot(b.x-points[i].x,b.y-points[i].y)})).sort((a,b)=>b.length-a.length);
      const {a,b}=segments[0],box=graph.getBoundingClientRect();
      return{x:box.x+(a.x+b.x)/2,y:box.y+(a.y+b.y)/2};
    });
    const response=page.waitForResponse(r=>{const u=new URL(r.url());return u.pathname==='/edge'&&u.searchParams.get('from')==='module:npc'&&u.searchParams.get('to')==='module:world';});
    await page.mouse.click(click.x,click.y);
    await response;
    await expect(page.locator('#details')).toContainText('Observed relationship');
    await page.evaluate(()=>{const n=document.querySelector('#graph')._cyreg.cy.getElementById('module:npc');n.position({x:n.position('x'),y:n.position('y')+30});n.emit('drag');});
    await expect.poll(geometry).toEqual({diagonal:0,intersections:0});
    // Construction/import-only nodes start hidden. Showing them must apply
    // custom routes immediately, rather than leaving default taxi geometry.
    await page.locator('#dependency-view').selectOption('all');
    await expect.poll(()=>page.evaluate(()=>{
      const cy=document.querySelector('#graph')._cyreg.cy;
      return cy.getElementById('module:bootstrap').connectedEdges().map(e=>e.style('curve-style'));
    })).toEqual(['segments','segments']);
    await expect.poll(geometry).toEqual({diagonal:0,intersections:0});
  }finally{await page.unroute('**/api/model');}
});

test('working changes show internal edits and retain removed modules for inspection',async()=>{
  fs.rmSync(root,{recursive:true,force:true});
  fs.cpSync(path.join(repo,'testdata/projects/go'),root,{recursive:true});
  const git=args=>execFileSync('git',['-C',root,...args]);
  git(['init','-q']);git(['add','.']);git(['-c','user.name=Test','-c','user.email=test@example.invalid','commit','-qm','base']);
  const world=path.join(root,'world/world.go');
  fs.appendFileSync(world,'\n// internal-only edit\n');
  fs.writeFileSync(path.join(root,'npc/private.go'),'package npc\nvar localValue = 7\n');
  git(['add','npc/private.go']);
  fs.writeFileSync(path.join(root,'world/untracked.go'),'package world\nvar localValue = 8\n');
  await page.goto(url);
  await expect(page.locator('#summary')).toContainText('2 modules');
  await expect(page.getByLabel('Comparison scope')).toHaveValue('working');
  await page.getByRole('button',{name:'Compare',exact:true}).click();
  await expect(page.locator('#changes')).toContainText('world/world.go');
  await expect(page.locator('#changes')).toContainText('npc/private.go');
  await expect(page.locator('#changes')).toContainText('world/untracked.go');
  const graph=()=>page.evaluate(()=>{
    const cy=document.querySelector('#graph')._cyreg.cy;
    return {changed:cy.nodes('.changed').map(n=>n.id()).sort(),seamBadges:cy.nodes().filter(n=>n.data('label').includes('Seam changed')).length,changedEdges:cy.edges('.changed, .added, .removed').length};
  });
  await expect.poll(graph).toEqual({changed:['module:npc','module:world'],seamBadges:0,changedEdges:0});
  await page.evaluate(() => { document.querySelector('#graph')._cyreg.cy.getElementById('module:world').emit('tap'); });
  await expect(page.locator('#details')).toContainText('world/world.go');
  await page.getByLabel('Changed + neighbors').check();
  await expect(page.locator('#details')).not.toContainText('Seam changes');
  await page.screenshot({path:path.join(repo,'.build/comparison-preview.png')});
  // Remove a module and its configuration, keeping a valid project.
  fs.rmSync(path.join(root,'npc'),{recursive:true});
  fs.writeFileSync(path.join(root,'seamgraph.yaml'),'version: 1\nmodules:\n  world:\n    language: go\n    paths: [world]\n    seams:\n      reader:\n        symbols: [Reader, Open]\npolicies:\n  default: deny\n');
  await page.getByRole('button',{name:'Compare',exact:true}).click();
  await expect(page.locator('#changes')).toContainText('removed module');
  await expect.poll(()=>page.evaluate(()=>{
    const cy=document.querySelector('#graph')._cyreg.cy;
    return {removed:cy.getElementById('module:npc').hasClass('removed'),edges:cy.edges('.removed').length};
  })).toEqual({removed:true,edges:1});
  await page.evaluate(() => { document.querySelector('#graph')._cyreg.cy.getElementById('module:npc').emit('tap'); });
  await expect(page.locator('#details')).toContainText('removed');
  await expect(page.locator('#details')).toContainText('npc/npc.go');
  await page.evaluate(() => { document.querySelector('#graph')._cyreg.cy.edges('.removed')[0].emit('tap'); });
  await expect(page.locator('#details')).toContainText('Connection changes');
  await expect(page.locator('#details')).toContainText('removed');
  await expect(page.locator('#status')).not.toContainText('unknown');
  await page.screenshot({path:path.join(repo,'.build/comparison-removed-preview.png')});
  await page.setViewportSize({width:390,height:844});
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await expect(page.getByRole('button',{name:'Compare',exact:true})).toBeVisible();
});


test('Help is a standalone responsive page available when project analysis fails',async()=>{
  await page.route('**/api/model',route=>route.fulfill({status:500,body:'Project analysis unavailable'}));
  try {
    await page.goto(url);
    await expect(page.getByRole('alert')).toContainText('Project analysis unavailable');
    await expect(page.getByRole('button',{name:'About evidence'})).toHaveCount(0);
    await page.getByRole('link',{name:'Help',exact:true}).click();
    await expect(page).toHaveURL(url+'/help');
    await expect(page.getByRole('heading',{name:'Using SeamGraph'})).toBeVisible();
    await expect(page.getByRole('link',{name:'Help',exact:true})).toHaveAttribute('aria-current','page');
    await expect(page.locator('script')).toHaveCount(0);
    await page.getByRole('link',{name:'Reference modes',exact:true}).click();
    await expect(page.getByRole('heading',{name:'Reference modes',exact:true})).toBeInViewport();
    await expect(page.locator('tbody tr')).toHaveCount(5);
    await page.setViewportSize({width:390,height:844});
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await page.getByRole('heading',{name:'Understanding evidence'}).scrollIntoViewIfNeeded();
    await expect(page.getByText('Edges show observed dependencies.',{exact:false})).toBeVisible();
    await page.getByRole('link',{name:'Architecture',exact:true}).click();
    await expect(page).toHaveURL(url+'/');
    await expect(page.getByRole('alert')).toBeVisible();
  } finally { await page.unroute('**/api/model'); }
});

test('Rules opens its own responsive page with navigation to Help and Architecture',async()=>{
  await page.goto(url);
  await expect(page.locator('#summary')).toContainText('modules');
  await page.getByRole('link',{name:'Rules',exact:true}).click();
  await expect(page).toHaveURL(url+'/rules');
  await expect(page).toHaveTitle('SeamGraph · Rules');
  await expect(page.getByRole('heading',{name:'Architecture rules'})).toBeVisible();
  await expect(page.getByRole('link',{name:'Rules',exact:true})).toHaveAttribute('aria-current','page');
  await expect(page.locator('#details')).toHaveCount(0);
  await expect(page.locator('script')).toHaveCount(0);
  await expect(page.getByText('Default:',{exact:false})).toBeVisible();
  await page.setViewportSize({width:390,height:844});
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.getByRole('link',{name:'Help',exact:true}).click();
  await expect(page).toHaveURL(url+'/help');
  await page.getByRole('link',{name:'Rules',exact:true}).click();
  await expect(page.getByRole('heading',{name:'Architecture rules'})).toBeVisible();
  await page.getByRole('link',{name:'Architecture',exact:true}).click();
  await expect(page).toHaveURL(url+'/');
  await expect(page.locator('#graph')).toBeVisible();
});

test('Architecture, Rules and Help share the same responsive header and navigation',async()=>{
  for(const viewport of [{width:1440,height:1000},{width:390,height:844}]){
    await page.setViewportSize(viewport);
    let expected;
    for(const [path,title] of [['/','Architecture'],['/rules','Rules'],['/help','Help']]){
      await page.goto(url+path);
      if(path==='/')await expect(page.locator('#summary')).toContainText('modules');
      const nav=page.getByRole('navigation',{name:'Main navigation',exact:true});
      await expect(nav.locator('[aria-current="page"]')).toHaveText(title);
      const chrome=await page.evaluate(()=>{
        const header=document.querySelector('.app-header'),nav=document.querySelector('.app-navigation');
        return {inHeader:header.contains(nav),brand:header.querySelector('div').textContent,headerHeight:header.getBoundingClientRect().height,
          navTop:nav.getBoundingClientRect().top,navHeight:nav.getBoundingClientRect().height,
          links:[...nav.querySelectorAll('a')].map(a=>[a.textContent,a.getAttribute('href')])};
      });
      expect(chrome.inHeader).toBe(true);
      expected??=chrome;
      expect(chrome).toEqual(expected);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    }
  }
});

test('details show loading feedback, replace slow selections, and clear loading on errors',async()=>{
  const snapshot=JSON.parse(fs.readFileSync(path.join(repo,'testdata/ir/go.json')));
  await page.route('**/api/model',route=>route.fulfill({json:{snapshot,check:{violations:[],incomplete:false}}}));
  let release;
  const gate=()=>new Promise(resolve=>{release=resolve;});
  let pending=gate(),mode='delayed';
  const moduleRoute=/\/module\?/;
  await page.route(moduleRoute,async route=>{
    const id=new URL(route.request().url()).searchParams.get('id');
    if(mode==='error'){await pending;await route.fulfill({status:500,body:'Details could not be analyzed'});return;}
    if(id==='module:world')await pending;
    await route.fulfill({contentType:'text/html',body:`<p>${id} loaded</p><button data-expand-module="module:world">Expand seams in graph</button>`}).catch(()=>{});
  });
  const tap=id=>page.evaluate(id=>{document.querySelector('#graph')._cyreg.cy.getElementById(id).emit('tap');},id);
  try {
    await page.goto(url);
    await expect(page.locator('#summary')).toContainText('2 modules');
    await tap('module:world');
    await expect(page.locator('#details-loading')).toBeVisible();
    await expect(page.locator('#details')).toHaveAttribute('aria-busy','true');
    release();
    await expect(page.locator('#details')).toContainText('module:world loaded');
    await expect(page.locator('#details-loading')).toBeHidden();
    await expect(page.locator('#details')).toHaveAttribute('aria-busy','false');
    // A seam selection uses its owning module's request and the same indicator.
    await page.locator('[data-expand-module="module:world"]').click();
    pending=gate();
    await tap('seam:world.reader');
    await expect(page.locator('#details-loading')).toBeVisible();
    await tap('module:npc');
    await expect(page.locator('#details')).toContainText('module:npc loaded');
    release();
    await expect(page.locator('#details-loading')).toBeHidden();
    await expect(page.locator('#details')).not.toContainText('module:world loaded');
    mode='error';pending=gate();
    await tap('module:world');
    await expect(page.locator('#details-loading')).toBeVisible();
    release();
    await expect(page.locator('#details [role="alert"]')).toHaveText('Details could not be analyzed');
    await expect(page.locator('#details-loading')).toBeHidden();
    await expect(page.locator('#details')).toHaveAttribute('aria-busy','false');
  } finally {
    release();if(!page.isClosed()){await page.unroute('**/api/model');await page.unroute(moduleRoute);}
  }
});

test('generated fulfillment repository supports exploration, comparison and a bounded larger graph', async () => {
  test.setTimeout(60000);
  fs.rmSync(root, {recursive:true, force:true});
  execFileSync('python3', [path.join(repo,'scripts/examples/generate.py'), root, '--warehouses','120'], {cwd:repo});
  await page.setViewportSize({width:1440,height:1000});
  const errors=[];
  const recordError = error => errors.push(String(error));
  page.on('pageerror', recordError);
  try {
    const started=Date.now();
    await page.goto(url);
    await expect(page.locator('#summary')).toContainText('138 modules');
    await page.waitForFunction(()=>document.querySelector('#graph')._cyreg?.cy?.nodes().length===138);
    expect(Date.now()-started).toBeLessThan(10000);
    await page.getByLabel('Dependency view').selectOption('all');
    await page.locator('#search').fill('warehouse-042');
    await expect.poll(()=>page.evaluate(()=>document.querySelector('#graph')._cyreg.cy.nodes(':visible').length)).toBe(1);
    await page.locator('#search').fill('');
    await page.evaluate(()=>document.querySelector('#graph')._cyreg.cy.getElementById('module:orders').emit('tap'));
    await expect(page.locator('#details')).toContainText('seam:orders.lifecycle');
    const source=page.locator('#details a[href^="/source?"]').first();
    await expect(source).toBeVisible();
    const response=await page.request.get(new URL(await source.getAttribute('href'),url).href);
    expect(response.ok()).toBe(true);
    expect(await response.text()).toContain('package');
    await page.getByLabel('Comparison scope').selectOption('revisions');
    await page.getByLabel('Base revision').fill('main');
    await page.getByLabel('Head revision').fill('review/boundary');
    await page.getByRole('button',{name:'Compare',exact:true}).click();
    await expect(page.locator('#changes')).toContainText('added violation',{timeout:15000});
    const refs=JSON.parse(fs.readFileSync(path.join(root,'.seamgraph/scenarios.json'))).refs;
    await expect(page.locator('#changes')).toContainText(refs['review/boundary']);
    await page.getByLabel('Base revision').fill('review/boundary');
    await page.getByLabel('Head revision').fill('review/boundary-fixed');
    await page.getByRole('button',{name:'Compare',exact:true}).click();
    await expect(page.locator('#changes')).toContainText('removed violation',{timeout:15000});
    expect(execFileSync('git',['-C',root,'status','--porcelain'],{encoding:'utf8'})).toBe('');
    await page.screenshot({path:path.join(repo,'.build/order-review-large.png'),fullPage:true});
    expect(errors).toEqual([]);
  } finally {
    page.off('pageerror', recordError);
  }
});

test('fulfillment review connects directories, hotspots, impact, durable findings and evidence exports', async () => {
  test.setTimeout(180000);
  fs.rmSync(root,{recursive:true,force:true});
  execFileSync('python3',[path.join(repo,'scripts/examples/generate.py'),root],{cwd:repo});
  const reviewPath=(extra={})=>'/review?'+new URLSearchParams({base:'target',head:'review/history',...extra});
  const checkout='storefront/checkout/checkout.ts';
  await page.setViewportSize({width:1440,height:1000});
  await page.goto(url+reviewPath());
  await expect(page.getByRole('heading',{name:'Local branch review',exact:true})).toBeVisible();
  await expect(page.getByLabel('Directory explorer')).toContainText('storefront');
  const hotspot=page.locator(`[data-hotspot-path="${checkout}"]`);
  await expect(hotspot.locator('td')).toHaveText([checkout,'3','3','3','6','+0 / −0']);
  await expect(page.locator('[data-hotspot-path="unassigned/logo.bin"]')).toContainText('Unknown (binary)');
  await page.getByRole('navigation',{name:'Commit type filters'}).getByRole('link',{name:'assets (1 commits)',exact:true}).click();
  await expect(page.locator('[data-hotspot-path]')).toHaveCount(1);
  await expect(page.locator('[data-review-commit]')).toHaveCount(1);
  await page.getByRole('link',{name:'All commit types',exact:true}).click();
  await page.locator(`[data-hotspot-path="${checkout}"] a`).click();
  await expect(page.locator('[data-review-before]')).toContainText('Place order');
  await expect(page.locator('[data-review-after]')).toContainText('Place order');
  await expect(page.locator('[data-review-commit]')).toHaveCount(3);
  await page.locator('[data-review-commit] summary').first().click();
  await expect(page.locator('[data-commit-patch]').first()).toContainText('Confirm order');
  await expect(page.locator('[data-net-patch]')).toBeEmpty();
  const oracle=JSON.parse(fs.readFileSync(path.join(root,'.seamgraph/scenarios.json'))).history;
  const facts=await (await page.request.get(url+reviewPath({export:'json'}))).json();
  const groups={};
  for(const commit of facts.activity.commits)groups[commit.type]=(groups[commit.type]||0)+1;
  expect(groups).toEqual(oracle.groups);
  expect(facts.activity.commits.filter(commit=>commit.breaking)).toHaveLength(2);
  for(const hotspot of facts.hotspots){
    const expected=oracle.files[hotspot.path];
    expect(hotspot.touches).toBe(expected.touches);
    if(!hotspot.binary){expect(hotspot.churn).toBe(expected.churn);}
  }
  const originalHead=await page.locator('[data-review-head]').textContent();
  await page.getByLabel('Review note',{exact:true}).fill('Check retry behavior <script>alert(1)</script>');
  await page.getByLabel('Viewed',{exact:true}).check();
  await page.getByRole('button',{name:'Save file review',exact:true}).click();
  await expect(page.locator('[data-review-mark]')).toContainText('viewed: true');
  const oldForm=await page.locator('.review-note').evaluate(form=>Object.fromEntries(new FormData(form)));
  const git=args=>execFileSync('git',['-C',root,...args],{encoding:'utf8'});
  git(['switch','-q','review/history']);
  const sourceFile=path.join(root,checkout);
  fs.writeFileSync(sourceFile,fs.readFileSync(sourceFile,'utf8').replace('Place order','Next order'));
  git(['add',checkout]);
  git(['-c','user.name=Test','-c','user.email=test@example.invalid','commit','-qm','fix(checkout): clarify the next action']);
  const staleSave=await page.request.post(url+'/review',{form:oldForm});
  expect(staleSave.status()).toBe(409);
  expect(await staleSave.text()).toContain('comparison changed');
  await page.reload();
  await expect(page.locator('[data-review-mark]')).toContainText('needs reconsideration');
  await expect(page.locator('[data-review-after]')).toContainText('Next order');
  // Restarting the server proves that review state is held by the repository,
  // rather than a browser session or an in-memory application cache.
  const exited=new Promise(resolve=>server.once('exit',resolve));
  server.kill();await exited;
  server=spawn(path.join(repo,'.build/seamgraph'),['serve','--root',root,'--addr','127.0.0.1:18765'],{cwd:repo,stdio:'pipe'});
  for(let i=0;i<100;i++){
    try{if((await fetch(url)).ok)break;}catch{}
    await new Promise(resolve=>setTimeout(resolve,50));
  }
  await page.goto(url+reviewPath({file:checkout}));
  const mark=page.locator('[data-review-mark]');
  await expect(mark).toContainText('needs reconsideration');
  await expect(mark).toContainText(originalHead);
  await expect(mark).toContainText('Check retry behavior <script>alert(1)</script>');
  const response=await page.request.get(url+reviewPath({file:checkout,export:'json'}));
  const exported=await response.json();
  expect(exported.marks[0].entry.head).toBe(originalHead);
  expect(exported.marks[0].entry.context).toContain('Place order');
  expect(exported.after.text).toContain('Next order');
  expect(exported.comparison.after.relations.some(rel=>rel.evidence.length>0)).toBe(true);
  expect(exported.activity.commits).toHaveLength(9);
  for(const format of ['html','markdown']){
    const download=await page.request.get(url+reviewPath({file:checkout,export:format}));
    expect(download.headers()['content-disposition']).toContain('attachment');
    const text=await download.text();
    expect(text).toContain(originalHead);
    expect(text).toContain('Check retry behavior');
    expect(text).not.toContain('<script>');
    expect(text).not.toContain('/assets/');
  }
  await page.goto(url+reviewPath({object:'symbol:orders:orders.Service.Place'}));
  await expect(page.locator('[data-review-object]')).toContainText('module:demo');
  await expect(page.locator('[data-review-object]')).toContainText('module:http');
  await expect(page.locator('[data-review-object]')).toContainText('Possible transitive module impact');
  await page.goto(url+reviewPath({file:'unassigned/status.txt'}));
  await expect(page.getByLabel('Review details')).toContainText('File absent at this revision');
  await expect(page.locator('[data-review-before]')).toContainText('architectur');
  await page.goto(url+reviewPath({file:'unassigned/logo.bin'}));
  await expect(page.getByLabel('Review details')).toContainText('Binary file; text diff unavailable');
  await page.setViewportSize({width:390,height:844});
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:path.join(repo,'.build/fulfillment-review-mobile.png'),fullPage:true});
});
