/* Graph projections retain original relation IDs; layouts never change evidence. */
(function (root) {
  function category(relation, symbols, construction) {
    if (construction.has(relation.toId)) return "construction";
    if (relation.kind === "imports") return "imports";
    const symbol = symbols.get(relation.toId);
    if (relation.kind === "calls" || ["function", "method"].includes(symbol?.kind) ||
        (symbol?.kind === "field" && /\bfunc\(/.test(symbol.displaySignature))) return "api";
    return "data";
  }

  function project(snapshot, violations = []) {
    const owners = new Map(snapshot.modules.map(m => [m.id, m.id]));
    for (const x of [...snapshot.seams, ...snapshot.symbols]) owners.set(x.id, x.moduleId);
    const symbols = new Map(snapshot.symbols.map(s => [s.id, s]));
    const construction = new Set();
    const ordinary = new Set();
    // An explicitly construction-only module exports function factories, not
    // adapter types with operational methods. Its outgoing references are wiring.
    const wiring = new Set(snapshot.modules.filter(m => {
      const seams = snapshot.seams.filter(s => s.moduleId === m.id);
      return seams.length > 0 && seams.every(s => s.id.endsWith(".construction") &&
        s.contractMembers.length > 0 && s.contractMembers.every(id => symbols.get(id)?.kind === "function"));
    }).map(m => m.id));
    for (const seam of snapshot.seams) {
      const target = seam.id.endsWith(".construction") ? construction : ordinary;
      for (const id of seam.contractMembers) target.add(id);
    }
    // A member shared with an operational seam is not construction-only.
    for (const id of ordinary) construction.delete(id);
    const violated = new Set(violations.map(v => v.relationId));
    const pairs = new Map();
    for (const relation of snapshot.relations) {
      const from = owners.get(relation.fromId), to = owners.get(relation.toId);
      if (!from || !to || from === to || !["imports", "uses", "calls"].includes(relation.kind)) continue;
      const id = from + "→" + to;
      if (!pairs.has(id)) pairs.set(id, {id, source: from, target: to, count: 0, violated: false, categories: [], relationIds: []});
      const pair = pairs.get(id), kind = wiring.has(from) ? "construction" : category(relation, symbols, construction);
      pair.count++;
      pair.violated ||= violated.has(relation.id);
      if (!pair.categories.includes(kind)) pair.categories.push(kind);
      pair.relationIds.push(relation.id);
    }
    return {owners, pairs};
  }

  // Collapse strongly connected components before assigning dependency layers.
  // Cycles remain visible in one layer; no edge is removed to make a DAG.
  function layout(ids, edges) {
    const nodes = [...new Set(ids)].sort(), outgoing = new Map(nodes.map(id => [id, []]));
    for (const edge of edges) if (outgoing.has(edge.source) && outgoing.has(edge.target)) outgoing.get(edge.source).push(edge.target);
    for (const [id, nexts] of outgoing) outgoing.set(id, [...new Set(nexts)].sort());
    const index = new Map(), low = new Map(), stack = [], active = new Set(), components = [];
    let cursor = 0;
    function visit(id) {
      index.set(id, cursor); low.set(id, cursor++); stack.push(id); active.add(id);
      for (const next of outgoing.get(id).sort()) {
        if (!index.has(next)) {visit(next); low.set(id, Math.min(low.get(id), low.get(next)));}
        else if (active.has(next)) low.set(id, Math.min(low.get(id), index.get(next)));
      }
      if (low.get(id) === index.get(id)) {
        const group = []; let next;
        do {next = stack.pop(); active.delete(next); group.push(next);} while (next !== id);
        components.push(group.sort());
      }
    }
    for (const id of nodes) if (!index.has(id)) visit(id);
    const owner = new Map(); components.forEach((group, i) => group.forEach(id => owner.set(id, i)));
    const targets = components.map(() => new Set()), incoming = components.map(() => 0), ranks = components.map(() => 0);
    for (const [id, nexts] of outgoing) for (const next of nexts) {
      const a = owner.get(id), b = owner.get(next);
      if (a !== b && !targets[a].has(b)) {targets[a].add(b); incoming[b]++;}
    }
    const queue = incoming.flatMap((count, i) => count === 0 ? [i] : []);
    for (let i = 0; i < queue.length; i++) for (const target of targets[queue[i]]) {
      ranks[target] = Math.max(ranks[target], ranks[queue[i]] + 1);
      if (--incoming[target] === 0) queue.push(target);
    }
    // Keep cycle members in a single movable block. Virtual vertices split long
    // dependencies into adjacent-layer segments for crossing minimization only.
    const layers = Array.from({length: ranks.length ? Math.max(...ranks) + 1 : 0}, () => []);
    const vertices = components.map((members, i) => ({key: members[0], members, rank: ranks[i], before: [], after: []}));
    for (const vertex of vertices) layers[vertex.rank].push(vertex);
    const strips = layers.map(() => []);
    let virtualNodes = 0;
    function connect(source, target) {
      source.after.push(target); target.before.push(source);
      strips[source.rank].push({source, target});
    }
    for (const [i, nexts] of targets.entries()) for (const next of nexts) {
      const source = vertices[i], target = vertices[next];
      let previous = source;
      for (let rank = source.rank + 1; rank < target.rank; rank++) {
        const point = {key: source.key + "\0" + target.key + "\0" + rank, members: [], rank, before: [], after: []};
        layers[rank].push(point); connect(previous, point); previous = point; virtualNodes++;
      }
      connect(previous, target);
    }
    for (const layer of layers) layer.sort(byKey);
    const alphabeticalCrossings = crossings(layers, strips);
    let best = layers.map(layer => [...layer]), bestScore = alphabeticalCrossings;
    // Barycenter sweeps can temporarily worsen an arrangement. Always retain
    // the best complete ordering rather than returning the last sweep.
    for (let pass = 0; pass < 6 && bestScore > 0; pass++) {
      for (const forward of [true, false]) {
        const order = layers.map((_, i) => i);
        if (!forward) order.reverse();
        const coordinates = rowCenters(layers);
        for (const rank of order) {
          const neighbors = forward ? "before" : "after";
          const scores = new Map(layers[rank].map(v => [v, v[neighbors].length
            ? mean(v[neighbors].map(n => coordinates.get(n))) : coordinates.get(v)]));
          layers[rank].sort((a, b) => scores.get(a) - scores.get(b) || byKey(a, b));
          updateCenters(layers[rank], coordinates);
        }
        const score = crossings(layers, strips);
        if (score < bestScore) {best = layers.map(layer => [...layer]); bestScore = score;}
      }
    }
    // Adjacent swaps inspect just the two affected strips. Bound the work even
    // when a dense graph creates many virtual vertices.
    const ordered = best.map(layer => [...layer]);
    const indices = new Map(ordered.flatMap(layer => layer.map((v, i) => [v, i])));
    let attempts = 0;
    for (let pass = 0; pass < 3 && bestScore > 0 && attempts < 2000; pass++) {
      let improved = false;
      for (let rank = 0; rank < ordered.length && attempts < 2000; rank++) {
        const layer = ordered[rank];
        for (let i = 0; i + 1 < layer.length && attempts < 2000; i++, attempts++) {
          const a = layer[i], b = layer[i + 1];
          let delta = 0;
          // Swapping adjacent vertices changes only crossings between their
          // own incident segments. Shared endpoints contribute zero.
          for (const side of ["before", "after"]) for (const p of a[side]) for (const q of b[side])
            delta += Math.sign(indices.get(q) - indices.get(p));
          if (delta < 0) {
            [layer[i], layer[i + 1]] = [b, a];
            indices.set(b, i); indices.set(a, i + 1);
            bestScore += delta; improved = true;
          }
        }
      }
      if (!improved) break;
    }
    const coordinates = align(ordered);
    const result = new Map();
    for (const vertex of vertices) vertex.members.forEach((id, row) => result.set(id, {
      x: vertex.rank * 240,
      y: coordinates.get(vertex) + (row - (vertex.members.length - 1) / 2) * 92,
    }));
    const ys = [...result.values()].map(p => p.y);
    const center = ys.length ? (Math.min(...ys) + Math.max(...ys)) / 2 : 0;
    const positions = new Map(nodes.map(id => [id, {x: result.get(id).x, y: result.get(id).y - center}]));
    return {positions, crossings: bestScore, alphabeticalCrossings, virtualNodes};
  }

  function byKey(a, b) {return a.key < b.key ? -1 : a.key > b.key ? 1 : 0;}
  function mean(values) {return values.reduce((sum, value) => sum + value, 0) / values.length;}
  function height(vertex) {return vertex.members.length || 0.25;}
  function updateCenters(layer, coordinates) {
    let row = -layer.reduce((sum, v) => sum + height(v), 0) / 2;
    for (const vertex of layer) {coordinates.set(vertex, row + height(vertex) / 2); row += height(vertex);}
  }
  function rowCenters(layers) {
    const result = new Map();
    for (const layer of layers) updateCenters(layer, result);
    return result;
  }

  // Count strict inversions with a Fenwick tree. Shared endpoints never count
  // as crossings; a source group is queried before any of its edges are added.
  function stripCrossings(layers, strips, rank) {
    if (rank < 0 || rank + 1 >= layers.length || !strips[rank].length) return 0;
    const sources = new Map(layers[rank].map((v, i) => [v, i]));
    const targets = new Map(layers[rank + 1].map((v, i) => [v, i]));
    const edges = strips[rank].map(e => [sources.get(e.source), targets.get(e.target)])
      .sort((a, b) => a[0] - b[0] || a[1] - b[1]);
    const tree = new Array(targets.size + 1).fill(0);
    let count = 0, seen = 0;
    function prefix(index) {let sum = 0; for (let i = index + 1; i > 0; i -= i & -i) sum += tree[i]; return sum;}
    for (let start = 0; start < edges.length;) {
      let end = start;
      while (end < edges.length && edges[end][0] === edges[start][0]) end++;
      for (let i = start; i < end; i++) count += seen - prefix(edges[i][1]);
      for (let i = start; i < end; i++) {
        for (let j = edges[i][1] + 1; j < tree.length; j += j & -j) tree[j]++;
        seen++;
      }
      start = end;
    }
    return count;
  }
  function crossings(layers, strips) {return strips.reduce((sum, _, rank) => sum + stripCrossings(layers, strips, rank), 0);}

  // Weighted isotonic regression finds the closest neighbor-aligned positions
  // subject to the chosen order and a minimum gap. Cycles stay contiguous.
  function align(layers) {
    const coordinates = rowCenters(layers);
    for (const [vertex, row] of coordinates) coordinates.set(vertex, row * 92);
    for (let pass = 0; pass < 4; pass++) for (const forward of [true, false]) {
      const order = layers.map((_, i) => i);
      if (!forward) order.reverse();
      for (const rank of order) {
        const layer = layers[rank], offsets = [], blocks = [];
        for (let i = 0; i < layer.length; i++) {
          const vertex = layer[i], neighbors = [...vertex.before, ...vertex.after];
          const desired = neighbors.length ? mean(neighbors.map(n => coordinates.get(n))) : coordinates.get(vertex);
          offsets[i] = i ? offsets[i - 1] + (height(layer[i - 1]) + height(vertex)) * 46 : 0;
          const weight = vertex.members.length || 0.1;
          blocks.push({start: i, end: i, sum: (desired - offsets[i]) * weight, weight});
          while (blocks.length > 1) {
            const a = blocks[blocks.length - 2], b = blocks[blocks.length - 1];
            if (a.sum / a.weight <= b.sum / b.weight) break;
            blocks.splice(-2, 2, {start: a.start, end: b.end, sum: a.sum + b.sum, weight: a.weight + b.weight});
          }
        }
        for (const block of blocks) for (let i = block.start; i <= block.end; i++)
          coordinates.set(layer[i], block.sum / block.weight + offsets[i]);
      }
    }
    return coordinates;
  }
  function positions(ids, edges) {return layout(ids, edges).positions;}
  // Route original edges through clear horizontal/vertical corridors. Ports are
  // shared by related edges so fan-in and fan-out form common trunks.
  function orthogonal(nodes, edges) {
    const boxes = new Map(nodes.map(n => [n.id, {...n, width: n.width || 130, height: n.height || 56}]));
    const valid = edges.filter(e => boxes.has(e.source) && boxes.has(e.target) && e.source !== e.target)
      .map(e => ({...e, id: e.id || e.source + "→" + e.target}));
    const ports = new Map(), groups = new Map();
    const compare = (a,b) => a < b ? -1 : a > b ? 1 : 0;
    for (const edge of valid) {
      const source = boxes.get(edge.source), target = boxes.get(edge.target);
      const direction = Math.sign(target.x - source.x);
      const sourceSide = direction || (target.y >= source.y ? 1 : -1);
      const targetSide = direction ? -direction : sourceSide;
      for (const [role, box, side, other] of [["source",source,sourceSide,target],["target",target,targetSide,source]]) {
        if (!groups.has(box.id)) groups.set(box.id, new Map());
        const sides = groups.get(box.id);
        if (!sides.has(side)) sides.set(side, []);
        sides.get(side).push({edge, role, box, side, other});
      }
    }
    for (const sides of groups.values()) for (const group of sides.values()) {
      group.sort((a,b) => a.other.y - b.other.y || compare(a.edge.id,b.edge.id) || compare(a.role,b.role));
      group.forEach((entry,i) => {
        if (!ports.has(entry.edge.id)) ports.set(entry.edge.id, {});
        ports.get(entry.edge.id)[entry.role] = {side: entry.side,
          offset: 0,
          lead: 28 + (compare(entry.edge.source,entry.edge.target)>0 && valid.some(e => e.source===entry.edge.target && e.target===entry.edge.source) ? 12 : 0)};
      });
    }
    const obstacles = [...boxes.values()].map(n => ({id:n.id,left:n.x-n.width/2-8,right:n.x+n.width/2+8,top:n.y-n.height/2-8,bottom:n.y+n.height/2+8}));
    const rows = [...new Set(obstacles.flatMap(b => [b.top-6,b.bottom+6]))].sort((a,b)=>a-b);
    const routed = [], result = new Map();
    const span = e => Math.abs(boxes.get(e.source).x-boxes.get(e.target).x);
    valid.sort((a,b)=>span(a)-span(b)||compare(a.id,b.id));
    for (const edge of valid) {
      const source = boxes.get(edge.source), target = boxes.get(edge.target), assigned = ports.get(edge.id);
      let best = null, bestCost = Infinity;
      const related = routed.filter(r => r.edge.source === edge.source || r.edge.target === edge.target);
      const sharedRows = routed.flatMap(r => r.points.map(p => p.y));
      const sharedColumns = [...new Set(routed.flatMap(r => r.points.map(p => p.x)))];
      // Prefer flow-facing sides; opposite sides handle back edges, cycles and
      // expanded seam nodes without changing the dependency direction.
      for (const [sourceSide,targetSide,penalty] of [
        [assigned.source.side,assigned.target.side,0],
        [-assigned.source.side,assigned.target.side,80],
        [assigned.source.side,-assigned.target.side,80],
        [-assigned.source.side,-assigned.target.side,160],
      ]) {
        const start = {x:source.x+sourceSide*source.width/2,y:source.y+assigned.source.offset};
        const end = {x:target.x+targetSide*target.width/2,y:target.y+assigned.target.offset};
        const from = {x:start.x+sourceSide*assigned.source.lead,y:start.y};
        const to = {x:end.x+targetSide*assigned.target.lead,y:end.y};
        const candidates = [...new Set([start.y,end.y,(start.y+end.y)/2,...sharedRows,...rows])]
          .map(row => [start,from,{x:from.x,y:row},{x:to.x,y:row},to,end]);
        // Reuse vertical corridors even when the original connections have
        // different endpoints. The entry and exit branches remain separate.
        for (const x of sharedColumns) candidates.push([
          start,from,{x,y:from.y},{x,y:to.y},to,end,
        ]);
        // Join an existing destination route upstream, then follow its whole
        // suffix. This allows sources in different columns to share a trunk
        // instead of only meeting at the final attachment point.
        for (const route of related) {
          if (route.edge.target !== edge.target) continue;
          const last=route.points[route.points.length-1];
          if (last.x!==end.x || last.y!==end.y) continue;
          for (let j=1;j<route.points.length-1;j++) {
            const join=route.points[j],suffix=route.points.slice(j);
            candidates.push([start,from,{x:from.x,y:join.y},...suffix]);
            candidates.push([start,from,{x:join.x,y:from.y},...suffix]);
          }
        }
        for (const candidate of candidates) {
          const points = simplify(candidate);
          if (!clearPath(points,obstacles,edge.source,edge.target)) continue;
          let cost = penalty + (points.length-2)*20;
          for (let i=1;i<points.length;i++) {
            const a=points[i-1], b=points[i];
            const length=Math.abs(b.x-a.x)+Math.abs(b.y-a.y);
            // Reusing a related corridor costs less than opening another one.
            // Cap the discount so long detours never win merely by overlapping.
            let discount=0;
            for (const route of routed) for (let j=1;j<route.points.length;j++) {
              const c=route.points[j-1],d=route.points[j];
              const preference=route.edge.source===edge.source || route.edge.target===edge.target ? 0.65 : 0.45;
              let overlap=0;
              if (a.y===b.y && c.y===d.y && a.y===c.y && (b.x-a.x)*(d.x-c.x)>0)
                overlap=Math.max(overlap,Math.max(0,Math.min(Math.max(a.x,b.x),Math.max(c.x,d.x))-Math.max(Math.min(a.x,b.x),Math.min(c.x,d.x))));
              if (a.x===b.x && c.x===d.x && a.x===c.x && (b.y-a.y)*(d.y-c.y)>0)
                overlap=Math.max(overlap,Math.max(0,Math.min(Math.max(a.y,b.y),Math.max(c.y,d.y))-Math.max(Math.min(a.y,b.y),Math.min(c.y,d.y))));
              discount=Math.max(discount,overlap*preference);
            }
            cost += length - discount;
          }
          if (cost < bestCost) {bestCost=cost;best={points,sourceSide,targetSide};}
        }
        if (best && penalty === 0) break;
      }
      // Simple corridor candidates can all be blocked. Search the obstacle
      // boundary grid instead of drawing an unchecked fallback through nodes.
      if (!best) {
        for (const [sourceSide,targetSide] of [
          [assigned.source.side,assigned.target.side],[-assigned.source.side,assigned.target.side],
          [assigned.source.side,-assigned.target.side],[-assigned.source.side,-assigned.target.side],
        ]) {
          const start={x:source.x+sourceSide*source.width/2,y:source.y};
          const end={x:target.x+targetSide*target.width/2,y:target.y};
          const from={x:start.x+sourceSide*28,y:start.y},to={x:end.x+targetSide*28,y:end.y};
          const middle=obstacleRoute(from,to,obstacles);
          if (!middle) continue;
          const points=simplify([start,...middle,end]);
          if (clearPath(points,obstacles,edge.source,edge.target)) {best={points,sourceSide,targetSide};break;}
        }
      }
      // Overlapping nodes can make routing impossible. Never display a path
      // through a box; retain the edge record with an explicit hidden route.
      if (!best) {result.set(edge.id,{blocked:true});continue;}
      routed.push({edge,points:best.points});
      const dx=target.x-source.x,dy=target.y-source.y,length2=dx*dx+dy*dy;
      if (length2<1e-8) {result.set(edge.id,{blocked:true});continue;}
      const bends=best.points.length>2?best.points.slice(1,-1):[{x:(best.points[0].x+best.points[1].x)/2,y:(best.points[0].y+best.points[1].y)/2}];
      result.set(edge.id,{points:best.points,
        sourceEndpoint:`${best.sourceSide*source.width/2}px ${assigned.source.offset}px`,
        targetEndpoint:`${best.targetSide*target.width/2}px ${assigned.target.offset}px`,
        weights:bends.map(p=>((p.x-source.x)*dx+(p.y-source.y)*dy)/length2),
        distances:bends.map(p=>(dx*(p.y-source.y)-dy*(p.x-source.x))/Math.sqrt(length2)),
      });
    }
    return result;
  }
  function obstacleRoute(start,end,boxes) {
    const xs=[...new Set([start.x,end.x,...boxes.flatMap(b=>[b.left-6,b.right+6])])].sort((a,b)=>a-b);
    const ys=[...new Set([start.y,end.y,...boxes.flatMap(b=>[b.top-6,b.bottom+6])])].sort((a,b)=>a-b);
    const inside=p=>boxes.some(b=>p.x>b.left&&p.x<b.right&&p.y>b.top&&p.y<b.bottom);
    if (inside(start)||inside(end)) return null;
    const key=(x,y)=>y*xs.length+x;
    const sx=xs.indexOf(start.x),sy=ys.indexOf(start.y),goal=key(xs.indexOf(end.x),ys.indexOf(end.y));
    const first=key(sx,sy),costs=new Map([[first,0]]),parents=new Map(),open=[{x:sx,y:sy,id:first,score:0}],closed=new Set();
    while(open.length) {
      open.sort((a,b)=>b.score-a.score||b.id-a.id);
      const current=open.pop();
      if(closed.has(current.id))continue;
      if(current.id===goal){
        const path=[];let id=goal;
        while(id!==undefined){path.push({x:xs[id%xs.length],y:ys[Math.floor(id/xs.length)]});id=parents.get(id);}
        return path.reverse();
      }
      closed.add(current.id);
      const a={x:xs[current.x],y:ys[current.y]};
      for(const [x,y] of [[current.x-1,current.y],[current.x+1,current.y],[current.x,current.y-1],[current.x,current.y+1]]){
        if(x<0||y<0||x>=xs.length||y>=ys.length)continue;
        const id=key(x,y),b={x:xs[x],y:ys[y]};
        if(closed.has(id)||inside(b)||!clearPath([a,b],boxes,null,null))continue;
        const cost=costs.get(current.id)+Math.abs(b.x-a.x)+Math.abs(b.y-a.y);
        if(cost>=(costs.get(id)??Infinity))continue;
        costs.set(id,cost);parents.set(id,current.id);
        open.push({x,y,id,score:cost+Math.abs(b.x-end.x)+Math.abs(b.y-end.y)});
      }
    }
    return null;
  }
  function simplify(points) {
    const result=[];
    for (const point of points) {
      const previous=result[result.length-1];
      if (previous && previous.x===point.x && previous.y===point.y) continue;
      // Keep U-turns: eliminating them could move a route through a node.
      if (result.length>1) {
        const a=result[result.length-2],b=previous;
        if ((a.x===b.x&&b.x===point.x&&(b.y-a.y)*(point.y-b.y)>=0) ||
            (a.y===b.y&&b.y===point.y&&(b.x-a.x)*(point.x-b.x)>=0)) result.pop();
      }
      result.push(point);
    }
    return result;
  }
  function clearPath(points,boxes,source,target) {
    for (let i=1;i<points.length;i++) {
      const a=points[i-1],b=points[i];
      for (const box of boxes) {
        const endpoint=(i===1&&box.id===source)||(i===points.length-1&&box.id===target);
        const bounds=endpoint ? {left:box.left+8,right:box.right-8,top:box.top+8,bottom:box.bottom-8} : box;
        if (a.y===b.y && a.y>bounds.top && a.y<bounds.bottom && Math.max(Math.min(a.x,b.x),bounds.left)<Math.min(Math.max(a.x,b.x),bounds.right)) return false;
        if (a.x===b.x && a.x>bounds.left && a.x<bounds.right && Math.max(Math.min(a.y,b.y),bounds.top)<Math.min(Math.max(a.y,b.y),bounds.bottom)) return false;
      }
    }
    return true;
  }

  const api = {project, positions, layout, orthogonal};
  root.SeamGraphLayout = api;
  if (typeof module !== "undefined") module.exports = api;
})(typeof globalThis === "undefined" ? this : globalThis);
