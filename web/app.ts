/// <reference path="types.ts" />
/// <reference path="util.ts" />
/// <reference path="stats-manager.ts" />
/// <reference path="pills-manager.ts" />
/// <reference path="lock-panel-manager.ts" />
/// <reference path="snapshot-manager.ts" />

class App {
  private static _statusEl: HTMLDivElement;
  private static _bannerText: HTMLSpanElement;
  private static _banner: HTMLDivElement;

  state: AppState;
  statsMgr: StatsManager;
  pillsMgr: PillsManager;
  lockMgr: LockPanelManager;
  snapMgr: SnapshotManager;

  private initRepoBtn: HTMLButtonElement;
  private initBanner: HTMLDivElement;
  private refreshBtn: HTMLButtonElement;
  private statsPanel: HTMLElement;
  private volumeView: HTMLElement;
  private themeToggle: HTMLButtonElement;
  private themeIcon: HTMLElement;
  private moonSvg: string;
  private sunSvg: string;

  constructor() {
    this.state = {
      snapshots: [], volumes: [], selectedVolume: '', volumeFilter: '',
      query: '', sortNewestFirst: true, hostname: 'webadmin', prevStats: null,
    };

    const snapshotTable = document.getElementById('snapshotTable') as HTMLTableSectionElement;
    const searchInput = document.getElementById('searchInput') as HTMLInputElement;
    const sortButton = document.getElementById('sortButton') as HTMLButtonElement;
    const statusMessage = document.getElementById('statusMessage') as HTMLDivElement;
    const volumePills = document.getElementById('volumePills') as HTMLDivElement;
    const volumeFilterInput = document.getElementById('volumeFilterInput') as HTMLInputElement;
    const createLockButton = document.getElementById('createLockButton') as HTMLButtonElement;
    const deleteLocksButton = document.getElementById('deleteLocksButton') as HTMLButtonElement;
    const lockPanel = document.getElementById('lockPanel') as HTMLElement;
    const lockStatusText = document.getElementById('lockStatusText') as HTMLDivElement;
    const lockOwner = document.getElementById('lockOwner') as HTMLDivElement;
    const lockExpiry = document.getElementById('lockExpiry') as HTMLDivElement;
    const statsGrid = document.getElementById('statsGrid') as HTMLDivElement;
    const volumeView = document.getElementById('volumeView') as HTMLElement;
    const lockPanelContent = document.getElementById('lockPanelContent') as HTMLElement;
    const lockPanelSkeleton = document.getElementById('lockPanelSkeleton') as HTMLElement;
    this.themeIcon = document.getElementById('themeIcon') as HTMLElement;
    this.refreshBtn = document.getElementById('refreshButton') as HTMLButtonElement;
    this.themeToggle = document.getElementById('themeToggle') as HTMLButtonElement;
    this.initRepoBtn = document.getElementById('initRepoButton') as HTMLButtonElement;
    this.initBanner = document.getElementById('repoInitBanner') as HTMLDivElement;
    this.statsPanel = document.getElementById('statsPanel') as HTMLElement;
    this.volumeView = volumeView;

    App._statusEl = statusMessage;
    App._banner = document.getElementById('errorBanner') as HTMLDivElement;
    App._bannerText = document.getElementById('errorBannerText') as HTMLSpanElement;

    this.sunSvg = this.themeIcon.innerHTML;
    this.moonSvg = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path></svg>';

    this.statsMgr = new StatsManager(statsGrid, () => this.state.prevStats, (s) => { this.state.prevStats = s; });
    this.pillsMgr = new PillsManager(volumePills, volumeFilterInput,
      () => this.state, (p) => Object.assign(this.state, p), (vol) => this.onVolumeChange(vol));
    this.lockMgr = new LockPanelManager(lockPanel, lockPanelContent, lockPanelSkeleton,
      lockStatusText, lockOwner, lockExpiry, createLockButton, deleteLocksButton,
      () => this.state);
    this.snapMgr = new SnapshotManager(snapshotTable, searchInput, sortButton,
      () => this.state, (p) => Object.assign(this.state, p));

    this.bindEvents();
  }

  static showStatus(msg: string, isError = false): void {
    this._statusEl.textContent = msg;
    this._statusEl.style.color = isError ? 'var(--red)' : '';
  }

  static setErrorBanner(msg: string): void {
    if (msg) {
      App._bannerText.textContent = msg;
      App._banner.style.display = '';
    } else {
      App._banner.style.display = 'none';
    }
  }

  private bindEvents(): void {
    const searchInput = document.getElementById('searchInput') as HTMLInputElement;
    const volumeFilterInput = document.getElementById('volumeFilterInput') as HTMLInputElement;
    const volumePills = document.getElementById('volumePills') as HTMLDivElement;
    const createLockButton = document.getElementById('createLockButton') as HTMLButtonElement;
    const deleteLocksButton = document.getElementById('deleteLocksButton') as HTMLButtonElement;

    searchInput.addEventListener('input', () => {
      this.state.query = searchInput.value.trim().toLowerCase();
      this.snapMgr.render();
    });

    volumeFilterInput.addEventListener('input', () => {
      this.state.volumeFilter = volumeFilterInput.value.trim().toLowerCase();
      this.pillsMgr.render();
    });

    volumePills.addEventListener('click', (e) => this.pillsMgr.handleContainerClick(e));

    createLockButton.addEventListener('click', () => this.lockMgr.createLock());
    deleteLocksButton.addEventListener('click', () => this.lockMgr.deleteLocks());

    this.refreshBtn.addEventListener('click', async () => {
      App.setErrorBanner('');
      try { await fetch('/api/stats/refresh', { method: 'POST' }); } catch {}
      await Promise.all([
        this.statsMgr.load().catch(e => App.setErrorBanner(e.message)),
        this.snapMgr.load().catch(e => App.setErrorBanner(e.message)),
      ]);
      if (this.state.selectedVolume) this.lockMgr.refresh();
    });

    const sortButton = document.getElementById('sortButton') as HTMLButtonElement;
    sortButton.addEventListener('click', () => {
      this.state.sortNewestFirst = !this.state.sortNewestFirst;
      sortButton.textContent = this.state.sortNewestFirst ? 'Sort by newest' : 'Sort by oldest';
      this.snapMgr.render();
    });

    this.initRepoBtn.addEventListener('click', () => this.initRepo());

    this.themeToggle.addEventListener('click', () => {
      const isLight = document.body.classList.toggle('light');
      this.themeIcon.innerHTML = isLight ? this.moonSvg : this.sunSvg;
    });
  }

  async start(): Promise<void> {
    this.pillsMgr.showSkeleton();
    await Promise.all([
      this.checkRepoStatus(),
      this.pillsMgr.load(),
      this.snapMgr.load().catch(e => App.setErrorBanner(e.message)),
      this.statsMgr.load().catch(e => App.setErrorBanner(e.message)),
    ]);
  }

  private onVolumeChange(vol: string): void {
    if (vol) {
      this.statsPanel.style.display = 'none';
      this.volumeView.style.display = 'grid';
      this.lockMgr.refresh();
    } else {
      this.statsPanel.style.display = '';
      this.volumeView.style.display = 'none';
    }
    this.snapMgr.render();
  }

  private async checkRepoStatus(): Promise<void> {
    try {
      const resp = await fetch('/api/repo/status');
      if (!resp.ok) {
        let msg = 'Failed to check repository status';
        try { const b = await resp.json(); if (b.error) msg = b.error; } catch {}
        App.showStatus(msg, true);
        App.setErrorBanner(msg);
        return;
      }
      const data = await resp.json() as RepoStatus;
      this.initBanner.style.display = data.initialized ? 'none' : 'flex';
      if (data.hostname) this.state.hostname = data.hostname;
      if (data.initialized !== false) App.setErrorBanner('');
    } catch {
      const msg = 'Cannot reach server';
      App.showStatus(msg, true);
      App.setErrorBanner(msg);
    }
  }

  private async initRepo(): Promise<void> {
    this.initRepoBtn.disabled = true;
    this.initRepoBtn.textContent = 'Initializing...';
    try {
      const resp = await fetch('/api/repo/init', { method: 'POST' });
      if (!resp.ok) {
        const body = await resp.json();
        throw new Error(body.error || 'Failed to initialize repository');
      }
      this.initBanner.style.display = 'none';
      await Promise.all([this.snapMgr.load(), this.statsMgr.load()]);
    } catch (err) {
      App.showStatus((err as Error).message, true);
    } finally {
      this.initRepoBtn.disabled = false;
      this.initRepoBtn.textContent = 'Initialize Repository';
    }
  }
}

window.addEventListener('load', () => {
  const app = new App();
  app.start();
});
