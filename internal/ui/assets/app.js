/* All assets and data remain local. */
document.addEventListener("alpine:init", () => {
  Alpine.data("shell", () => ({
    help: false,
    toggle() {
      this.help = !this.help;
    },
  }));
});
document.addEventListener("DOMContentLoaded", async () => {
  const status = document.querySelector("#status");
  document.addEventListener("htmx:responseError", (event) => {
    status.textContent = event.detail.xhr.responseText || "Request failed.";
  });
  document.addEventListener("click", async (event) => {
    const copy = event.target.closest("[data-copy]");
    if (copy) {
      try {
        await navigator.clipboard.writeText(copy.dataset.copy);
        status.textContent = "Copied to clipboard.";
      } catch {
        status.textContent =
          "Clipboard unavailable. Select and copy the visible ID.";
      }
    }
    const cancel = event.target.closest("[data-cancel]");
    if (cancel) {
      const form = cancel.closest("form");
      htmx.ajax("POST", "/rules/cancel", {
        values: {
          token: form.elements.token.value,
          proposal: form.elements.proposal.value,
        },
        target: "#proposal",
      });
    }
  });
  try {
    if (typeof htmx === "undefined")
      throw Error("The local interaction script failed to load. Reload the page and check /assets/htmx.min.js.");
    if (typeof cytoscape === "undefined")
      throw Error("The graph renderer failed to load. Reload the page and check /assets/cytoscape.min.js.");
    htmx.config.allowEval = false;
    htmx.config.allowScriptTags = false;
    const response = await fetch("/api/model");
    if (!response.ok) throw Error(await response.text());
    const { snapshot: s, check } = await response.json();
    document.querySelector("#summary").textContent =
      `${s.modules.length} modules · ${s.seams.length} seams · ${check.violations.length} violations${check.incomplete ? " · analysis incomplete" : ""}`;
    const owners = new Map(s.modules.map((m) => [m.id, m.id]));
    for (const x of [...s.seams, ...s.symbols]) owners.set(x.id, x.moduleId);
    const violated = new Set(check.violations.map((v) => v.relationId));
    const pairs = new Map();
    for (const r of s.relations) {
      const from = owners.get(r.fromId),
        to = owners.get(r.toId);
      if (from === to || !["imports", "uses", "calls"].includes(r.kind))
        continue;
      const id = from + "→" + to;
      if (!pairs.has(id))
        pairs.set(id, {
          id,
          source: from,
          target: to,
          count: 0,
          violated: false,
        });
      const p = pairs.get(id);
      p.count++;
      p.violated ||= violated.has(r.id);
    }
    const nodes = s.modules.map((m) => ({
      data: {
        id: m.id,
        label: m.displayName,
        search: [
          m.id,
          ...s.seams.filter((x) => x.moduleId === m.id).map((x) => x.id),
        ]
          .join(" ")
          .toLowerCase(),
        violated: check.violations.some(
          (v) => v.consumerId === m.id || v.providerId === m.id,
        ),
      },
    }));
    document.querySelector("#graph").replaceChildren();
    const cy = cytoscape({
      container: document.querySelector("#graph"),
      maxZoom: 1.8,
      elements: [...nodes, ...[...pairs.values()].map((data) => ({ data }))],
      style: [
        {
          selector: "node",
          style: {
            "background-color": "#142f32",
            "border-width": 2,
            "border-color": "#34d399",
            label: "data(label)",
            color: "#e2e8f0",
            "font-size": 12,
            width: 68,
            height: 68,
            "text-valign": "center",
            "text-wrap": "wrap",
            "text-max-width": 80,
          },
        },
        {
          selector: "edge",
          style: {
            width: 1.5,
            "line-color": "#475569",
            "target-arrow-color": "#475569",
            "target-arrow-shape": "triangle",
            "curve-style": "bezier",
          },
        },
        { selector: "node[?violated]", style: { "border-color": "#fb7185" } },
        {
          selector: "edge[?violated]",
          style: { "line-color": "#fb7185", "target-arrow-color": "#fb7185" },
        },
        {
          selector: ":selected",
          style: { "border-color": "#f8fafc", "border-width": 4 },
        },
        {
          selector: ".added",
          style: {
            "background-color": "#065f46",
            "line-color": "#34d399",
            "target-arrow-color": "#34d399",
            "border-color": "#34d399",
          },
        },
        {
          selector: ".removed",
          style: {
            "background-color": "#4c1d2b",
            "border-color": "#fb7185",
            "line-style": "dashed",
            "line-color": "#fb7185",
            "target-arrow-color": "#fb7185",
          },
        },
        {
          selector: ".changed",
          style: {
            "border-color": "#fbbf24",
            "line-color": "#fbbf24",
            "target-arrow-color": "#fbbf24",
          },
        },
        { selector: ".hidden", style: { display: "none" } },
      ],
      layout: {
        name: s.modules.length > 40 ? "grid" : "cose",
        animate: false,
        nodeRepulsion: () => 9000,
      },
      wheelSensitivity: 0.25,
    });
    const graphObserver = new ResizeObserver(() => {
      cy.resize();
      cy.fit(cy.elements(":visible"), 35);
    });
    graphObserver.observe(document.querySelector("#graph"));
    document.addEventListener("click", (event) => {
      const button = event.target.closest("[data-expand-module]");
      if (button) {
        const id = button.dataset.expandModule,
          node = cy.getElementById(id),
          children = cy.nodes().filter((n) => n.data("moduleId") === id);
        if (children.length) {
          children.remove();
          button.textContent = "Expand seams in graph";
        } else {
          for (const seam of s.seams.filter((x) => x.moduleId === id))
            cy.add({
              data: {
                id: seam.id,
                moduleId: id,
                parent: id,
                kind: "seam",
                label: seam.id.split(".").at(-1),
                search: seam.id.toLowerCase(),
              },
            });
          button.textContent = "Collapse seams in graph";
        }
        cy.layout({
          name: cy.nodes().length > 40 ? "grid" : "cose",
          animate: false,
        }).run();
      }
      if (event.target.closest('[data-view="architecture"]')) cy.fit();
    });
    cy.on("tap", "node", (event) =>
      htmx.ajax(
        "GET",
        "/module?id=" +
          encodeURIComponent(
            event.target.data("moduleId") || event.target.id(),
          ),
        {
          target: "#details",
        },
      ),
    );
    cy.on("tap", "edge", (event) =>
      htmx.ajax(
        "GET",
        "/edge?from=" +
          encodeURIComponent(event.target.data("source")) +
          "&to=" +
          encodeURIComponent(event.target.data("target")),
        { target: "#details" },
      ),
    );
    const filter = () => {
      const query = document.querySelector("#search").value.toLowerCase(),
        only = document.querySelector("#violations-only").checked;
      cy.nodes().forEach((n) =>
        n.toggleClass(
          "hidden",
          !n.data("search").includes(query) || (only && !n.data("violated")),
        ),
      );
      cy.edges().forEach((e) =>
        e.toggleClass(
          "hidden",
          e.source().hasClass("hidden") ||
            e.target().hasClass("hidden") ||
            (only && !e.data("violated")),
        ),
      );
      cy.fit(cy.elements(":visible"), 45);
    };
    document.querySelector("#search").addEventListener("input", filter);
    document
      .querySelector("#violations-only")
      .addEventListener("change", filter);
    document.addEventListener("htmx:afterSwap", () => {
      const element = document.querySelector("[data-architecture-changes]");
      if (!element) return;
      const changes = JSON.parse(element.dataset.architectureChanges);
      cy.elements(".revision-only").remove();
      cy.elements().removeClass("added removed changed");
      const oldOwners = new Map(owners);
      for (const c of changes) {
        for (const value of [c.before, c.after]) {
          if (value?.moduleId) oldOwners.set(value.id, value.moduleId);
        }
      }
      for (const c of changes) {
        const css =
          c.status === "added"
            ? "added"
            : c.status === "removed"
              ? "removed"
              : "changed";
        if (c.kind === "module") {
          if (!cy.getElementById(c.entityId).length) {
            const value = c.after || c.before;
            cy.add({
              data: {
                id: c.entityId,
                label: value.displayName,
                search: c.entityId.toLowerCase(),
              },
              classes: "revision-only",
            });
          }
          cy.getElementById(c.entityId).addClass(css);
        }
        if (c.kind === "relation") {
          const value = c.after || c.before,
            from = oldOwners.get(value.fromId),
            to = oldOwners.get(value.toId);
          if (from && to && from !== to) {
            const id = from + "→" + to;
            let edge = cy.getElementById(id);
            if (
              !edge.length &&
              cy.getElementById(from).length &&
              cy.getElementById(to).length
            )
              edge = cy.add({
                data: { id, source: from, target: to },
                classes: "revision-only",
              });
            edge.addClass(css);
          }
        }
      }
      if (cy.elements(".revision-only").length)
        cy.layout({ name: "grid", animate: false }).run();
    });
  } catch (error) {
    status.textContent = String(error);
    document.querySelector("#summary").textContent = "Architecture unavailable";
    const graph = document.querySelector("#graph");
    const panel = document.createElement("div");
    panel.className = "p-7 text-sm";
    panel.setAttribute("role", "alert");
    const title = document.createElement("h2");
    title.className = "text-xl text-red-300 mb-3";
    title.textContent = "Could not load the architecture";
    const message = document.createElement("pre");
    message.className = "whitespace-pre-wrap break-all text-slate-300";
    message.textContent = String(error);
    const hint = document.createElement("p");
    hint.className = "mt-4 text-slate-400";
    hint.textContent = "Check the server's --root directory and its seamgraph.yaml, then reload this page.";
    panel.append(title, message, hint);
    graph.replaceChildren(panel);
  }
});
