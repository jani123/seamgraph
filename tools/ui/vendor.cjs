const fs = require("node:fs"),
  path = require("node:path");
const target = path.resolve(__dirname, "../../internal/ui/assets");
for (const [source, destination] of Object.entries({
  "htmx.org/dist/htmx.min.js": "htmx.min.js",
  "@alpinejs/csp/dist/cdn.min.js": "alpine.min.js",
  "cytoscape/dist/cytoscape.min.js": "cytoscape.min.js",
  "htmx.org/LICENSE": "HTMX_LICENSE",
  "cytoscape/LICENSE": "CYTOSCAPE_LICENSE",
  "highlight.js/LICENSE": "HIGHLIGHT_LICENSE",
}))
  fs.copyFileSync(
    path.join(__dirname, "node_modules", source),
    path.join(target, destination),
  );


require("esbuild").buildSync({
  entryPoints:[path.join(__dirname,"source-worker.cjs")],
  bundle:true, platform:"browser", format:"iife", minify:true,
  outfile:path.join(target,"source-worker.js")
});

// Preserve the upstream theme and attribution while normalizing trailing spaces.
const theme = fs.readFileSync(
  path.join(__dirname,"node_modules/highlight.js/styles/github-dark.css"),"utf8"
).replace(/[ \t]+$/gm,"");
fs.writeFileSync(path.join(target,"highlight.css"),theme);
