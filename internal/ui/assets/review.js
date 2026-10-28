/* Local checklist editing and diff search preserve original source anchors. */
(() => {
 for(const form of document.querySelectorAll('.review-checklist')) {
  const text=form.querySelector('textarea');
  const items=document.createElement('div');items.setAttribute('role','group');items.setAttribute('aria-label','Checklist items');
  text.parentElement.before(items);
  const render=()=>{
   items.replaceChildren();
   text.value.split('\n').forEach((line,index)=>{
    const match=/^- \[([ xX])\] (.+)$/.exec(line);if(!match)return;
    const label=document.createElement('label'),box=document.createElement('input');
    box.type='checkbox';box.checked=match[1].toLowerCase()==='x';
    label.append(box,document.createTextNode(match[2]));items.append(label);
    box.addEventListener('change',()=>{const lines=text.value.split('\n');lines[index]=`- [${box.checked?'x':' '}] ${match[2]}`;text.value=lines.join('\n');});
   });
  };
  text.addEventListener('input',render);render();
 }
 const input=document.querySelector('[data-diff-search]');
 if(!input)return;
 input.addEventListener('input',()=>{
  const text=input.value.toLowerCase();let count=0;
  for(const code of document.querySelectorAll('[data-diff-code]')) {
   const match=!!text && code.textContent.toLowerCase().includes(text);
   code.classList.toggle('diff-search-match',match);if(match)count++;
  }
  document.querySelector('[data-diff-matches]').textContent=text?`${count} matching lines`:'';
  document.querySelector('.diff-search-match')?.scrollIntoView({block:'nearest'});
 });
})();

/* Tree glyphs explain their meaning on hover and keyboard focus. */
(() => {
 const tree=document.querySelector('.review-tree');if(!tree)return;
 const tooltip=document.createElement('div');tooltip.id='tree-icon-tooltip';tooltip.className='tree-tooltip';tooltip.setAttribute('role','tooltip');tooltip.hidden=true;document.body.append(tooltip);
 let owner;
 const hide=()=>{tooltip.hidden=true;if(owner?.getAttribute('aria-describedby')===tooltip.id)owner.removeAttribute('aria-describedby');owner=null;};
 const show=node=>{
  hide();tooltip.textContent=node.dataset.treeTooltip;tooltip.hidden=false;
  owner=node.closest('a, summary');owner?.setAttribute('aria-describedby',tooltip.id);
  const rect=node.getBoundingClientRect(),width=tooltip.offsetWidth,height=tooltip.offsetHeight;
  tooltip.style.left=Math.max(8,Math.min(rect.left,innerWidth-width-8))+'px';
  tooltip.style.top=(rect.bottom+height+12>innerHeight?Math.max(8,rect.top-height-6):rect.bottom+6)+'px';
 };
 tree.addEventListener('pointerover',event=>{const node=event.target.closest('[data-tree-tooltip]');if(node)show(node);else hide();});
 tree.addEventListener('pointerleave',hide);
 tree.addEventListener('focusin',event=>{const node=event.target.closest('[data-tree-tooltip]');if(node)show(node);});
 tree.addEventListener('focusout',hide);
 document.addEventListener('keydown',event=>{if(event.key==='Escape')hide();});
 document.addEventListener('click',hide);
 document.addEventListener('scroll',hide,true);
})();
