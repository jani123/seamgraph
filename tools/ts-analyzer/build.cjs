const fs = require("node:fs");
const path = require("node:path");
const esbuild = require("esbuild");
const target = path.resolve(__dirname, "../../internal/analyzer/typescript");
const library = path.dirname(require.resolve("typescript"));
for (const file of fs.readdirSync(library)) {
  if (/^lib.*\.d\.ts$/.test(file))
    fs.copyFileSync(path.join(library, file), path.join(target, file));
}
for (const file of ["LICENSE.txt", "ThirdPartyNoticeText.txt"]) {
  const source = path.join(library, "..", file);
  if (fs.existsSync(source))
    fs.copyFileSync(source, path.join(target, "TYPESCRIPT_" + file));
}
esbuild.buildSync({
  entryPoints: [path.join(__dirname, "analyzer.cjs")],
  bundle: true,
  platform: "node",
  format: "cjs",
  minify: true,
  outfile: path.join(target, "helper.cjs"),
});
