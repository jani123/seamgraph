/* Save and edit comments in place without replacing the source workspace. */
(() => {
  const dialog=document.querySelector('#comment-composer');
  if(!dialog)return;
  const form=dialog.querySelector('form'),fields=form.elements;
  const originalFile=fields.selectedFile.value;
  let trigger,records=[];
  const sideName=side=>side==='before'?'Base':'Head';
  const anchorLabel=record=>record.line && record.line!=='0'?`${sideName(record.side)} line ${record.line}`:'File comment';
  const readRecords=()=>{records=[...document.querySelectorAll('[data-comment-record]')].map(node=>({...node.dataset}));};
  const matchesAnchor=record=>record.file===fields.file.value && record.side===fields.side.value && record.line===(fields.line.value||'0');
  const choose=record=>{
    fields.commentId.value=record?.id||'';
    fields.commentVersion.value=record?.version||'';
    fields.note.value=record?.note||'';
    dialog.querySelector('#comment-title').textContent=record?'Edit comment':'Add comment';
    dialog.querySelector('[data-comment-new]').hidden=!record || record.status!=='current';
    dialog.querySelector('[data-comment-delete]').hidden=!record;
    const preview=dialog.querySelector('[data-comment-preview]');
    if(record){
      preview.textContent=`Original comparison: ${record.base} → ${record.head}\n${record.status}\n${record.context}`;
      preview.hidden=false;
    }
  };
  const showExisting=()=>{
    const list=dialog.querySelector('[data-comment-existing]');list.replaceChildren();
    for(const record of records.filter(matchesAnchor)){
      const article=document.createElement('article'),note=document.createElement('p'),edit=document.createElement('button');
      note.textContent=record.note;edit.type='button';edit.dataset.commentEdit=record.id;edit.textContent='Edit this comment';
      article.append(note,edit);list.append(article);
    }
  };
  const open=(button,record)=>{
    trigger=button;
    fields.file.value=record?.file||originalFile;
    fields.side.value=record?.side||button.dataset.commentSide||'';
    fields.line.value=record?.line||button.dataset.commentLine||'';
    dialog.querySelector('[data-comment-anchor]').textContent=fields.file.value+' · '+anchorLabel({side:fields.side.value,line:fields.line.value});
    const preview=dialog.querySelector('[data-comment-preview]');
    preview.textContent=button.closest('.diff-row, .split-cell')?.querySelector('[data-diff-code]')?.textContent||'';
    preview.hidden=!fields.line.value;
    dialog.querySelector('[data-comment-error]').hidden=true;
    choose(record);showExisting();
    if(!dialog.open)dialog.showModal();
    fields.note.focus({preventScroll:true});
  };
  const decorate=()=>{
    readRecords();
    const buttons=[...document.querySelectorAll('[data-comment-line]')],anchors=new Map(),visible=new Set();
    for(const record of records){
      if(record.file!==originalFile || record.status!=='current')continue;
      const key=record.side+':'+record.line;
      if(!anchors.has(key))anchors.set(key,[]);
      anchors.get(key).push(record);
    }
    for(const button of buttons){
      visible.add(button.dataset.commentSide+':'+button.dataset.commentLine);
      button.dataset.originalLabel ||= button.getAttribute('aria-label');
      const comments=anchors.get(button.dataset.commentSide+':'+button.dataset.commentLine)||[];
      button.classList.toggle('has-comments',!!comments.length);
      button.dataset.commentCount=String(comments.length);
      button.setAttribute('aria-label',comments.length?`Review ${comments.length} comment${comments.length===1?'':'s'} on ${sideName(button.dataset.commentSide)} line ${button.dataset.commentLine}`:button.dataset.originalLabel);
      button.title=comments.length?comments.map(record=>record.note).join('\n\n'):'Add a comment on this line';
    }
    document.querySelector('[data-file-comment-indicators]')?.remove();
    const header=document.querySelector('.code-source-header');
    if(!header)return;
    const indicators=document.createElement('span');indicators.dataset.fileCommentIndicators='';
    for(const record of records.filter(record=>record.file===originalFile && (record.line==='0' || record.status!=='current' || !visible.has(record.side+':'+record.line)))){
      const button=document.createElement('button');button.type='button';button.dataset.commentEdit=record.id;
      button.textContent=`💬 ${anchorLabel(record)}${record.status==='current'?'':' · '+record.status}`;button.title=record.note;indicators.append(button);
    }
    header.append(indicators);
  };
  document.addEventListener('click',event=>{
    if(event.target.closest('[data-comment-close]')){dialog.close();return;}
    if(event.target.closest('[data-comment-new]')){choose();fields.note.focus();return;}
    const edit=event.target.closest('[data-comment-edit]');
    if(edit){const record=records.find(item=>item.id===edit.dataset.commentEdit);if(record)open(edit,record);return;}
    const button=event.target.closest('[data-comment-open], [data-comment-line]');
    if(!button)return;
    const comments=records.filter(record=>record.file===originalFile && record.status==='current' && record.side===(button.dataset.commentSide||'') && record.line===(button.dataset.commentLine||'0'));
    open(button,comments.length===1?comments[0]:undefined);
  });
  form.addEventListener('submit',async event=>{
    event.preventDefault();
    const submit=form.querySelector('[type=submit]'),error=dialog.querySelector('[data-comment-error]');
    if(submit.disabled)return;
    const deleting=event.submitter?.hasAttribute('data-comment-delete');
    form.querySelectorAll('[type=submit]').forEach(button=>button.disabled=true);
    submit.textContent=deleting?'Deleting…':'Saving…';error.hidden=true;
    try{
      const body=new URLSearchParams(new FormData(form));body.set('returnTo',location.pathname+location.search);
      if(deleting)body.set('deleteComment','1');
      const response=await fetch(form.action,{method:'POST',body,headers:{'X-SeamGraph-Comment':'1'}});
      if(!response.ok)throw Error(await response.text());
      const documentUpdate=new DOMParser().parseFromString(await response.text(),'text/html');
      for(const selector of ['[data-comment-state]','.change-comments']){
        const replacement=documentUpdate.querySelector(selector);
        if(!replacement)throw Error('The comment was saved, but its updated state is unavailable. Refresh before editing.');
        document.querySelector(selector).replaceWith(replacement);
      }
      decorate();dialog.close();
      if(trigger?.isConnected)trigger.focus({preventScroll:true});
    }catch(reason){error.textContent=reason.message;error.hidden=false;}
    finally{form.querySelectorAll('[type=submit]').forEach(button=>button.disabled=false);submit.textContent='Save comment';}
  });
  decorate();
})();
