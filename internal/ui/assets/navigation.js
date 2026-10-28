/* Shared comparison controls and native navigation feedback. */
(() => {
  const comparisonBusy = (form, busy) => {
    if (!form?.matches("[data-comparison-controls]")) return;
    form.setAttribute("aria-busy", String(busy));
    form.querySelector("button").disabled = busy || form.dataset.branchesUnavailable === "true";
    form.querySelector("[data-comparison-loading]").hidden = !busy;
  };
  document.addEventListener("htmx:beforeRequest", event => comparisonBusy(event.detail.elt, true));
  document.addEventListener("htmx:afterRequest", event => comparisonBusy(event.detail.elt, false));
  document.querySelectorAll("[data-comparison-controls]").forEach(form => {
    const scope=form.elements.scope,base=form.querySelector('[data-revision-field="base"]'),head=form.querySelector('[data-revision-field="head"]');
    const target=form.querySelector('[data-branch-picker="base"]'),review=form.querySelector('[data-branch-picker="head"]');
    let revisions=scope.value==='working'?['HEAD~1','HEAD']:[base.value,head.value];
    const update=()=>{
      const working=scope.value==='working',branches=scope.value==='branches'||scope.value==='target-tip';
      if(working){base.value='HEAD';head.value='WORKTREE';}
      else if(!branches){[base.value,head.value]=revisions;}
      for(const input of [base,head]){input.hidden=branches;input.disabled=branches;input.readOnly=working;}
      form.querySelectorAll('[data-branch-field]').forEach(label=>label.hidden=!branches);
      for(const select of [target,review]){if(select)select.disabled=!branches || form.dataset.branchCount==='0';}
      const unavailable=branches && (!target || target.disabled || !review || review.disabled);
      form.dataset.branchesUnavailable=String(unavailable);form.querySelector('button').disabled=unavailable;
      const message=form.parentElement.querySelector('[data-branch-unavailable]');if(message)message.hidden=!unavailable;
    };
    scope.addEventListener('change',update);
    form.addEventListener('input',event=>{if((event.target===base||event.target===head)&&scope.value==='revisions')revisions=[base.value,head.value];});
    for(const select of [target,review]){select?.addEventListener('change',()=>{revisions=[target.value,review.value];});}
    update();
  });
  const indicator = document.querySelector("#view-loading");
  const reset = () => {
    indicator.hidden = true;
    document.querySelectorAll("[data-comparison-controls]").forEach(form => comparisonBusy(form, false));
    document.querySelector("main")?.removeAttribute("aria-busy");
  };
  const show = (event) => {
    // Other handlers may turn a link or form into an in-page interaction.
    setTimeout(() => {
      if (event.defaultPrevented) return;
      comparisonBusy(event.target, true);
      indicator.hidden = false;
      document.querySelector("main")?.setAttribute("aria-busy", "true");
    }, 0);
  };
  const navigatesHere = (element, address) => {
    const target = element.getAttribute("target");
    if (target && target !== "_self") return false;
    const destination = new URL(address, location.href);
    if (destination.origin !== location.origin) return false;
    if (destination.searchParams.has("export")) return false;
    return !(destination.hash && destination.pathname === location.pathname &&
      destination.search === location.search);
  };
  document.addEventListener("click", (event) => {
    if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    const link = event.target.closest("a[href]");
    if (!link || link.hasAttribute("download") || !navigatesHere(link, link.href)) return;
    show(event);
  });
  document.addEventListener("submit", (event) => {
    const form = event.target;
    if (form.method === "dialog") return;
    const submitter = event.submitter;
    if (submitter?.hasAttribute("formtarget") && submitter.formTarget !== "_self") return;
    if (navigatesHere(form, submitter?.getAttribute("formaction") || form.action)) show(event);
  });
  window.addEventListener("pageshow", reset);
})();
