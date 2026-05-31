interface ColResizeElements {
  treePanelEl?: HTMLElement;
  contentEl?: HTMLElement;
}

export function colResize(
  node: HTMLElement,
  elements: ColResizeElements,
): { destroy: () => void; update: (e: ColResizeElements) => void } {
  let treePanelEl = elements.treePanelEl;
  let contentEl = elements.contentEl;
  let startX = 0;
  let startWidth = 0;

  function onMouseDown(e: MouseEvent) {
    e.preventDefault();
    startX = e.pageX;
    startWidth = treePanelEl ? treePanelEl.offsetWidth : 300;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }

  function onMouseMove(e: MouseEvent) {
    if (!treePanelEl || !contentEl) return;
    const rect = contentEl.getBoundingClientRect();
    const minW = 200;
    const maxW = rect.width - 200;
    const deltaX = e.pageX - startX;
    let w = startWidth + deltaX;
    if (w < minW) w = minW;
    if (w > maxW) w = maxW;
    treePanelEl.style.minWidth = '0';
    treePanelEl.style.flex = `0 0 ${String(w)}px`;
  }

  function onMouseUp() {
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  }

  node.addEventListener('mousedown', onMouseDown);

  return {
    update(newElements: ColResizeElements) {
      treePanelEl = newElements.treePanelEl;
      contentEl = newElements.contentEl;
    },
    destroy() {
      node.removeEventListener('mousedown', onMouseDown);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    },
  };
}

interface RowResizeElements {
  contentEl?: HTMLElement;
  panelEl?: HTMLElement;
}

export function rowResize(
  node: HTMLElement,
  elements: RowResizeElements,
): { destroy: () => void; update: (e: RowResizeElements) => void } {
  let contentEl = elements.contentEl;
  let panelEl = elements.panelEl;
  let startY = 0;
  let startHeight = 0;
  let startMaxHeight = 0;
  let atBottom = false;
  let atBottomStartHeight = 0;
  let tabPanelEl: HTMLElement | null = null;

  function onMouseDown(e: MouseEvent) {
    e.preventDefault();
    if (!contentEl || !panelEl) return;
    if (tabPanelEl) tabPanelEl.style.marginBottom = '';
    tabPanelEl = panelEl.closest('.tab-panel');
    startY = e.clientY;
    startHeight = contentEl.offsetHeight;
    startMaxHeight = window.innerHeight - contentEl.getBoundingClientRect().top - 40;
    atBottom = false;
    atBottomStartHeight = startHeight;
    document.body.style.overflow = 'hidden';
    document.body.style.cursor = 'row-resize';
    document.body.style.userSelect = 'none';
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }

  function onMouseMove(e: MouseEvent) {
    if (!contentEl) return;
    const deltaY = e.clientY - startY;
    const minH = 200;
    let h = startHeight + deltaY;
    if (h < minH) h = minH;
    if (h > startMaxHeight) h = startMaxHeight;
    contentEl.style.height = `${String(h)}px`;
    if (h < startHeight) {
      if (!atBottom && window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 1) {
        atBottom = true;
        atBottomStartHeight = h;
      }
      if (atBottom && h < atBottomStartHeight && tabPanelEl) {
        tabPanelEl.style.marginBottom = `${String(atBottomStartHeight - h)}px`;
      }
    } else if (atBottom) {
      atBottom = false;
      if (tabPanelEl) tabPanelEl.style.marginBottom = '';
    }
  }

  function onMouseUp() {
    atBottom = false;
    document.body.style.overflow = '';
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  }

  function shrinkMarginOnScroll() {
    if (tabPanelEl) {
      const s = tabPanelEl.style.marginBottom;
      if (s && s !== '' && s !== '0px' && s !== '0') {
        const m = parseFloat(s);
        if (m > 0) {
          const maxScroll = document.documentElement.scrollHeight - window.innerHeight;
          const distFromBottom = maxScroll - window.scrollY;
          if (distFromBottom > 0) {
            tabPanelEl.style.marginBottom = `${String(Math.max(0, m - distFromBottom))}px`;
          }
        }
      }
    }
  }

  window.addEventListener('scroll', shrinkMarginOnScroll);

  node.addEventListener('mousedown', onMouseDown);

  return {
    update(newElements: RowResizeElements) {
      contentEl = newElements.contentEl;
      panelEl = newElements.panelEl;
    },
    destroy() {
      node.removeEventListener('mousedown', onMouseDown);
      document.body.style.overflow = '';
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
      window.removeEventListener('scroll', shrinkMarginOnScroll);
      if (tabPanelEl) tabPanelEl.style.marginBottom = '';
    },
  };
}
