/* Read-only source viewing. Highlighting stays local and off the UI thread. */
(() => {
  const languages = {go:"go",ts:"typescript",tsx:"typescript",mts:"typescript",cts:"typescript",js:"javascript",jsx:"javascript",py:"python",json:"json",yaml:"yaml",yml:"yaml",sh:"bash",bash:"bash",html:"xml",htm:"xml",xml:"xml",svg:"xml",css:"css",md:"markdown",sql:"sql",ini:"ini",toml:"ini"};
  const sources=[...document.querySelectorAll("[data-source-code]")];
  const languageFor=code=>languages[code.dataset.sourceFile.split(".").pop().toLowerCase()];
  const candidates=sources.filter(code=>languageFor(code) && code.textContent.length<=250000);
  const pending = new Map();
  let worker;
  try {
    if (window.Worker && candidates.length) worker = new Worker("/assets/source-worker.js");
  } catch { /* Plain source remains available if workers are unavailable. */ }
  if (worker) {
    worker.onmessage = ({data}) => {
      const code = pending.get(data.id);
      if (!code) return;
      pending.delete(data.id);
      if (typeof data.html === "string") {
        // highlight.js escapes source before producing its own token spans.
        code.innerHTML = data.html;
        code.classList.add("hljs");
        code.dataset.highlighted = "true";
      }
      if (!pending.size) worker.terminate();
    };
    worker.onerror = event => { event.preventDefault(); worker.terminate(); };
    setTimeout(() => worker.terminate(), 10000);
  }
  // Keep very large files readable without constructing excessive token DOM.
  if (worker) candidates.forEach((code,id) => {
    pending.set(id,code);
    worker.postMessage({id,language:languageFor(code),text:code.textContent});
  });
  document.querySelectorAll("[data-source-focus]").forEach(scroll => {
    const line=Number(scroll.dataset.sourceFocus);
    const source=scroll.querySelector("code");
    if (source && line>1) {
      const height=parseFloat(getComputedStyle(source).lineHeight);
      scroll.scrollTop=Math.max(0,(line-3)*height);
    }
  });
})();
