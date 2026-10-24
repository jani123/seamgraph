"""SeamGraph protocol v1: static AST and project import resolution only."""
import ast
import hashlib
import json
import os
from pathlib import Path
import sys
from urllib.parse import quote, quote_plus


def analyze(request):
    if request.get('version') != 1 or sys.version_info < (3, 10):
        raise ValueError('Protocol v1 and Python 3.10+ required')
    root = Path(request['root']).resolve()
    configs = request['config']['Modules']
    result = dict(schemaVersion=1, modules=[], seams=[], symbols=[], relations=[], diagnostics=[])
    def relative(file):
        return Path(file).resolve().relative_to(root).as_posix()
    def inside(file, directory):
        return directory == '.' or file == directory or file.startswith(directory + '/')
    def owner(file):
        file = relative(file)
        return next(('module:'+name for name, m in configs.items() if any(inside(file, p) for p in m['Paths'])), '')
    def evidence(file, node, resolution='resolved'):
        return dict(file=relative(file), startLine=node.lineno, startColumn=node.col_offset+1,
                    endLine=getattr(node, 'end_lineno', node.lineno), endColumn=getattr(node, 'end_col_offset', node.col_offset+1)+1,
                    confidence='best-effort' if resolution != 'resolved' else 'exact', resolution=resolution)
    def diag(code, message, file=None, node=None, resolution='unresolved', severity='warning'):
        result['diagnostics'].append(dict(code=code, severity=severity, entityIds=[],
                    evidence=[evidence(file, node, resolution)] if node else [], message=message))
    def relation(kind, source, target, file, node, bypass=False):
        location = evidence(file, node)
        location['kind'] = 'reference'
        info = files[Path(file).resolve()]
        parent = info['parents'].get(node)
        if isinstance(parent, ast.Call) and parent.func is node:
            location['kind'] = 'call'
        current = node
        while current is not None:
            if current in info['declarations']:
                location['sourceId'] = info['declarations'][current]
                break
            current = info['parents'].get(current)
        edge = dict(id='edge:'+kind+':'+quote_plus(source)+'->'+quote_plus(target), fromId=source, toId=target, kind=kind, evidence=[location])
        if bypass:
            edge['bypassesSeam'] = True
        result['relations'].append(edge)
    def project_files(directory):
        ignored = {'.venv', '__pycache__', '.git', 'node_modules', '.build', '.seamgraph', '.devbox', '.codex-local'}
        for current, directories, names in os.walk(directory):
            directories[:] = sorted(d for d in directories if d not in ignored)
            for name in sorted(names):
                if name.endswith('.py'):
                    yield Path(current)/name
    files = {}
    import_roots = {root}
    for name, conf in configs.items():
        if conf['Language'] != 'python':
            continue
        import_roots.update(root/p for p in (conf.get('ImportRoots') or []))
        result['modules'].append(dict(id='module:'+name, displayName=name, sourceRoots=conf['Paths'], seams=['seam:'+name+'.'+s for s in (conf.get('Seams') or {})]))
        for seam in (conf.get('Seams') or {}):
            result['seams'].append(dict(id='seam:'+name+'.'+seam, moduleId='module:'+name, contractMembers=[]))
        for directory in conf['Paths']:
            source_root = root/directory
            source_root.resolve().relative_to(root)
            if not source_root.is_dir():
                raise ValueError('Missing Python root '+directory)
            for file in project_files(source_root):
                if any(p in {'.venv', '__pycache__', '.git', 'node_modules', '.build', '.seamgraph', '.devbox', '.codex-local'} for p in file.relative_to(root).parts) or file.name.startswith('test_') or file.name.endswith('_test.py'):
                    continue
                relative(file)
                files[file.resolve()] = dict(file=file.resolve(), tree=ast.parse(file.read_text(encoding='utf-8'), filename=relative(file)), own='module:'+name, symbols={}, imports={}, exports={}, all=None, declarations={})
    for info in files.values():
        info['parents'] = {child: parent for parent in ast.walk(info['tree']) for child in ast.iter_child_nodes(parent)}
    modules = {}
    for file, info in files.items():
        candidates = []
        for import_root in sorted(import_roots, key=lambda p: -len(p.parts)):
            try:
                name = file.relative_to(import_root).with_suffix('').parts
            except ValueError:
                continue
            if name[-1] == '__init__':
                name = name[:-1]
            if name:
                key = '.'.join(name)
                if key in modules and modules[key] is not info:
                    raise ValueError('Ambiguous Python import '+key)
                modules[key] = info
                candidates.append(key)
        info['name'] = min(candidates, key=lambda s: len(s.split('.'))) if candidates else ''
    symbol_map = {}
    namespace_targets = {}
    signature_groups = set()
    def add_symbol(info, name, node, kind, signature):
        canonical = relative(info['file'])+'#'+name
        sid = 'symbol:'+info['own'][7:]+':'+quote(canonical, safe='+:@&=$')
        visibility = 'private' if name.split('.')[-1].startswith('_') else 'public'
        parent_name = name.rsplit('.', 1)[0] if '.' in name else ''
        parent_id = info['symbols'].get(parent_name, '')
        documentation = ast.get_docstring(node, clean=True) if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)) else ''
        symbol = dict(id=sid, moduleId=info['own'], kind=kind, displaySignature=signature, visibility=visibility,
                      definition=dict(digest=hashlib.sha256(ast.dump(node, include_attributes=False).encode()).hexdigest(), name=name.rsplit('.', 1)[-1], parentId=parent_id, location=evidence(info['file'], node) if hasattr(node, 'lineno') else {}, documentation=documentation or ''))
        if not hasattr(node, 'lineno'):
            symbol.pop('definition')
        info['declarations'][node] = sid
        decorators = [ast.unparse(d) for d in getattr(node, 'decorator_list', [])]
        grouped = any(d == 'property' or d == 'overload' or d.endswith(('.setter', '.getter', '.deleter', '.overload')) for d in decorators)
        if sid in symbol_map and (grouped or sid in signature_groups):
            previous = symbol_map[sid]
            symbol['displaySignature'] = previous['displaySignature'] + '\n' + signature
            symbol['definition']['digest'] = hashlib.sha256((previous['definition']['digest'] + symbol['definition']['digest']).encode()).hexdigest()
            symbol['definition']['location'] = previous['definition']['location']
            symbol['definition']['documentation'] = previous['definition']['documentation'] or symbol['definition']['documentation']
        if grouped:
            signature_groups.add(sid)
        symbol_map[sid] = symbol
        info['symbols'][name] = sid
        return sid
    for info in files.values():
        for node in info['tree'].body:
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                signature = ''.join('@'+ast.unparse(d)+' ' for d in node.decorator_list) + ('async ' if isinstance(node, ast.AsyncFunctionDef) else '')+node.name+'('+ast.unparse(node.args)+')'+(' -> '+ast.unparse(node.returns) if node.returns else '')
                add_symbol(info, node.name, node, 'function', signature)
            elif isinstance(node, ast.ClassDef):
                signature = 'class '+node.name+'('+', '.join(ast.unparse(b) for b in node.bases)+')'
                constructor = next((m for m in node.body if isinstance(m, (ast.FunctionDef, ast.AsyncFunctionDef)) and m.name == '__init__'), None)
                if constructor:
                    signature += ' constructor('+ast.unparse(constructor.args)+')'
                add_symbol(info, node.name, node, 'class', signature)
                for member in node.body:
                    if isinstance(member, (ast.FunctionDef, ast.AsyncFunctionDef)):
                        name = node.name+'.'+member.name
                        add_symbol(info, name, member, 'method', ''.join('@'+ast.unparse(d)+' ' for d in member.decorator_list)+name+'('+ast.unparse(member.args)+')'+(' -> '+ast.unparse(member.returns) if member.returns else ''))
                    elif isinstance(member, ast.AnnAssign) and isinstance(member.target, ast.Name):
                        name = node.name+'.'+member.target.id
                        add_symbol(info, name, member, 'field', name+': '+ast.unparse(member.annotation))
            elif isinstance(node, (ast.Assign, ast.AnnAssign)):
                targets = node.targets if isinstance(node, ast.Assign) else [node.target]
                for target in targets:
                    if not isinstance(target, ast.Name):
                        continue
                    if target.id == '__all__':
                        try:
                            exported = ast.literal_eval(node.value)
                            if not isinstance(exported, (list, tuple)) or not all(isinstance(s, str) for s in exported):
                                raise ValueError()
                            info['all'] = exported
                        except (ValueError, TypeError, SyntaxError):
                            diag('PY_DYNAMIC_EXPORTS', '__all__ is not a literal list of strings', info['file'], node, 'dynamic')
                    else:
                        annotation = ast.unparse(node.annotation) if isinstance(node, ast.AnnAssign) else 'value'
                        add_symbol(info, target.id, node, 'variable', target.id+': '+annotation)
            elif isinstance(node, (ast.Import, ast.ImportFrom)):
                if isinstance(node, ast.Import):
                    for alias in node.names:
                        info['imports'][alias.asname or alias.name.split('.')[0]] = ('module', alias.name, '', node)
                else:
                    base = node.module or ''
                    if node.level:
                        package = info['name'].split('.') if info['file'].name == '__init__.py' else info['name'].split('.')[:-1]
                        if node.level > len(package):
                            diag('PY_RELATIVE_IMPORT', 'Relative import escapes configured package', info['file'], node)
                        base = '.'.join(package[:len(package)-node.level+1]+([base] if base else []))
                    for alias in node.names:
                        info['imports'][alias.asname or alias.name] = ('member', base, alias.name, node)
    resolving = set()
    def resolve(module_name, member=''):
        key = (module_name, member)
        if key in resolving:
            return None
        info = modules.get(module_name)
        if not info:
            # Namespace packages have no __init__.py. Resolve descendants explicitly.
            if not member and any(m.startswith(module_name+'.') for m in modules):
                return ('module', module_name)
            return None
        if not member:
            return ('module', module_name)
        if member in info['symbols']:
            sid = info['symbols'][member]
            return ('module', namespace_targets[sid]) if sid in namespace_targets else ('symbol', sid)
        if member in info['imports']:
            kind, target, attr, _ = info['imports'][member]
            resolving.add(key)
            value = resolve(target, attr if kind == 'member' else '')
            resolving.remove(key)
            return value
        if module_name+'.'+member in modules or any(m.startswith(module_name+'.'+member+'.') for m in modules):
            return ('module', module_name+'.'+member)
        # Explicit static star re-export, honoring __all__ when present.
        star = info['imports'].get('*')
        if star:
            target = modules.get(star[1])
            if target and (target['all'] is None and not member.startswith('_') or member in (target['all'] or [])):
                resolving.add(key)
                value = resolve(star[1], member)
                resolving.remove(key)
                return value
        return None
    def export_names(module_name, seen=None):
        seen = set() if seen is None else seen
        if module_name in seen:
            return []
        seen.add(module_name)
        info = modules.get(module_name)
        if not info:
            return []
        if info['all'] is not None:
            return info['all']
        names = {n for n in set(info['symbols']) | set(info['imports']) if n != '*' and not n.startswith('_') and '.' not in n}
        if '*' in info['imports']:
            names.update(export_names(info['imports']['*'][1], seen))
        return sorted(names)
    entrypoints = {}
    for seam in result['seams']:
        name = seam['moduleId'][7:]
        sn = seam['id'][len('seam:'+name+'.'):]
        declaration = configs[name]['Seams'][sn]
        members = set()
        for selector in declaration.get('Symbols') or []:
            found = [sid for info in files.values() if info['own'] == seam['moduleId'] for n, sid in info['symbols'].items() if n == selector]
            if len(found) != 1:
                raise ValueError('Seam '+seam['id']+' selector '+selector+' resolves to '+str(len(found))+' symbols')
            members.update(found)
        for ep in declaration.get('Entrypoints') or []:
            file = (root/ep).resolve()
            info = files.get(file)
            if not info:
                raise ValueError('Missing Python entrypoint '+ep)
            entrypoints[file] = seam['id']
            exported = export_names(info['name'])
            for n in exported:
                resolved = resolve(info['name'], n)
                if resolved and resolved[0] == 'symbol':
                    if symbol_map[resolved[1]]['moduleId'] != seam['moduleId']:
                        diag('PY_FOREIGN_EXPORT', 'Entrypoint exports a symbol owned by another module: '+n, file, severity='error')
                    else:
                        members.add(resolved[1])
                elif resolved and resolved[0] == 'module':
                    target_info = modules.get(resolved[1])
                    if target_info and target_info['own'] == seam['moduleId']:
                        namespace = add_symbol(info, n, info['tree'], 'namespace', 'namespace '+n)
                        namespace_targets[namespace] = resolved[1]
                        members.add(namespace)
                        for exported_name in export_names(resolved[1]):
                            value = resolve(resolved[1], exported_name)
                            if value and value[0] == 'symbol' and symbol_map[value[1]]['moduleId'] == seam['moduleId']:
                                members.add(value[1])
                            elif not value:
                                diag('PY_EXPORT_UNRESOLVED', 'Cannot prove namespace export '+n+'.'+exported_name, file, severity='error')
                    else:
                        diag('PY_EXPORT_UNRESOLVED', 'Cannot prove namespace export '+n, file, severity='error')
                else:
                    diag('PY_EXPORT_UNRESOLVED', 'Cannot prove exported member '+n, file, severity='error')
        for sid in list(members):
            symbol_map[sid]['visibility'] = 'public'
            # Public class methods form part of the selected class contract.
            for other, symbol in symbol_map.items():
                if other.startswith(sid+'.') and symbol['visibility'] == 'public':
                    members.add(other)
        seam['contractMembers'] = sorted(members)
    def external(module):
        return module.split('.')[0] in sys.stdlib_module_names
    for info in files.values():
        aliases = {}
        for local, (kind, target, attr, node) in info['imports'].items():
            resolved = resolve(target, attr if kind == 'member' else '')
            target_info = modules.get(target)
            if local == '*':
                if target_info:
                    for n in export_names(target):
                        value = resolve(target, n)
                        if value:
                            aliases[n] = value
                else:
                    diag('PY_STAR_IMPORT', 'Star import cannot be resolved: '+target, info['file'], node)
                continue
            if not resolved:
                if not external(target):
                    diag('PY_IMPORT_UNRESOLVED', 'Cannot resolve '+target+('.'+attr if attr else ''), info['file'], node)
                continue
            aliases[local] = resolved
            if kind == 'module' and isinstance(node, ast.Import):
                for imported in node.names:
                    if not imported.asname and imported.name.split('.')[0] == local:
                        aliases[local] = ('module', local)
            if target_info and target_info['own'] != info['own']:
                provider = configs[target_info['own'][7:]]
                has_entrypoints = any(s.get('Entrypoints') for s in (provider.get('Seams') or {}).values())
                bypass = has_entrypoints and target_info['file'] not in entrypoints
                relation('imports', info['own'], target_info['own'], info['file'], node, bypass)
                if resolved[0] == 'symbol':
                    relation('uses', info['own'], resolved[1], info['file'], node, bypass)
        direct_imports = {id(n) for n in info['tree'].body if isinstance(n, (ast.Import, ast.ImportFrom))}
        scopes = {}
        def mark_scope(node, shadowed):
            scopes[id(node)] = shadowed
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Lambda)):
                local = {a.arg for a in node.args.posonlyargs + node.args.args + node.args.kwonlyargs}
                if node.args.vararg:
                    local.add(node.args.vararg.arg)
                if node.args.kwarg:
                    local.add(node.args.kwarg.arg)
                def bindings(child):
                    if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef, ast.Lambda)):
                        return
                    if isinstance(child, ast.Name) and isinstance(child.ctx, ast.Store):
                        local.add(child.id)
                    for descendant in ast.iter_child_nodes(child):
                        bindings(descendant)
                bodies = node.body if isinstance(node.body, list) else [node.body]
                for child in bodies:
                    bindings(child)
                for child in ast.iter_child_nodes(node):
                    if child not in bodies:
                        mark_scope(child, shadowed)
                for child in bodies:
                    mark_scope(child, shadowed | local)
                return
            for child in ast.iter_child_nodes(node):
                mark_scope(child, shadowed)
        mark_scope(info['tree'], set())
        rebound = set()
        for statement in info['tree'].body:
            if isinstance(statement, (ast.Assign, ast.AnnAssign, ast.AugAssign)):
                targets = statement.targets if isinstance(statement, ast.Assign) else [statement.target]
                for target in targets:
                    if isinstance(target, ast.Name) and target.id in aliases:
                        rebound.add(target.id)
                        diag('PY_REBOUND_IMPORT', 'Imported binding is reassigned: '+target.id, info['file'], target, 'dynamic')
        for name in rebound:
            aliases.pop(name, None)
        def resolve_expr(node):
            if isinstance(node, ast.Name):
                return None if node.id in scopes.get(id(node), set()) else (aliases.get(node.id) or (('symbol', info['symbols'][node.id]) if node.id in info['symbols'] else None))
            if isinstance(node, ast.Attribute):
                base = resolve_expr(node.value)
                if base and base[0] == 'module':
                    return resolve(base[1], node.attr)
                if base and base[0] == 'symbol' and base[1]+'.'+node.attr in symbol_map:
                    return ('symbol', base[1]+'.'+node.attr)
            return None
        for node in ast.walk(info['tree']):
            if isinstance(node, ast.AugAssign) and isinstance(node.target, ast.Name) and node.target.id == '__all__':
                diag('PY_DYNAMIC_EXPORTS', 'Mutating __all__ needs runtime resolution', info['file'], node, 'dynamic')
            if isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute) and isinstance(node.func.value, ast.Name) and node.func.value.id == '__all__':
                diag('PY_DYNAMIC_EXPORTS', 'Runtime __all__ mutation cannot be proven', info['file'], node, 'dynamic')
            if isinstance(node, (ast.Import, ast.ImportFrom)) and id(node) not in direct_imports:
                diag('PY_SCOPED_IMPORT_UNRESOLVED', 'Scoped/conditional imports are not yet resolved', info['file'], node)
            annotation = node.annotation if isinstance(node, (ast.AnnAssign, ast.arg)) else node.returns if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) else None
            if isinstance(annotation, ast.Constant) and isinstance(annotation.value, str):
                diag('PY_STRING_ANNOTATION_UNRESOLVED', 'String type annotations require additional resolution', info['file'], annotation)
            if isinstance(node, ast.Attribute):
                if isinstance(node.ctx, (ast.Store, ast.Del)) and resolve_expr(node.value):
                    diag('PY_MONKEY_PATCH', 'Imported attribute mutation cannot be proven safe', info['file'], node, 'dynamic')
                value = resolve_expr(node)
                if value and value[0] == 'symbol':
                    relation('uses', info['own'], value[1], info['file'], node)
                elif resolve_expr(node.value) and not value:
                    diag('PY_ATTRIBUTE_UNRESOLVED', 'Imported attribute cannot be statically resolved: '+ast.unparse(node), info['file'], node)
            elif isinstance(node, ast.Name) and isinstance(node.ctx, ast.Load):
                value = resolve_expr(node)
                if value and value[0] == 'symbol':
                    relation('uses', info['own'], value[1], info['file'], node)
            elif isinstance(node, ast.Call):
                callee = ast.unparse(node.func)
                dynamic_alias = False
                if isinstance(node.func, ast.Name) and node.func.id in info['imports']:
                    imported = info['imports'][node.func.id]
                    dynamic_alias = imported[1] == 'importlib' and imported[2] == 'import_module'
                if isinstance(node.func, ast.Attribute) and isinstance(node.func.value, ast.Name) and node.func.value.id in info['imports']:
                    imported = info['imports'][node.func.value.id]
                    dynamic_alias = imported[1] == 'importlib' and node.func.attr == 'import_module'
                if dynamic_alias or callee in {'__import__', 'importlib.import_module', 'eval', 'exec', 'getattr', 'setattr'}:
                    diag('PY_DYNAMIC_REFERENCE', 'Runtime import or attribute behavior: '+callee, info['file'], node, 'dynamic')
            elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name == '__getattr__':
                diag('PY_DYNAMIC_EXPORTS', 'Runtime __getattr__ exports cannot be proven', info['file'], node, 'dynamic')
    result['symbols'] = list(symbol_map.values())
    return result


if __name__ == '__main__':
    try:
        print(json.dumps(dict(version=1, snapshot=analyze(json.load(sys.stdin))), sort_keys=True))
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(2)
