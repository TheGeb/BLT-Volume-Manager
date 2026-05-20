/// <reference path="types.ts" />
/// <reference path="util.ts" />

class PillsManager {
  private container: HTMLDivElement;
  private filterInput: HTMLInputElement;
  private getState: () => AppState;
  private setState: (patch: Partial<AppState>) => void;
  private onSelect: (vol: string) => void;

  constructor(
    container: HTMLDivElement,
    filterInput: HTMLInputElement,
    getState: () => AppState,
    setState: (patch: Partial<AppState>) => void,
    onSelect: (vol: string) => void,
  ) {
    this.container = container;
    this.filterInput = filterInput;
    this.getState = getState;
    this.setState = setState;
    this.onSelect = onSelect;
  }

  showSkeleton(): void {
    this.container.innerHTML = '';
    const pillWidth = 100;
    const gap = 8;
    const containerWidth = this.container.offsetWidth || 600;
    const perRow = Math.max(1, Math.floor((containerWidth + gap) / (pillWidth + gap)));
    for (let i = 0; i < perRow; i++) {
      const pill = document.createElement('div');
      pill.className = 'skeleton skeleton-pill';
      pill.style.width = `${pillWidth}px`;
      this.container.appendChild(pill);
    }
  }

  async load(retries = 5, delay = 2000): Promise<void> {
    this.showSkeleton();
    for (let attempt = 0; attempt < retries; attempt++) {
      try {
        const resp = await fetch('/api/pills');
        if (!resp.ok) { await this.sleep(delay); continue; }
        const data = await resp.json();
        if (!data.volumes || data.volumes.length === 0) {
          if (attempt < retries - 1) { await this.sleep(delay); continue; }
        }
        this.setState({ volumes: data.volumes ?? [], pillsCachedAt: data.cached_at });
        this.render();
        this.renderCachedAt(this.container.parentElement!);
        return;
      } catch { await this.sleep(delay); }
    }
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(r => setTimeout(r, ms));
  }

  render(): void {
    const st = this.getState();
    const filter = st.volumeFilter;
    const matched = filter ? st.volumes.filter(v => v.toLowerCase().includes(filter)) : st.volumes;
    fadeIn(this.container, () => {
    this.container.innerHTML = '';
    matched.forEach(volume => {
      const pill = document.createElement('button');
      pill.className = 'volume-pill' + (volume === st.selectedVolume ? ' active' : '');
      pill.dataset.volume = volume;
      pill.textContent = volume;
      this.container.appendChild(pill);
    });
    });
  }

  renderCachedAt(parent: HTMLElement): void {
    const at = this.getState().pillsCachedAt;
    if (!at) return;
    parent.querySelectorAll('.pills-cached').forEach(el => el.remove());
    const el = document.createElement('div');
    el.className = 'pills-cached';
    el.textContent = `Cached: ${new Date(at).toLocaleString()}`;
    parent.appendChild(el);
  }

  handleContainerClick(e: MouseEvent): void {
    const pill = (e.target as HTMLElement).closest('.volume-pill') as HTMLElement | null;
    if (!pill) return;
    const vol = pill.dataset.volume || '';
    const st = this.getState();
    const newVol = vol === st.selectedVolume ? '' : vol;
    this.setState({ selectedVolume: newVol, volumeFilter: '', query: '' });
    this.filterInput.value = '';
    this.render();
    this.onSelect(newVol);
  }
}
