/* Local workspace tabs retain the selected tree item and source scroll position. */
(() => {
  const tabs = [...document.querySelectorAll('.code-tabs [role=tab]')];
  if (!tabs.length) return;
  const select = (tab, updateURL = true) => {
    const review = tab.id === 'code-review-tab';
    for (const item of tabs) {
      item.setAttribute('aria-selected', String(item === tab));
      item.tabIndex = item === tab ? 0 : -1;
      document.getElementById(item.getAttribute('aria-controls')).hidden = item !== tab;
    }
    // Keep the workspace choice when submitting filters or following local links.
    document.querySelectorAll('input[name=tools]').forEach(input => { input.value = review ? '1' : '0'; });
    document.querySelectorAll('.code-main a[href]').forEach(link => {
      const address = new URL(link.href, location.href);
      if (address.origin !== location.origin || address.pathname !== '/explore') return;
      if (review) address.searchParams.set('tools', '1');
      else address.searchParams.delete('tools');
      link.href = address.pathname + address.search + address.hash;
    });
    if (updateURL) {
      const address = new URL(location.href);
      if (review) address.searchParams.set('tools', '1');
      else address.searchParams.delete('tools');
      history.replaceState(history.state, '', address.pathname + address.search + address.hash);
    }
  };
  for (const tab of tabs) {
    tab.addEventListener('click', () => select(tab));
    tab.addEventListener('keydown', event => {
      const index = tabs.indexOf(tab);
      const next = {ArrowRight: (index + 1) % tabs.length, ArrowLeft: (index + tabs.length - 1) % tabs.length, Home: 0, End: tabs.length - 1}[event.key];
      if (next === undefined) return;
      event.preventDefault();
      select(tabs[next]);
      tabs[next].focus();
    });
  }
  select(tabs.find(tab => tab.getAttribute('aria-selected') === 'true') || tabs[0], false);
})();
