/* One console for analyzer failures and structured diagnostics across views. */
(() => {
  const dialog = document.querySelector('#diagnostics-console');
  const trigger = document.querySelector('[data-diagnostics-open]');
  const list = dialog.querySelector('[data-diagnostics-list]');
  const count = severity => {
    const n = list.querySelectorAll(`[data-diagnostic-level="${severity}"]`).length;
    return n;
  };
  const refresh = () => {
    const errors = count('error'), warnings = count('warning');
    trigger.querySelector('[data-diagnostics-errors]').textContent = errors;
    trigger.querySelector('[data-diagnostics-warnings]').textContent = warnings;
    const label = (n, severity) => `${n} ${severity}${n === 1 ? '' : 's'}`;
    trigger.setAttribute('aria-label', `Diagnostics console: ${label(errors, 'error')}, ${label(warnings, 'warning')}`);
    dialog.querySelector('[data-diagnostics-empty]').hidden = list.children.length > 0;
  };
  trigger.addEventListener('click', () => dialog.showModal());
  dialog.addEventListener('click', event => {
    const bounds = dialog.getBoundingClientRect();
    if (event.target === dialog && (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom)) dialog.close();
  });
  document.addEventListener('seamgraph:diagnostics', event => {
    for (const entry of event.detail) {
      const severity = ['error', 'warning', 'info'].includes(entry.severity) ? entry.severity : 'info';
      const row = document.createElement('li'); row.dataset.diagnosticLevel = severity;
      const heading = document.createElement('div');
      const level = document.createElement('strong'); level.textContent = severity;
      const code = document.createElement('code'); code.textContent = entry.code;
      heading.append(level, ' ', code);
      const message = document.createElement('pre'); message.dataset.diagnosticMessage = ''; message.textContent = entry.message;
      row.append(heading, message);
      for (const location of entry.evidence || []) {
        const source = document.createElement('code'); source.textContent = `${location.file}:${location.startLine}:${location.startColumn}`;
        row.append(source);
      }
      if (entry.entityIds?.length) {
        const entities = document.createElement('p'); entities.textContent = entry.entityIds.join(', '); row.append(entities);
      }
      if ([...list.children].some(existing => existing.dataset.diagnosticLevel === severity && existing.textContent === row.textContent)) continue;
      list.append(row);
    }
    refresh();
  });
})();
