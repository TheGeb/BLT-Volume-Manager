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
  private checkBtn: HTMLButtonElement;
  private repairBtn: HTMLButtonElement;
  private volumeStatsPanel: HTMLElement;
  private volumeTroubleshootPanel: HTMLElement;
  private volumeView: HTMLElement;
  private testPanel: HTMLElement;
  private testVolumeInput: HTMLInputElement;
  private testCreateBtn: HTMLButtonElement;
  private testStatus: HTMLDivElement;
  private statsToggleIcon: HTMLElement;
  private statsHeader: HTMLElement;
  private statsGrid: HTMLDivElement;
  private statsLoaded = false;
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
    const statsGrid = document.getElementById('volumeStatsGrid') as HTMLDivElement;
    this.statsGrid = statsGrid;
    this.statsHeader = document.getElementById('statsHeader') as HTMLElement;
    this.statsToggleIcon = document.getElementById('statsToggleIcon') as HTMLElement;
    const volumeView = document.getElementById('volumeView') as HTMLElement;
    const lockPanelContent = document.getElementById('lockPanelContent') as HTMLElement;
    const lockPanelSkeleton = document.getElementById('lockPanelSkeleton') as HTMLElement;
    this.themeIcon = document.getElementById('themeIcon') as HTMLElement;
    this.refreshBtn = document.getElementById('refreshButton') as HTMLButtonElement;
    this.checkBtn = document.getElementById('volumeCheckButton') as HTMLButtonElement;
    this.repairBtn = document.getElementById('volumeRepairButton') as HTMLButtonElement;
    this.themeToggle = document.getElementById('themeToggle') as HTMLButtonElement;
    this.initRepoBtn = document.getElementById('initRepoButton') as HTMLButtonElement;
    this.initBanner = document.getElementById('repoInitBanner') as HTMLDivElement;
    this.volumeStatsPanel = document.getElementById('volumeStatsPanel') as HTMLElement;
    this.volumeTroubleshootPanel = document.getElementById('volumeTroubleshootPanel') as HTMLElement;
    this.volumeView = volumeView;
    this.testPanel = document.getElementById('testPanel') as HTMLElement;
    this.testVolumeInput = document.getElementById('testVolumeInput') as HTMLInputElement;
    this.testCreateBtn = document.getElementById('testCreateBtn') as HTMLButtonElement;
    this.testStatus = document.getElementById('testStatus') as HTMLDivElement;

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

    this.testCreateBtn.addEventListener('click', () => this.createTestVolume());

    this.bindEvents();
    this.onVolumeChange('');
  }

  static showStatus(msg: string, isError = false): void {
    this._statusEl.textContent = msg;
    this._statusEl.style.color = isError ? 'var(--red)' : '';
  }

  static setBanner(msg: string, isError = false): void {
    if (msg) {
      App._bannerText.textContent = msg;
      App._banner.style.color = isError ? 'var(--red)' : 'var(--text)';
      App._banner.style.borderColor = isError ? 'var(--red)' : 'var(--border)';
      App._banner.style.background = isError ? 'rgba(239,68,68,0.08)' : 'transparent';
      App._banner.classList.add('visible');
    } else {
      App._banner.classList.remove('visible');
      App._banner.style.background = '';
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
      App.setBanner('');
      const vol = this.state.selectedVolume;
      try { await fetch('/api/stats/refresh', { method: 'POST' }); } catch {}
      if (vol) {
        await this.snapMgr.load(vol).catch(e => App.setBanner(e.message, true));
        if (this.statsGrid.style.display !== 'none') {
          this.statsMgr.load(vol).catch(e => App.setBanner(e.message, true));
        } else {
          this.statsLoaded = false;
        }
      }
      await this.pillsMgr.load().catch(e => App.setBanner(e.message, true));
      if (vol) this.lockMgr.refresh();
    });

    const sortButton = document.getElementById('sortButton') as HTMLButtonElement;
    sortButton.addEventListener('click', () => {
      this.state.sortNewestFirst = !this.state.sortNewestFirst;
      sortButton.textContent = this.state.sortNewestFirst ? 'Sort by newest' : 'Sort by oldest';
      this.snapMgr.render();
    });

    this.checkBtn.addEventListener('click', async () => {
      const vol = this.state.selectedVolume;
      if (!vol) return;
      this.checkBtn.disabled = true;
      this.checkBtn.textContent = 'Checking...';
      App.setBanner('');
      App.setBanner('Checking repository integrity...');
      try {
        const resp = await fetch(`/api/repo/check?volume=${encodeURIComponent(vol)}`, { method: 'POST' });
        if (resp.ok) {
          const d = await resp.json();
          App.setBanner(d.status);
        } else {
          const b = await resp.json();
          throw new Error(b.error || 'check failed');
        }
      } catch (err) {
        App.setBanner((err as Error).message, true);
      } finally {
        this.checkBtn.disabled = false;
        this.checkBtn.textContent = 'Check';
      }
    });

    this.repairBtn.addEventListener('click', async () => {
      const vol = this.state.selectedVolume;
      if (!vol) return;
      this.repairBtn.disabled = true;
      this.repairBtn.textContent = 'Repairing...';
      App.setBanner('');
      App.setBanner('Running repair (unlock + rebuild index)...');
      try {
        const resp = await fetch(`/api/repo/repair?volume=${encodeURIComponent(vol)}`, { method: 'POST' });
        if (resp.ok) {
          const d = await resp.json();
          App.setBanner(d.status);
        } else {
          const b = await resp.json();
          throw new Error(b.error || 'repair failed');
        }
      } catch (err) {
        App.setBanner((err as Error).message, true);
      } finally {
        this.repairBtn.disabled = false;
        this.repairBtn.textContent = 'Repair';
      }
    });

    this.statsHeader.addEventListener('click', () => {
      const grid = this.statsGrid;
      const expanded = grid.style.display !== 'none';
      grid.style.display = expanded ? 'none' : '';
      this.statsToggleIcon.style.transform = expanded ? 'rotate(0deg)' : 'rotate(90deg)';
      if (!expanded && !this.statsLoaded) {
        const vol = this.state.selectedVolume;
        if (vol) {
          this.statsLoaded = true;
          this.statsMgr.load(vol).catch(() => {});
        }
      }
    });

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
    ]);
  }

  private async createTestVolume(): Promise<void> {
    const name = this.testVolumeInput.value.trim();
    if (!name) {
      this.testStatus.textContent = 'Enter a volume name';
      this.testStatus.style.color = 'var(--red)';
      return;
    }
    this.testCreateBtn.disabled = true;
    this.testCreateBtn.textContent = 'Creating...';
    this.testStatus.textContent = '';
    try {
      const resp = await fetch('/api/test/create-volume', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
      });
      const d = await resp.json();
      if (!resp.ok) {
        throw new Error(d.error || `HTTP ${resp.status}`);
      }
      this.testStatus.textContent = d.message;
      this.testStatus.style.color = '';
      this.testVolumeInput.value = '';
      await this.pillsMgr.load();
      this.onVolumeChange(name);
    } catch (err) {
      this.testStatus.textContent = (err as Error).message;
      this.testStatus.style.color = 'var(--red)';
    } finally {
      this.testCreateBtn.disabled = false;
      this.testCreateBtn.textContent = 'Create test volume';
    }
  }

  private onVolumeChange(vol: string): void {
    document.dispatchEvent(new CustomEvent('close-snapshot-viewer'));
    this.testPanel.style.display = vol ? 'none' : '';
    if (vol) {
      this.volumeView.style.display = 'grid';
      this.volumeStatsPanel.style.display = '';
      this.volumeTroubleshootPanel.style.display = '';
      this.snapMgr.load(vol).catch(() => {});
      this.lockMgr.refresh();
      this.statsLoaded = false;
      this.statsGrid.style.display = 'none';
      this.statsGrid.innerHTML = '';
      this.statsToggleIcon.style.transform = 'rotate(0deg)';
    } else {
      this.volumeView.style.display = 'none';
      this.volumeStatsPanel.style.display = 'none';
      this.volumeTroubleshootPanel.style.display = 'none';
    }
  }

  private async checkRepoStatus(): Promise<void> {
    try {
      const resp = await fetch('/api/pills');
      if (!resp.ok) {
        App.setBanner('Failed to check repository status', true);
        return;
      }
      const data = await resp.json() as { volumes: string[] };
      if (data.volumes && data.volumes.length > 0) {
        App.setBanner('');
      } else {
        App.setBanner('No volumes found. Create one with: docker volume create --driver s3vol --name <name>');
      }
    } catch {
      App.setBanner('Cannot reach server', true);
    }
  }

}

window.addEventListener('load', () => {
  const app = new App();
  app.start();
});
