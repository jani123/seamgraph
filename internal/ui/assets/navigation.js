/* Shared comparison controls and native navigation feedback. */
(() => {
  document.querySelectorAll("[data-comparison-controls]").forEach(form => {
    const scope = form.elements.scope, base = form.elements.base, head = form.elements.head;
    let revisions = scope.value === "working" ? ["HEAD~1", "HEAD"] : [base.value, head.value];
    const update = () => {
      const working = scope.value === "working";
      if (working) { base.value = "HEAD"; head.value = "WORKTREE"; }
      else { [base.value, head.value] = revisions; }
      base.readOnly = head.readOnly = working;
    };
    scope.addEventListener("change", update);
    form.addEventListener("input", event => { if ((event.target === base || event.target === head) && scope.value === "revisions") revisions = [base.value, head.value]; });
    update();
  });
  const indicator = document.querySelector("#view-loading");
  const reset = () => {
    indicator.hidden = true;
    document.querySelector("main")?.removeAttribute("aria-busy");
  };
  const show = (event) => {
    // Other handlers may turn a link or form into an in-page interaction.
    setTimeout(() => {
      if (event.defaultPrevented) return;
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
