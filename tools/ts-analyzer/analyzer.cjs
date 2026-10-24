"use strict";
const ts = require("typescript");
const fs = require("node:fs");
const path = require("node:path");
function analyze(req) {
  if (req.version !== 1 || Number(process.versions.node.split(".")[0]) < 18)
    throw Error("Protocol v1 and Node.js 18+ required");
  const root = fs.realpathSync(req.root),
    config = req.config,
    modules = config.Modules;
  const snap = {
    schemaVersion: 1,
    modules: [],
    seams: [],
    symbols: [],
    relations: [],
    diagnostics: [],
  };
  const rel = (p) =>
    path.relative(root, fs.realpathSync(p)).split(path.sep).join("/");
  const safe = (p) =>
    p && p !== ".." && !p.startsWith("../") && !path.isAbsolute(p);
  const within = (p, r) => r === "." || p === r || p.startsWith(r + "/");
  const owner = (p) => {
    try {
      p = rel(p);
      if (!safe(p)) return "";
      for (const [name, m] of Object.entries(modules))
        if (m.Paths.some((r) => within(p, r))) return "module:" + name;
    } catch {}
    return "";
  };
  const enc = (x) =>
    encodeURIComponent(x).replace(
      /[!'()*]/g,
      (c) => "%" + c.charCodeAt(0).toString(16).toUpperCase(),
    );
  // Match Go url.PathEscape: slashes escaped; plus, colon, @, &, =, $ retained.
  const pathEnc = (x) =>
    enc(x).replace(/%(2B|3A|40|26|3D|24)/g, (_, hex) =>
      String.fromCharCode(parseInt(hex, 16)),
    );
  const edgeID = (kind, from, to) =>
    "edge:" +
    kind +
    ":" +
    enc(from).replace(/%20/g, "+") +
    "->" +
    enc(to).replace(/%20/g, "+");
  const evidence = (node, resolution = "resolved") => {
    const sf = node.getSourceFile(),
      start = sf.getLineAndCharacterOfPosition(node.getStart(sf)),
      end = sf.getLineAndCharacterOfPosition(node.getEnd());
    return {
      file: rel(sf.fileName),
      startLine: start.line + 1,
      startColumn: start.character + 1,
      endLine: end.line + 1,
      endColumn: end.character + 1,
      confidence: "exact",
      resolution,
    };
  };
  const diagnostic = (
    code,
    message,
    node,
    resolution = "unresolved",
    severity = "warning",
  ) =>
    snap.diagnostics.push({
      code,
      severity,
      entityIds: [],
      evidence: node ? [evidence(node, resolution)] : [],
      message,
    });
  let roots = [];
  function walk(dir) {
    for (const f of fs.readdirSync(dir, { withFileTypes: true })) {
      if (
        [
          "node_modules",
          ".git",
          ".venv",
          "dist",
          ".build",
          ".seamgraph",
          ".devbox",
          ".codex-local",
        ].includes(f.name)
      )
        continue;
      const p = path.join(dir, f.name);
      if (f.isDirectory()) walk(p);
      else if (
        /\.(ts|tsx|mts|cts)$/.test(f.name) &&
        !f.name.endsWith(".d.ts") &&
        !/\.(test|spec)\./.test(f.name)
      ) {
        const r = rel(p);
        if (!safe(r)) throw Error("Source escapes project: " + p);
        roots.push(p);
      }
    }
  }
  for (const [name, m] of Object.entries(modules)) {
    if (m.Language !== "typescript") continue;
    for (const r of m.Paths) {
      const real = fs.realpathSync(path.join(root, r));
      if (real !== root && !safe(path.relative(root, real)))
        throw Error("Module root escapes project: " + r);
      walk(real);
    }
    snap.modules.push({
      id: "module:" + name,
      displayName: name,
      sourceRoots: m.Paths,
      seams: Object.keys(m.Seams || {}).map((n) => "seam:" + name + "." + n),
    });
    for (const n of Object.keys(m.Seams || {}))
      snap.seams.push({
        id: "seam:" + name + "." + n,
        moduleId: "module:" + name,
        contractMembers: [],
      });
  }
  let options = {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.NodeNext,
    moduleResolution: ts.ModuleResolutionKind.NodeNext,
    noEmit: true,
  };
  const configs = new Set();
  for (const m of Object.values(modules)) {
    if (m.Language !== "typescript") continue;
    if (m.TSConfig) configs.add(path.join(root, m.TSConfig));
    else
      for (const r of m.Paths) {
        const found = ts.findConfigFile(
          path.join(root, r),
          ts.sys.fileExists,
          "tsconfig.json",
        );
        if (found && safe(path.relative(root, found))) configs.add(found);
      }
  }
  if (configs.size > 1)
    throw Error(
      "Multiple TypeScript configurations in one scan need a shared root tsconfig (project references are not yet supported)",
    );
  const cfg = [...configs][0];
  if (cfg) {
    const parsed = ts.readConfigFile(cfg, ts.sys.readFile);
    if (parsed.error)
      throw Error(
        ts.flattenDiagnosticMessageText(parsed.error.messageText, "\n"),
      );
    const resolved = ts.parseJsonConfigFileContent(
      parsed.config,
      ts.sys,
      path.dirname(cfg),
    );
    if (resolved.errors.length)
      throw Error(
        resolved.errors
          .map((d) => ts.flattenDiagnosticMessageText(d.messageText, "\n"))
          .join("\n"),
      );
    options = { ...resolved.options, noEmit: true };
  }
  const program = ts.createProgram([...new Set(roots)], options),
    checker = program.getTypeChecker();
  for (const d of ts.getPreEmitDiagnostics(program)) {
    let node;
    if (d.file && safe(path.relative(root, d.file.fileName))) {
      const at = d.start || 0;
      const a = d.file.getLineAndCharacterOfPosition(at),
        z = d.file.getLineAndCharacterOfPosition(at + (d.length || 1));
      snap.diagnostics.push({
        code: "TS_" + d.code,
        severity: "error",
        entityIds: [],
        message: ts.flattenDiagnosticMessageText(d.messageText, "\n"),
        evidence: [
          {
            file: rel(d.file.fileName),
            startLine: a.line + 1,
            startColumn: a.character + 1,
            endLine: z.line + 1,
            endColumn: z.character + 1,
            confidence: "exact",
            resolution: "unresolved",
          },
        ],
      });
    } else
      diagnostic(
        "TS_" + d.code,
        ts.flattenDiagnosticMessageText(d.messageText, "\n"),
        undefined,
        "unresolved",
        "error",
      );
  }
  const symIDs = new Map(),
    byName = new Map(),
    symbolNodes = new Map(),
    idSymbols = new Map();
  const deref = (s) => {
    const seen = new Set();
    while (s && s.flags & ts.SymbolFlags.Alias && !seen.has(s)) {
      seen.add(s);
      s = checker.getAliasedSymbol(s);
    }
    return s;
  };
  function accessorSignatures(symbol) {
    return (symbol.declarations || [])
      .filter(
        (d) => ts.isGetAccessorDeclaration(d) || ts.isSetAccessorDeclaration(d),
      )
      .map((d) => {
        const signature = checker.getSignatureFromDeclaration(d);
        return (
          (ts.isGetAccessorDeclaration(d) ? "get " : "set ") +
          (signature
            ? checker.signatureToString(
                signature,
                d,
                ts.TypeFormatFlags.NoTruncation,
              )
            : "unknown")
        );
      })
      .sort()
      .join("; ");
  }
  function constructorAccess(node) {
    const declaration = node.members.find(ts.isConstructorDeclaration);
    if (!declaration) return "";
    const flags = ts.getCombinedModifierFlags(declaration);
    return flags & ts.ModifierFlags.Private
      ? "private "
      : flags & ts.ModifierFlags.Protected
        ? "protected "
        : "";
  }
  function contractModifiers(declaration) {
    const flags = ts.getCombinedModifierFlags(declaration);
    return [
      flags & ts.ModifierFlags.Abstract ? "abstract" : "",
      flags & ts.ModifierFlags.Readonly ? "readonly" : "",
      flags & ts.ModifierFlags.Static ? "static" : "",
    ]
      .filter(Boolean)
      .join(" ");
  }
  function add(s) {
    s = deref(s);
    if (!s) return "";
    if (symIDs.has(s)) return symIDs.get(s);
    const decl = (s.declarations || []).find((d) => !ts.isSourceFile(d));
    if (!decl) return "";
    // Instantiated member symbols can have the same declaration as the original
    // symbol. Architecture identities and signatures describe that declaration,
    // rather than each consumer's inferred instantiation.
    const declared = decl.name && checker.getSymbolAtLocation(decl.name);
    if (declared && declared !== s) return add(declared);
    const sf = decl.getSourceFile(),
      own = owner(sf.fileName);
    if (!own || modules[own.slice(7)].Language !== "typescript") return "";
    const name = s.getName();
    if (name.startsWith("__")) return "";
    let suffix =
      (ts.getCombinedModifierFlags(decl) & ts.ModifierFlags.Static
        ? "static."
        : "") + name;
    let p = decl.parent;
    while (p && !ts.isSourceFile(p)) {
      if (p.name && ts.isIdentifier(p.name))
        suffix = p.name.text + "." + suffix;
      else if (
        ts.isClassDeclaration(p) &&
        !p.name &&
        ts.getCombinedModifierFlags(p) & ts.ModifierFlags.Default
      )
        suffix = "default." + suffix;
      p = p.parent;
    }
    const id =
      "symbol:" + own.slice(7) + ":" + pathEnc(rel(sf.fileName) + "#" + suffix);
    symIDs.set(s, id);
    idSymbols.set(id, s);
    symbolNodes.set(id, decl);
    let exports = [];
    const mod = checker.getSymbolAtLocation(sf);
    if (mod) exports = checker.getExportsOfModule(mod).map(deref);
    const isPublic =
      exports.includes(s) ||
      decl.modifiers?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword);
    const type = checker.getTypeOfSymbolAtLocation(s, decl);
    const signature = checker.typeToString(
      type,
      decl,
      ts.TypeFormatFlags.NoTruncation,
    );
    snap.symbols.push({
      id,
      moduleId: own,
      kind: ts.isInterfaceDeclaration(decl)
        ? "interface"
        : ts.isClassDeclaration(decl)
          ? "class"
          : ts.isFunctionDeclaration(decl)
            ? "function"
            : "declaration",
      displaySignature:
        (contractModifiers(decl) ? contractModifiers(decl) + " " : "") +
        name +
        (s.flags & ts.SymbolFlags.Optional ? "?" : "") +
        ": " +
        signature +
        (accessorSignatures(s) ? " [" + accessorSignatures(s) + "]" : "") +
        (ts.isInterfaceDeclaration(decl) || ts.isTypeAliasDeclaration(decl)
          ? " " +
            checker.typeToString(
              checker.getDeclaredTypeOfSymbol(s),
              decl,
              ts.TypeFormatFlags.InTypeAlias | ts.TypeFormatFlags.NoTruncation,
            )
          : ""),
      visibility: isPublic ? "public" : "private",
    });
    byName.set(own + ":" + name, [...(byName.get(own + ":" + name) || []), id]);
    return id;
  }
  for (const sf of program.getSourceFiles()) {
    if (!owner(sf.fileName) || sf.isDeclarationFile) continue;
    function visit(node) {
      if (
        node.name &&
        ts.isIdentifier(node.name) &&
        (ts.isVariableDeclaration(node) ||
          ts.isFunctionDeclaration(node) ||
          ts.isClassDeclaration(node) ||
          ts.isInterfaceDeclaration(node) ||
          ts.isTypeAliasDeclaration(node) ||
          ts.isEnumDeclaration(node))
      )
        add(checker.getSymbolAtLocation(node.name));
      ts.forEachChild(node, visit);
    }
    visit(sf);
  }
  const entrypoints = new Map();
  const expandedSignatures = new Set();
  for (const seam of snap.seams) {
    const name = seam.moduleId.slice(7),
      sn = seam.id.slice(("seam:" + name + ".").length),
      decl = modules[name].Seams[sn],
      members = new Set();
    for (const selector of decl.Symbols || []) {
      const ids = byName.get(seam.moduleId + ":" + selector) || [];
      if (ids.length !== 1)
        throw Error(
          "Seam " +
            seam.id +
            " selector " +
            selector +
            " resolves to " +
            ids.length +
            " symbols",
        );
      members.add(ids[0]);
    }
    for (const ep of decl.Entrypoints || []) {
      const sf = program.getSourceFile(path.join(root, ep));
      if (!sf) throw Error("Entrypoint not in TypeScript program: " + ep);
      const s = checker.getSymbolAtLocation(sf);
      if (!s) throw Error("Entrypoint is not an exported module: " + ep);
      entrypoints.set(ep, seam.id);
      for (const exported of checker.getExportsOfModule(s)) {
        const id = add(exported);
        if (id) {
          const symbol = snap.symbols.find((x) => x.id === id);
          if (symbol.moduleId !== seam.moduleId) {
            diagnostic(
              "TS_FOREIGN_EXPORT",
              "Entrypoint re-exports a symbol owned by another module",
              sf,
              "unresolved",
              "error",
            );
          } else {
            members.add(id);
            symbol.visibility = "public";
          }
        }
      }
    }
    for (const id of [...members]) {
      const node = symbolNodes.get(id);
      if (
        node &&
        (ts.isClassDeclaration(node) || ts.isInterfaceDeclaration(node))
      ) {
        const symbol = idSymbols.get(id),
          type = checker.getDeclaredTypeOfSymbol(symbol);
        const signatures = [];
        for (const property of [
          ...checker.getPropertiesOfType(type),
          ...(ts.isClassDeclaration(node)
            ? checker.getPropertiesOfType(
                checker.getTypeOfSymbolAtLocation(symbol, node),
              )
            : []),
        ]) {
          const declaration =
            property.valueDeclaration || property.declarations?.[0];
          if (
            !declaration ||
            (declaration.name && ts.isPrivateIdentifier(declaration.name)) ||
            (ts.getCombinedModifierFlags(declaration) &
              (ts.ModifierFlags.Private | ts.ModifierFlags.Protected)) !==
              0
          )
            continue;
          const member = add(property);
          if (
            member &&
            snap.symbols.find((x) => x.id === member).moduleId === seam.moduleId
          ) {
            members.add(member);
            snap.symbols.find((x) => x.id === member).visibility = "public";
            signatures.push(
              (contractModifiers(declaration)
                ? contractModifiers(declaration) + " "
                : "") +
                property.name +
                (property.flags & ts.SymbolFlags.Optional ? "?" : "") +
                ": " +
                checker.typeToString(
                  checker.getTypeOfSymbolAtLocation(property, declaration),
                  declaration,
                  ts.TypeFormatFlags.NoTruncation,
                ) +
                (accessorSignatures(property)
                  ? " [" + accessorSignatures(property) + "]"
                  : ""),
            );
          }
        }
        if (!expandedSignatures.has(id)) {
          snap.symbols.find((x) => x.id === id).displaySignature +=
            " { " +
            signatures.sort().join("; ") +
            " }" +
            (ts.isClassDeclaration(node)
              ? " " +
                constructorAccess(node) +
                checker
                  .getTypeOfSymbolAtLocation(symbol, node)
                  .getConstructSignatures()
                  .map((sig) =>
                    checker.signatureToString(
                      sig,
                      node,
                      ts.TypeFormatFlags.NoTruncation,
                    ),
                  )
                  .join("; ")
              : "");
          expandedSignatures.add(id);
        }
      }
    }
    seam.contractMembers = [...members];
  }
  function relation(kind, from, to, node, bypass = false) {
    snap.relations.push({
      id: edgeID(kind, from, to),
      fromId: from,
      toId: to,
      kind,
      evidence: [evidence(node)],
      ...(bypass ? { bypassesSeam: true } : {}),
    });
  }
  for (const sf of program.getSourceFiles()) {
    const from = owner(sf.fileName);
    if (!from || sf.isDeclarationFile) continue;
    const bypassSymbols = new Map();
    for (const statement of sf.statements) {
      if (
        (ts.isImportDeclaration(statement) ||
          ts.isExportDeclaration(statement)) &&
        statement.moduleSpecifier &&
        ts.isStringLiteral(statement.moduleSpecifier)
      ) {
        const spec = statement.moduleSpecifier;
        const resolved = ts.resolveModuleName(
          spec.text,
          sf.fileName,
          options,
          ts.sys,
        ).resolvedModule;
        if (!resolved) {
          diagnostic(
            "TS_IMPORT_UNRESOLVED",
            "Cannot resolve " + spec.text,
            spec,
          );
          continue;
        }
        const to = owner(resolved.resolvedFileName);
        const targetPath = rel(resolved.resolvedFileName);
        if (
          !to &&
          safe(targetPath) &&
          !targetPath.split("/").includes("node_modules")
        )
          diagnostic(
            "TS_UNOWNED_PROJECT_REFERENCE",
            "Resolved local module has no declared architectural owner: " +
              targetPath,
            spec,
          );
        if (to && modules[to.slice(7)].Language !== "typescript") {
          diagnostic(
            "TS_TARGET_ANALYSIS_MISMATCH",
            "Imported TypeScript source belongs to a module configured for a different adapter",
            spec,
          );
          continue;
        }

        if (to && to !== from) {
          const provider = modules[to.slice(7)],
            hasEntrypoints = Object.values(provider.Seams || {}).some(
              (s) => (s.Entrypoints || []).length,
            ),
            bypass =
              hasEntrypoints &&
              !entrypoints.has(rel(resolved.resolvedFileName));
          relation("imports", from, to, spec, bypass);
          if (statement.importClause) {
            function mapNames(n) {
              if (ts.isIdentifier(n)) {
                const s = deref(checker.getSymbolAtLocation(n));
                if (s) bypassSymbols.set(s, bypass);
              }
              ts.forEachChild(n, mapNames);
            }
            mapNames(statement.importClause);
          }
        }
      }
    }
    function visit(node) {
      if (ts.isIdentifier(node)) {
        const original = checker.getSymbolAtLocation(node),
          s = deref(original),
          id = add(s);
        if (id) {
          const sym = snap.symbols.find((x) => x.id === id);
          if (sym.moduleId !== from)
            relation("uses", from, id, node, bypassSymbols.get(s) || false);
        }
      }
      if (ts.isElementAccessExpression(node)) {
        const base = deref(checker.getSymbolAtLocation(node.expression));
        const decl = base?.declarations?.[0];
        if (
          decl &&
          owner(decl.getSourceFile().fileName) &&
          owner(decl.getSourceFile().fileName) !== from
        ) {
          if (ts.isStringLiteral(node.argumentExpression)) {
            const property = checker.getPropertyOfType(
              checker.getTypeAtLocation(node.expression),
              node.argumentExpression.text,
            );
            const id = add(property);
            if (id)
              relation(
                "uses",
                from,
                id,
                node,
                bypassSymbols.get(base) || false,
              );
            else
              diagnostic(
                "TS_COMPUTED_REFERENCE",
                "Cannot resolve computed member",
                node,
              );
          } else
            diagnostic(
              "TS_DYNAMIC_REFERENCE",
              "Computed cross-module property access is not statically proven",
              node,
              "dynamic",
            );
        }
      }
      if (
        ts.isCallExpression(node) &&
        (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
          (ts.isIdentifier(node.expression) &&
            ["require", "eval"].includes(node.expression.text)))
      )
        diagnostic(
          "TS_DYNAMIC_REFERENCE",
          "Runtime module/code loading is not statically proven",
          node,
          "dynamic",
        );
      ts.forEachChild(node, visit);
    }
    visit(sf);
  }
  const concrete = [...idSymbols].filter(([id]) =>
    ts.isClassDeclaration(symbolNodes.get(id)),
  );
  const interfaces = [...idSymbols].filter(([id]) =>
    ts.isInterfaceDeclaration(symbolNodes.get(id)),
  );
  for (const [from, source] of concrete) {
    const sourceType = checker.getDeclaredTypeOfSymbol(source);
    for (const [to, target] of interfaces) {
      const targetType = checker.getDeclaredTypeOfSymbol(target);
      if (
        checker.getPropertiesOfType(targetType).length &&
        checker.isTypeAssignableTo(sourceType, targetType)
      )
        relation(
          "implements",
          from,
          to,
          symbolNodes.get(from).name || symbolNodes.get(from),
        );
    }
  }
  return snap;
}
module.exports = { analyze };
if (require.main === module) {
  let input = "";
  process.stdin.on("data", (b) => (input += b));
  process.stdin.on("end", () => {
    try {
      process.stdout.write(
        JSON.stringify({ version: 1, snapshot: analyze(JSON.parse(input)) }),
      );
    } catch (e) {
      process.stderr.write(String(e.stack || e));
      process.exitCode = 2;
    }
  });
}
