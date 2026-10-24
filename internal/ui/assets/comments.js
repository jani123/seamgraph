/* Comments are saved against the captured file/revision, never the live DOM. */
(() => {
  const dialog = document.querySelector('#comment-composer');
  if (!dialog) return;
  const form = dialog.querySelector('form');
  document.addEventListener('click', event => {
    if (event.target.closest('[data-comment-close]')) { dialog.close(); return; }
    const button = event.target.closest('[data-comment-open], [data-comment-line]');
    if (!button) return;
    form.elements.side.value = button.dataset.commentSide || '';
    form.elements.line.value = button.dataset.commentLine || '';
    const side = form.elements.side.value === 'before' ? 'Base' : 'Head';
    dialog.querySelector('[data-comment-anchor]').textContent = form.elements.file.value + (form.elements.line.value ? ` · ${side} line ${form.elements.line.value}` : ' · File comment');
    const preview = dialog.querySelector('[data-comment-preview]');
    preview.textContent = button.closest('.diff-row')?.querySelector('[data-diff-code]')?.textContent || '';
    preview.hidden = !form.elements.line.value;
    dialog.showModal();
  });
})();
