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
    for (let i = 0; i < perRow * 2; i++) {
      const pill = document.createElement('div');
      pill.className = 'skeleton skeleton-pill';
      pill.style.width = `${pillWidth}px`;
      this.container.appendChild(pill);
    }
  }

  async load(): Promise<void> {
    this.showSkeleton();
    try {
      const resp = await fetch('/api/pills');
      if (resp.ok) {
        const data = await resp.json();
        this.setState({ volumes: data.volumes });
        this.render();
      }
    } catch { /* fall through */ }
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
