const fs = require("node:fs"),
  path = require("node:path");
const target = path.resolve(__dirname, "../../internal/ui/assets");
for (const [source, destination] of Object.entries({
  "htmx.org/dist/htmx.min.js": "htmx.min.js",
  "@alpinejs/csp/dist/cdn.min.js": "alpine.min.js",
  "cytoscape/dist/cytoscape.min.js": "cytoscape.min.js",
  "htmx.org/LICENSE": "HTMX_LICENSE",
  "cytoscape/LICENSE": "CYTOSCAPE_LICENSE",
}))
  fs.copyFileSync(
    path.join(__dirname, "node_modules", source),
    path.join(target, destination),
  );
