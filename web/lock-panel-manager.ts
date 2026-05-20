/// <reference path="types.ts" />
/// <reference path="util.ts" />

class LockPanelManager {
  private panel: HTMLElement;
  private content: HTMLElement;
  private skeleton: HTMLElement;
  private statusText: HTMLDivElement;
  private ownerEl: HTMLDivElement;
  private expiryEl: HTMLDivElement;
  private createBtn: HTMLButtonElement;
  private deleteBtn: HTMLButtonElement;
  private getState: () => AppState;
  private timer: ReturnType<typeof setInterval> | null = null;
  private contentHeight = 0;

  constructor(
    panel: HTMLElement, content: HTMLElement, skeletonEl: HTMLElement,
    statusText: HTMLDivElement, ownerEl: HTMLDivElement, expiryEl: HTMLDivElement,
    createBtn: HTMLButtonElement, deleteBtn: HTMLButtonElement,
    getState: () => AppState,
  ) {
    this.panel = panel;
    this.content = content;
    this.skeleton = skeletonEl;
    this.statusText = statusText;
    this.ownerEl = ownerEl;
    this.expiryEl = expiryEl;
    this.createBtn = createBtn;
    this.deleteBtn = deleteBtn;
    this.getState = getState;
  }

  showSkeleton(): void {
    if (this.timer) { clearInterval(this.timer); this.timer = null; }
    this.panel.style.display = '';
    if (this.content.style.display !== 'none') {
      this.contentHeight = this.content.scrollHeight;
    }
    this.content.style.display = 'none';
    this.skeleton.style.display = 'block';
  }

  async refresh(): Promise<void> {
    const vol = this.getState().selectedVolume;
    if (!vol) return;
    this.showSkeleton();
    try {
      const resp = await fetch(`/api/volume/${encodeURIComponent(vol)}/locks`);
      if (!resp.ok) {
        this.statusText.textContent = 'Error';
        this.ownerEl.textContent = '';
        this.expiryEl.textContent = '';
        this.deleteBtn.disabled = false;
        return;
      }
      const data = await resp.json() as LockStatus;
      this.render(data);
    } catch {
      this.statusText.textContent = 'Error';
      this.ownerEl.textContent = '';
      this.expiryEl.textContent = '';
      this.deleteBtn.disabled = false;
    }
  }

  render(data: LockStatus): void {
    this.skeleton.style.display = 'none';
    this.content.style.display = '';
    if (this.timer) { clearInterval(this.timer); this.timer = null; }

    const startH = this.contentHeight;
    this.content.style.transition = 'none';
    this.content.style.height = '';
    this.contentHeight = 0;

    if (data.locked) {
      this.statusText.textContent = 'Locked';
      this.statusText.style.color = 'var(--yellow)';
      this.ownerEl.textContent = `by ${data.owner}`;
      const tick = () => {
        if (data.expires_in != null && data.expires_in > 0) {
          data.expires_in--;
          this.expiryEl.textContent = formatDuration(data.expires_in);
        } else {
          this.expiryEl.textContent = 'expired';
        }
      };
      tick();
      this.timer = setInterval(tick, 1000);
    } else {
      this.statusText.textContent = 'Unlocked';
      this.statusText.style.color = 'var(--green)';
      this.ownerEl.textContent = '';
      this.expiryEl.textContent = '';
    }
    this.deleteBtn.disabled = !data.locked;

    const newH = this.content.scrollHeight;
    if (startH > 0 && newH !== startH) {
      this.content.style.height = `${startH}px`;
      requestAnimationFrame(() => {
        this.content.style.transition = 'height 0.25s ease';
        this.content.style.height = `${newH}px`;
      });
      setTimeout(() => {
        this.content.style.height = '';
        this.content.style.transition = '';
      }, 260);
    }
  }

  async createLock(): Promise<void> {
    const vol = this.getState().selectedVolume;
    if (!vol) return;
    const ownerName = prompt('Lock owner name:', this.getState().hostname);
    if (!ownerName) return;
    App.showStatus(`Creating lock for volume ${vol}...`);
    try {
      const resp = await fetch(`/api/volume/${encodeURIComponent(vol)}/locks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ owner: ownerName }),
      });
      if (!resp.ok) {
        const body = await resp.json();
        throw new Error(body.error || 'Failed to create lock');
      }
      App.showStatus(`Lock created for volume ${vol}.`);
      await this.refresh();
    } catch (err) {
      App.showStatus((err as Error).message, true);
    }
  }

  async deleteLocks(): Promise<void> {
    const vol = this.getState().selectedVolume;
    if (!vol) return;
    if (!confirm(`Delete all locks for volume ${vol}?`)) return;
    App.showStatus(`Deleting locks for volume ${vol}...`);
    try {
      const resp = await fetch(`/api/volume/${encodeURIComponent(vol)}/locks`, { method: 'DELETE' });
      if (!resp.ok) {
        const body = await resp.json();
        throw new Error(body.error || 'Failed to delete locks');
      }
      App.showStatus(`Deleted locks for volume ${vol}.`);
      await this.refresh();
    } catch (err) {
      App.showStatus((err as Error).message, true);
    }
  }
}
