const {test} = require('node:test');
const assert = require('node:assert/strict');
const {project, positions} = require('../../internal/ui/assets/graph.js');

test('layered layout preserves direction and places cycles together', () => {
  const edges = [{source:'cli',target:'app'},{source:'app',target:'policy'},{source:'policy',target:'model'},
    {source:'a',target:'b'},{source:'b',target:'a'},{source:'b',target:'model'}];
  const p = positions(['model','policy','app','cli','a','b','isolated'],edges);
  assert.ok(p.get('cli').x < p.get('app').x);
  assert.ok(p.get('app').x < p.get('policy').x);
  assert.ok(p.get('policy').x < p.get('model').x);
  assert.equal(p.get('a').x,p.get('b').x);
  assert.notEqual(p.get('a').y,p.get('b').y);
  assert.equal(p.size,7);
  assert.deepEqual(positions(['cli','app','policy','model','isolated','b','a'],edges),p);
});
test('projection separates callable, data, import and construction evidence without dropping violations', () => {
  const s = {
    modules: [{id:'consumer'},{id:'provider'}],
    seams:[{id:'provider.construction',moduleId:'provider',contractMembers:['new','shared']},
      {id:'provider.queries',moduleId:'provider',contractMembers:['read','shared']}],
    symbols:[{id:'new',moduleId:'provider',kind:'function'},{id:'read',moduleId:'provider',kind:'method'},
      {id:'shared',moduleId:'provider',kind:'method'},{id:'value',moduleId:'provider',kind:'field'}],
    relations:[{id:'1',fromId:'consumer',toId:'provider',kind:'imports'},
      {id:'2',fromId:'consumer',toId:'new',kind:'uses'},
      {id:'3',fromId:'consumer',toId:'read',kind:'uses'},
      {id:'4',fromId:'consumer',toId:'value',kind:'uses'},
      {id:'5',fromId:'consumer',toId:'shared',kind:'uses'}]
  };
  const result=project(s,[{relationId:'4'}]).pairs.get('consumer→provider');
  assert.deepEqual(new Set(result.categories),new Set(['imports','construction','api','data']));
  assert.deepEqual(result.relationIds,['1','2','3','4','5']);
  assert.equal(result.count,5);
  assert.equal(result.violated,true);
});

// Independent straight-segment counter for the two-layer geometry and visual
// regressions. Shared endpoints and touching lines are not intersections.
function intersections(points, edges) {
  const turn = (a,b,c) => (b.x-a.x)*(c.y-a.y)-(b.y-a.y)*(c.x-a.x);
  let count = 0;
  for (let i=0;i<edges.length;i++) for(let j=i+1;j<edges.length;j++) {
    const a=edges[i],b=edges[j];
    if ([a.source,a.target].some(id=>id===b.source||id===b.target)) continue;
    const p=points.get(a.source),q=points.get(a.target),r=points.get(b.source),s=points.get(b.target);
    if(turn(p,q,r)*turn(p,q,s)<0 && turn(r,s,p)*turn(r,s,q)<0) count++;
  }
  return count;
}
function alphabetical(points) {
  const layers=new Map(), result=new Map();
  for(const [id,p] of points){if(!layers.has(p.x))layers.set(p.x,[]);layers.get(p.x).push(id);}
  for(const [x,ids] of layers)ids.sort().forEach((id,i)=>result.set(id,{x,y:i*92}));
  return result;
}
const {layout} = require('../../internal/ui/assets/graph.js');

test('neighbor ordering removes reversed-pair crossings and aligns chains', () => {
  const edges=[{source:'a',target:'d'},{source:'b',target:'c'}];
  const result=layout(['a','b','c','d'],edges);
  assert.equal(result.alphabeticalCrossings,1);
  assert.equal(result.crossings,0);
  assert.equal(intersections(result.positions,edges),0);
  const chain=layout(['a','b','c'],[{source:'a',target:'b'},{source:'b',target:'c'}]);
  assert.deepEqual([...chain.positions.values()].map(p=>p.y),[0,0,0]);
  const diamond=layout(['a','b','c','d'],[{source:'a',target:'b'},{source:'a',target:'c'},{source:'b',target:'d'},{source:'c',target:'d'}]);
  assert.equal(diamond.crossings,0);
  assert.equal(diamond.positions.get('a').y,diamond.positions.get('d').y);
});

test('long dependencies influence ordering without adding graph nodes', () => {
  const ids=['a0','a1','a2','b0','b1','b2','c0','c1','c2'];
  const edges=[['a0','b2'],['a1','b1'],['a2','b0'],['b0','c2'],['b1','c1'],['b2','c0'],['a0','c1'],['a2','c1']].map(([source,target])=>({source,target}));
  const result=layout(ids,edges);
  assert.equal(result.virtualNodes,2);
  assert.equal(result.positions.size,ids.length);
  assert.equal(result.crossings,0);
  assert.ok(result.alphabeticalCrossings>result.crossings);
  assert.ok(intersections(result.positions,edges)<intersections(alphabetical(result.positions),edges));
  assert.deepEqual(layout([...ids].reverse(),[...edges].reverse()),result);
});

test('cycles stay contiguous and same-column labels have room', () => {
  const ids=['a','z','m','b','sink'];
  const edges=[['a','z'],['z','a'],['a','sink'],['b','sink'],['m','sink']].map(([source,target])=>({source,target}));
  const {positions:p}=layout(ids,edges);
  assert.equal(p.get('a').x,p.get('z').x);
  const column=ids.filter(id=>p.get(id).x===p.get('a').x).sort((a,b)=>p.get(a).y-p.get(b).y);
  assert.equal(Math.abs(column.indexOf('a')-column.indexOf('z')),1);
  for(let i=1;i<column.length;i++)assert.ok(p.get(column[i]).y-p.get(column[i-1]).y>=92-1e-8);
  assert.deepEqual(layout([...ids,'a'],[...edges,...edges,{source:'missing',target:'sink'}]).positions,p);
  assert.equal(layout([],[]).positions.size,0);
});

test('crossing scores match independent geometry on varied two-layer graphs', () => {
  let seed=19;
  const random=()=>((seed=(Math.imul(seed,1664525)+1013904223)>>>0)/2**32);
  for(let run=0;run<24;run++) {
    const sources=Array.from({length:6},(_,i)=>'a'+i),targets=Array.from({length:6},(_,i)=>'b'+i);
    const edges=sources.flatMap(source=>targets.filter(()=>random()<0.5).map(target=>({source,target})));
    const result=layout([...sources,...targets],edges);
    assert.equal(result.virtualNodes,0);
    assert.equal(result.crossings,intersections(result.positions,edges));
    assert.ok(result.crossings<=result.alphabeticalCrossings);
  }
});

test('100 modules and 500 long dependencies remain bounded and deterministic', () => {
  let seed=7;
  const random=()=>((seed=(Math.imul(seed,1664525)+1013904223)>>>0)/2**32);
  const ids=Array.from({length:100},(_,i)=>String(i).padStart(3,'0')),pairs=new Set();
  while(pairs.size<500){let a=Math.floor(random()*100),b=Math.floor(random()*100);if(a>b)[a,b]=[b,a];if(a!==b)pairs.add(a+','+b);}
  const edges=[...pairs].map(pair=>{const[a,b]=pair.split(',').map(Number);return{source:ids[a],target:ids[b]};});
  const start=performance.now(),result=layout(ids,edges);
  assert.ok(performance.now()-start<2000,'layout should stay interactive on the acceptance graph');
  assert.ok(result.crossings<result.alphabeticalCrossings);
  assert.ok(result.virtualNodes>500);
  assert.equal(result.positions.size,100);
  for(const point of result.positions.values())assert.ok(Number.isFinite(point.x)&&Number.isFinite(point.y));
  assert.deepEqual(layout([...ids].reverse(),[...edges].reverse()),result);
});

const {orthogonal} = require('../../internal/ui/assets/graph.js');
function assertOrthogonal(points) {
  for(let i=1;i<points.length;i++) assert.ok(points[i].x===points[i-1].x||points[i].y===points[i-1].y,'every segment must be horizontal or vertical');
}
function assertClearRoutes(nodes,edges,routes) {
  for(const edge of edges) {
    const route=routes.get(edge.id);assert.ok(route,'original edge must retain a route');assertOrthogonal(route.points);
    for(let i=1;i<route.points.length;i++) {
      const a=route.points[i-1],b=route.points[i];
      for(const n of nodes) {
        const left=n.x-n.width/2,right=n.x+n.width/2,top=n.y-n.height/2,bottom=n.y+n.height/2;
        const horizontal=a.y===b.y&&a.y>top&&a.y<bottom&&Math.max(Math.min(a.x,b.x),left)<Math.min(Math.max(a.x,b.x),right);
        const vertical=a.x===b.x&&a.x>left&&a.x<right&&Math.max(Math.min(a.y,b.y),top)<Math.min(Math.max(a.y,b.y),bottom);
        assert.ok(!horizontal&&!vertical,`${edge.id} must not pass through ${n.id}`);
      }
    }
  }
}
test('orthogonal routes avoid intermediate boxes and bundle shared ports',()=>{
  const nodes=[{id:'a',x:0,y:0,width:132,height:58},{id:'blocker',x:360,y:0,width:132,height:58},
    {id:'b',x:720,y:0,width:132,height:58},{id:'c',x:720,y:140,width:132,height:58}];
  const edges=[{id:'ab',source:'a',target:'b'},{id:'ac',source:'a',target:'c'}];
  const routes=orthogonal(nodes,edges);
  assertClearRoutes(nodes,edges,routes);
  assert.equal(routes.get('ab').sourceEndpoint,routes.get('ac').sourceEndpoint);
  assert.deepEqual(routes.get('ab').points.slice(0,2),routes.get('ac').points.slice(0,2));
  assert.deepEqual(orthogonal([...nodes].reverse(),[...edges].reverse()),routes);
  // Independently reconstruct Cytoscape's segment coordinates from the styles.
  for(const edge of edges){const r=routes.get(edge.id),a=nodes.find(n=>n.id===edge.source),b=nodes.find(n=>n.id===edge.target);
    const dx=b.x-a.x,dy=b.y-a.y,length=Math.hypot(dx,dy);
    const actual=r.weights.map((w,i)=>({x:a.x+w*dx-dy/length*r.distances[i],y:a.y+w*dy+dx/length*r.distances[i]}));
    const expected=r.points.slice(1,-1);
    assert.equal(actual.length,expected.length);
    actual.forEach((p,i)=>{assert.ok(Math.abs(p.x-expected[i].x)<1e-8);assert.ok(Math.abs(p.y-expected[i].y)<1e-8);});
  }
});
test('orthogonal cycles and expanded seams use distinct clear corridors',()=>{
  const nodes=[{id:'a',x:0,y:0,width:132,height:58},{id:'b',x:0,y:140,width:132,height:58},
    {id:'seam',x:85,y:75,width:132,height:46}];
  const edges=[{id:'ab',source:'a',target:'b'},{id:'ba',source:'b',target:'a'},{id:'as',source:'a',target:'seam'}];
  const routes=orthogonal(nodes,edges);
  assertClearRoutes(nodes,edges,routes);
  assert.notDeepEqual(routes.get('ab').points,[...routes.get('ba').points].reverse());
});

test('fan-in shares a destination trunk while retaining individual edge IDs',()=>{
  const nodes=[{id:'a',x:0,y:0,width:130,height:56},{id:'b',x:0,y:160,width:130,height:56},
    {id:'provider',x:360,y:80,width:130,height:56}];
  const edges=[{id:'ap',source:'a',target:'provider'},{id:'bp',source:'b',target:'provider'}];
  const routes=orthogonal(nodes,edges);
  assertClearRoutes(nodes,edges,routes);
  assert.equal(routes.size,2);
  assert.deepEqual(routes.get('ap').points.slice(-2),routes.get('bp').points.slice(-2));
  assert.deepEqual(orthogonal([...nodes].reverse(),[...edges].reverse()),routes);
});

test('sources in different columns merge upstream of their destination',()=>{
  const nodes=[{id:'a',x:0,y:240,width:130,height:56},{id:'b',x:300,y:160,width:130,height:56},
    {id:'provider',x:720,y:80,width:130,height:56}];
  const edges=[{id:'ap',source:'a',target:'provider'},{id:'bp',source:'b',target:'provider'}];
  const routes=orthogonal(nodes,edges);
  assertClearRoutes(nodes,edges,routes);
  // Both routes must use the vertical trunk from y=160 down to y=80,
  // as well as the short final horizontal attachment.
  for(const route of routes.values()) assert.ok(route.points.some((p,i)=>{
    const next=route.points[i+1];
    return next && p.x===627 && next.x===627 && Math.min(p.y,next.y)<=80 && Math.max(p.y,next.y)>=160;
  }));
  assert.deepEqual(orthogonal([...nodes].reverse(),[...edges].reverse()),routes);
});

test('independent connections reuse a nearby vertical corridor',()=>{
  const nodes=[{id:'a',x:0,y:0},{id:'b',x:360,y:200},{id:'c',x:20,y:40},{id:'d',x:380,y:240}]
    .map(n=>({...n,width:20,height:20}));
  const edges=[{id:'ab',source:'a',target:'b'},{id:'cd',source:'c',target:'d'}];
  const routes=orthogonal(nodes,edges);
  assertClearRoutes(nodes,edges,routes);
  const vertical=id=>routes.get(id).points.flatMap((p,i,points)=>{
    const next=points[i+1];return next&&p.x===next.x?[{x:p.x,top:Math.min(p.y,next.y),bottom:Math.max(p.y,next.y)}]:[];
  });
  assert.ok(vertical('ab').some(a=>vertical('cd').some(b=>a.x===b.x&&Math.min(a.bottom,b.bottom)-Math.max(a.top,b.top)>=160)));
  assert.deepEqual(orthogonal([...nodes].reverse(),[...edges].reverse()),routes);
});


test('bundled connections never double back through their endpoint boxes',()=>{
  const nodes=[{id:'bootstrap',x:0,y:80},{id:'files',x:240,y:30},
    {id:'python',x:240,y:160},{id:'analysis',x:540,y:40},
    {id:'model',x:820,y:220}].map(n=>({...n,width:130,height:56}));
  const edges=[{id:'bf',source:'bootstrap',target:'files'},{id:'ba',source:'bootstrap',target:'analysis'},
    {id:'pa',source:'python',target:'analysis'},{id:'fm',source:'files',target:'model'},
    {id:'am',source:'analysis',target:'model'}];
  assertClearRoutes(nodes,edges,orthogonal(nodes,edges));
});

test('blocked corridors use obstacle search and overlapping endpoints never draw through boxes',()=>{
  const nodes=[{id:'a',x:0,y:0,width:130,height:56},{id:'b',x:600,y:0,width:130,height:56},
    {id:'left-wall',x:120,y:0,width:30,height:200},{id:'right-wall',x:480,y:0,width:30,height:300},
    {id:'middle',x:300,y:0,width:130,height:260}];
  const edges=[{id:'ab',source:'a',target:'b'}];
  assertClearRoutes(nodes,edges,orthogonal(nodes,edges));
  const overlapping=[{id:'a',x:0,y:0,width:130,height:56},{id:'b',x:10,y:0,width:130,height:56}];
  assertClearRoutes(overlapping,edges,orthogonal(overlapping,edges));
  overlapping[1].x=0;
  assert.equal(orthogonal(overlapping,edges).get('ab').blocked,true);
});
