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
    let { snapshot: s, check } = await response.json();
    let activeComparison = null;
    document.querySelector("#summary").textContent =
      `${s.modules.length} modules · ${s.seams.length} seams · ${check.violations.length} violations${check.incomplete ? " · analysis incomplete" : ""}`;
    let { owners, pairs } = SeamGraphLayout.project(s, check.violations);
    if (![...pairs.values()].some(p => p.categories.includes("api")))
      document.querySelector("#dependency-view").value = "all";
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
            "font-size": 14,
            width: 130,
            height: 56,
            shape: "round-rectangle",
            "text-valign": "center",
            "text-wrap": "wrap",
            "text-max-width": 120,
          },
        },
        {
          selector: "edge",
          style: {
            width: 1.5,
            "line-color": "#475569",
            "target-arrow-color": "#475569",
            "target-arrow-shape": "triangle",
            "curve-style": "taxi",
            "taxi-direction": "horizontal",
            "source-endpoint": "outside-to-line",
            "target-endpoint": "outside-to-line",
          },
        },
        {selector: 'node[kind="seam"]',style:{"border-style":"dashed","border-color":"#fbbf24","font-size":12,height:44}},
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
        {
          selector: "edge.hover-connected",
          style: {
            width: 3.5,
            "line-color": "#67e8f9",
            "target-arrow-color": "#67e8f9",
            "z-index": 10,
          },
        },
        {
          selector: "edge.hover-connected.added",
          style: { "line-color": "#34d399", "target-arrow-color": "#34d399" },
        },
        {
          selector: "edge.hover-connected.changed",
          style: { "line-color": "#fbbf24", "target-arrow-color": "#fbbf24" },
        },
        {
          selector: "edge.hover-connected[?violated], edge.hover-connected.removed",
          style: { "line-color": "#fb7185", "target-arrow-color": "#fb7185" },
        },
        {selector:"edge.changed",style:{"line-style":"dotted"}},
        {selector:"edge[?violated]",style:{"underlay-color":"#fb7185","underlay-opacity":0.3,"underlay-padding":3}},
        {selector:"node.removed",style:{"border-style":"dashed","background-opacity":0.5}},
        // Change badges and violation borders communicate independent facts.
        {selector:"node[?violated]",style:{"border-color":"#fb7185","border-width":3}},
        { selector: ".hidden", style: { display: "none" } },
      ],
      layout: {name: "preset"},
      wheelSensitivity: 0.25,
    });
    let focusedModule = null;
    const clearHover = () => cy.edges(".hover-connected").removeClass("hover-connected");
    cy.on("mouseover", "node", (event) => {
      clearHover();
      event.target.connectedEdges().addClass("hover-connected");
    });
    cy.on("mouseover", "edge", (event) => {
      clearHover();
      event.target.addClass("hover-connected");
    });
    cy.on("mouseout", "node, edge", clearHover);
    document.querySelector("#graph").addEventListener("pointerleave", clearHover);
    const routeEdges = () => {
      const routes = SeamGraphLayout.orthogonal(
        cy.nodes().filter(n => !n.hasClass("hidden")).map(n => ({id:n.id(),...n.position(),
          width:n.numericStyle("width")+2*n.numericStyle("border-width"),
          height:n.numericStyle("height")+2*n.numericStyle("border-width")})),
        cy.edges().filter(e => !e.hasClass("hidden")).map(e => ({id:e.id(),source:e.data("source"),target:e.data("target")})));
      cy.batch(() => {
        cy.edges().filter(e => !e.hasClass("hidden") && !routes.has(e.id())).style({opacity:0});
        for (const [id,route] of routes) cy.getElementById(id).style(route.blocked ? {opacity:0} : {
          opacity:1,
          "curve-style":"segments", "edge-distances":"node-position",
          "source-endpoint":route.sourceEndpoint,"target-endpoint":route.targetEndpoint,
          "segment-weights":route.weights,"segment-distances":route.distances,
        });
      });
    };
    let routeFrame;
    cy.on("drag dragfree select unselect", "node", () => {
      cancelAnimationFrame(routeFrame);
      routeFrame = requestAnimationFrame(routeEdges);
    });
    const arrange = () => {
      const visible = cy.nodes().filter(n => !n.data("moduleId") && !n.hasClass("hidden"));
      const positions = SeamGraphLayout.positions(visible.map(n => n.id()),
        cy.edges().filter(e => !e.hasClass("hidden") && !e.data("seamEdge")).map(e => ({source: e.data("source"), target: e.data("target")})));
      const rowScale=Math.max(1,...visible.map(n=>(n.numericStyle("height")+36)/92));
      for(const position of positions.values())position.y*=rowScale;
      cy.nodes().filter(n => !n.data("moduleId")).positions(n => positions.get(n.id()) || {x: 0, y: 0});
      cy.nodes().filter(n => n.data("moduleId")).forEach((n, i) => {
        const p = positions.get(n.data("moduleId")) || {x: 0, y: 0};
        n.position({x: p.x + 85, y: p.y + 75 + i * 50});
      });
      routeEdges();
      cy.fit(cy.elements(":visible"), 45);
    };
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
          for (const seam of s.seams.filter((x) => x.moduleId === id)) {
            cy.add({
              data: {
                id: seam.id,
                moduleId: id,
                kind: "seam",
                label: seam.id.split(".").at(-1),
                search: seam.id.toLowerCase(),
              },
            });
            const members = new Set(seam.contractMembers);
            const projection = SeamGraphLayout.project({...s, relations: s.relations.filter(r => members.has(r.toId))}, check.violations);
            let violated = false;
            for (const pair of projection.pairs.values()) if (pair.target === id) {
              violated ||= pair.violated;
              cy.add({data: {...pair, id: pair.source + "→" + seam.id, target: seam.id, seamEdge: true}});
            }
            cy.getElementById(seam.id).data("violated", violated);
          }
          button.textContent = "Collapse seams in graph";
        }
        filter();
      }
      if (event.target.closest('[data-view="architecture"]')) {focusedModule = null; filter();}
    });
    cy.on("tap", "node", (event) => {
      focusedModule = event.target.id();
      filter();
      if (activeComparison) {showComparisonDetails(event.target);return;}
      htmx.ajax("GET", "/module?id=" + encodeURIComponent(event.target.data("moduleId") || focusedModule), {target: "#details"});
    });
    cy.on("tap", "edge", (event) => {
      if (activeComparison) {showComparisonDetails(event.target);return;}
      const target = cy.getElementById(event.target.data("target"));
      htmx.ajax("GET", "/edge?from=" + encodeURIComponent(event.target.data("source")) +
        "&to=" + encodeURIComponent(target.data("moduleId") || target.id()), {target: "#details"});
    });
    const filter = () => {
      const query = document.querySelector("#search").value.toLowerCase(),
        only = document.querySelector("#violations-only").checked,
        view = document.querySelector("#dependency-view").value;
      const neighbors = new Set([focusedModule, owners.get(focusedModule)]);
      if (focusedModule) for (const edge of cy.edges()) {
        const source = edge.data("source"), target = edge.data("target");
        if (source === focusedModule) neighbors.add(target);
        if (target === focusedModule) neighbors.add(source);
      }
      const changed = new Set();
      if (activeComparison && document.querySelector("#changes-only").checked) {
        cy.nodes().filter(n=>n.data("changeStatus")).forEach(n=>changed.add(n.id()));
        cy.edges().filter(e=>e.data("status")).forEach(e=>{changed.add(e.data("source"));changed.add(e.data("target"));});
        const seed=new Set(changed);
        cy.edges().forEach(e=>{if(seed.has(e.data("source"))||seed.has(e.data("target"))){changed.add(e.data("source"));changed.add(e.data("target"));}});
      }
      cy.nodes().forEach(n => n.toggleClass("hidden",
        !n.data("search").includes(query) ||
        (activeComparison && document.querySelector("#changes-only").checked && !changed.has(n.id())) || (only && !n.data("violated")) ||
        (focusedModule && !neighbors.has(n.id()) && n.data("moduleId") !== focusedModule)));
      cy.edges().forEach(e => e.toggleClass("hidden",
        e.source().hasClass("hidden") || e.target().hasClass("hidden") ||
        (only && !e.data("violated")) ||
        (view !== "all" && !e.data("categories")?.includes(view))));
      if (view !== "all") {
        const connected = new Set();
        cy.edges().filter(e => !e.hasClass("hidden")).forEach(e => {connected.add(e.data("source")); connected.add(e.data("target"));});
        cy.nodes().forEach(n => {if (!connected.has(n.id()) && n.id() !== focusedModule && !(activeComparison && n.data("changeStatus"))) n.addClass("hidden");});
      }
      arrange();
    };
    document.querySelector("#dependency-view").addEventListener("change", filter);
    document.querySelector("#changes-only").addEventListener("change", filter);
    document.querySelector("#clear-comparison").addEventListener("click",()=>location.reload());
    const scope=document.querySelector('[name="scope"]');
    const updateScope=()=>document.querySelectorAll('[name="base"], [name="head"]').forEach(input=>{input.disabled=scope.value==="working";});
    scope.addEventListener("change",updateScope);updateScope();
    document.querySelector("#reset-focus").addEventListener("click", () => {focusedModule = null; filter();});
    filter();
    document.querySelector("#search").addEventListener("input", filter);
    document
      .querySelector("#violations-only")
      .addEventListener("change", filter);
    function showComparisonDetails(element) {
      const panel=document.querySelector("#details");panel.replaceChildren();
      const line=(text,tag="p")=>{const node=document.createElement(tag);node.textContent=text;node.className="my-2 break-all";panel.append(node);};
      line(element.isNode()?element.data("label").split("\n")[0]:"Connection changes","h2");
      line(element.id(),"code");
      const violations=check.violations.filter(v=>element.isNode()
        ? v.consumerId===element.id()||v.providerId===element.id()
        : v.consumerId===element.data("source")&&v.providerId===element.data("target"));
      if(violations.length){line("Current violations","h3");for(const v of violations)line(`${v.code}: ${v.message}`);}
      if(element.isNode()) {
        const module=activeComparison.projection.modules.get(element.id()),change=module.change;
        line(change?.status || module.status || "Unchanged module");
        if(change?.files.length){line("Changed files","h3");for(const file of change.files)line(`${file.status}: ${file.oldPath?file.oldPath+" → ":""}${file.path}`);}
        if(change?.definitionChanged)line("Module definition changed (configuration).");
        if(change?.policyChanged)line("Policy violations changed for this module.");
        if(change?.seams.length){line("Seam changes","h3");for(const seam of change.seams){line(`${seam.status}: ${seam.entityId}`);line(JSON.stringify({before:seam.before,after:seam.after},null,2),"pre");}}
        for(const side of ["before","after"]){
          const graph=activeComparison.result[side],definition=graph.modules.find(m=>m.id===element.id());
          if(!definition)continue;
          line(side==="before"?"Before":"After","h3");line(`Roots: ${definition.sourceRoots.join(", ")}`);
          for(const seam of graph.seams.filter(x=>x.moduleId===element.id())){line(seam.id,"code");for(const id of seam.contractMembers){const symbol=graph.symbols.find(x=>x.id===id);line(symbol?.displaySignature||id);}}
        }
        const incident=cy.edges().filter(e=>e.data("status")&&(e.data("source")===element.id()||e.data("target")===element.id()));
        if(incident.length){line("Connection changes","h3");incident.forEach(e=>line(`${e.data("status")}: ${e.data("source")} → ${e.data("target")}`));}
      } else {
        line(element.data("status") || "Unchanged references");
        if(element.data("violationStatus"))line(`Policy violations ${element.data("violationStatus")}.`);
        line(`${element.data("beforeCount")} references before → ${element.data("afterCount")} after`);
        for(const change of element.data("changes")||[]) {
          line(`${change.status}: ${(change.after||change.before).id}`,"code");
          line(JSON.stringify({before:change.before,after:change.after},null,2),"pre");
        }
      }
    }
    document.addEventListener("htmx:afterSwap", () => {
      const element = document.querySelector("[data-comparison]");
      if (!element || element.dataset.applied) return;
      element.dataset.applied="true";
      const result=JSON.parse(element.dataset.comparison),projection=SeamGraphLayout.comparison(result);
      activeComparison={result,projection};s=result.after;check=result.afterCheck;
      document.querySelector("#summary").textContent=`${s.modules.length} current modules · ${result.modules.length} changed modules · ${check.violations.length} violations`;
      owners=projection.owners;pairs=projection.pairs;focusedModule=null;
      cy.elements().remove();
      for(const module of projection.modules.values()) {
        const change=module.change,status=module.status || change?.status || "";
        const badges=[];
        if(status==="added")badges.push("+ Added");else if(status==="removed")badges.push("− Removed");
        else if(change?.files.length)badges.push("~ Modified");
        if(change?.seams.length)badges.push("Seam changed");
        if(change?.definitionChanged)badges.push("Config changed");
        if(change?.policyChanged)badges.push("Policy changed");
        cy.add({data:{id:module.id,label:[module.displayName,...badges].join("\n"),
          search:[module.id,module.displayName,...module.seams].join(" ").toLowerCase(),
          changeStatus:status,violated:check.violations.some(v=>v.consumerId===module.id||v.providerId===module.id)},
          classes:status==="modified"?"changed":status}).style("height",Math.max(56,(badges.length+1)*18+12));
      }
      for(const pair of pairs.values())cy.add({data:pair,classes:pair.status==="modified"||(!pair.status&&pair.violationStatus)?"changed":pair.status});
      document.querySelector("#changes-only").disabled=false;
      document.querySelector("#clear-comparison").hidden=false;
      document.querySelector("#details").textContent="Select a module or connection to inspect its changes. Removed objects show their before state.";
      filter();
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
