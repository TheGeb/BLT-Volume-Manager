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
  private volumeView: HTMLElement;
  private landingPanel: HTMLElement;
  private testVolumeInput: HTMLInputElement;
  private testCreateBtn: HTMLButtonElement;
  private testStatus: HTMLDivElement;
  private statsGrid: HTMLDivElement;
  private deleteVolumePanel: HTMLElement;
  private deleteVolumeBtn: HTMLButtonElement;
  private deleteModal: HTMLElement;
  private deleteModalVolumeName: HTMLElement;
  private deleteConfirmInput: HTMLInputElement;
  private deleteModalConfirm: HTMLButtonElement;
  private deleteModalCancel: HTMLButtonElement;
  private themeToggle: HTMLButtonElement;
  private themeIcon: HTMLElement;
  private moonSvg: string;
  private sunSvg: string;
  private tabSnapshots: HTMLButtonElement;
  private tabRepo: HTMLButtonElement;
  private snapshotsTab: HTMLElement;
  private repoTab: HTMLElement;

  constructor() {
    this.state = {
      snapshots: [], volumes: [], selectedVolume: '', volumeFilter: '',
      query: '', sortNewestFirst: true, hostname: 'webadmin', prevStats: null,
      showHot: true,
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
    const volumeView = document.getElementById('volumeView') as HTMLElement;
    const lockPanelContent = document.getElementById('lockPanelContent') as HTMLElement;
    const lockPanelSkeleton = document.getElementById('lockPanelSkeleton') as HTMLElement;
    this.themeIcon = document.getElementById('themeIcon') as HTMLElement;
    this.refreshBtn = document.getElementById('refreshButton') as HTMLButtonElement;
    this.deleteVolumePanel = document.getElementById('deleteVolumePanel') as HTMLElement;
    this.deleteVolumeBtn = document.getElementById('deleteVolumeBtn') as HTMLButtonElement;
    this.deleteModal = document.getElementById('deleteModal') as HTMLElement;
    this.deleteModalVolumeName = document.getElementById('deleteModalVolumeName') as HTMLElement;
    this.deleteConfirmInput = document.getElementById('deleteConfirmInput') as HTMLInputElement;
    this.deleteModalConfirm = document.getElementById('deleteModalConfirm') as HTMLButtonElement;
    this.deleteModalCancel = document.getElementById('deleteModalCancel') as HTMLButtonElement;
    this.checkBtn = document.getElementById('volumeCheckButton') as HTMLButtonElement;
    this.repairBtn = document.getElementById('volumeRepairButton') as HTMLButtonElement;
    this.themeToggle = document.getElementById('themeToggle') as HTMLButtonElement;
    this.tabSnapshots = document.getElementById('tabSnapshots') as HTMLButtonElement;
    this.tabRepo = document.getElementById('tabRepo') as HTMLButtonElement;
    this.snapshotsTab = document.getElementById('snapshotsTab') as HTMLElement;
    this.repoTab = document.getElementById('repoTab') as HTMLElement;
    this.initRepoBtn = document.getElementById('initRepoButton') as HTMLButtonElement;
    this.initBanner = document.getElementById('repoInitBanner') as HTMLDivElement;
    this.volumeView = volumeView;
    this.landingPanel = document.getElementById('landingPanel') as HTMLElement;
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
        this.statsMgr.load(vol).catch(e => App.setBanner(e.message, true));
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

    const showHotToggle = document.getElementById('showHotToggle') as HTMLInputElement;
    showHotToggle.addEventListener('change', () => {
      this.state.showHot = showHotToggle.checked;
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

    this.deleteVolumeBtn.addEventListener('click', () => {
      const vol = this.state.selectedVolume;
      if (!vol) return;
      this.deleteModalVolumeName.textContent = vol;
      this.deleteConfirmInput.value = '';
      this.deleteConfirmInput.placeholder = `Type "${vol}" to confirm`;
      this.deleteModalConfirm.disabled = true;
      this.deleteModal.style.display = '';
    });

    this.deleteModalCancel.addEventListener('click', () => {
      this.deleteModal.style.display = 'none';
    });

    this.deleteModal.addEventListener('click', (e) => {
      if (e.target === this.deleteModal) this.deleteModal.style.display = 'none';
    });
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') this.deleteModal.style.display = 'none';
    });

    this.deleteConfirmInput.addEventListener('input', () => {
      const vol = this.state.selectedVolume;
      this.deleteModalConfirm.disabled = this.deleteConfirmInput.value !== vol;
    });

    this.deleteModalConfirm.addEventListener('click', async () => {
      const vol = this.state.selectedVolume;
      if (!vol || this.deleteConfirmInput.value !== vol) return;
      this.deleteModalConfirm.disabled = true;
      this.deleteModalConfirm.textContent = 'Deleting...';
      try {
        const resp = await fetch(`/api/volume/${encodeURIComponent(vol)}`, { method: 'DELETE' });
        if (!resp.ok) {
          const d = await resp.json();
          throw new Error(d.error || 'delete failed');
        }
        this.deleteModal.style.display = 'none';
        App.setBanner(`Volume ${vol} deleted — updating...`);
        this.state.selectedVolume = '';
        this.pillsMgr.showSkeleton();
        this.onVolumeChange('');
        try { await fetch('/api/stats/refresh', { method: 'POST' }); } catch {}
        await this.pillsMgr.load();
      } catch (err) {
        App.setBanner((err as Error).message, true);
      } finally {
        this.deleteModalConfirm.disabled = false;
        this.deleteModalConfirm.textContent = 'Delete';
      }
    });

    this.themeToggle.addEventListener('click', () => {
      const isLight = document.body.classList.toggle('light');
      this.themeIcon.innerHTML = isLight ? this.moonSvg : this.sunSvg;
    });

    // Restore point info tooltip (appended to body to avoid overflow clipping)
    const rpInfo = document.querySelector('.restore-point-info') as HTMLElement;
    if (rpInfo) {
      let tooltipEl: HTMLDivElement | null = null;
      const tipText = rpInfo.getAttribute('data-tip') || '';
      rpInfo.addEventListener('mouseenter', () => {
        if (!tipText) return;
        tooltipEl = document.createElement('div');
        tooltipEl.textContent = tipText;
        tooltipEl.style.cssText = 'position:fixed;z-index:1000;background:var(--surface);color:var(--text);font-size:0.8rem;padding:10px 14px;border-radius:10px;border:1px solid var(--border);box-shadow:var(--shadow);white-space:normal;width:280px;text-align:left;line-height:1.4;pointer-events:none;';
        const rect = rpInfo.getBoundingClientRect();
        tooltipEl.style.left = Math.max(4, rect.left + rect.width / 2 - 140) + 'px';
        tooltipEl.style.top = (rect.bottom + 8) + 'px';
        document.body.appendChild(tooltipEl);
      });
      rpInfo.addEventListener('mouseleave', () => {
        if (tooltipEl) {
          tooltipEl.remove();
          tooltipEl = null;
        }
      });
    }

    const switchTab = (tab: 'snapshots' | 'repo') => {
      const isSnap = tab === 'snapshots';
      this.tabSnapshots.classList.toggle('tab-active', isSnap);
      this.tabRepo.classList.toggle('tab-active', !isSnap);
      this.snapshotsTab.style.display = isSnap ? '' : 'none';
      this.repoTab.style.display = isSnap ? 'none' : '';
      if (tab === 'repo') {
        const vol = this.state.selectedVolume;
        if (vol) this.statsMgr.load(vol).catch(() => {});
      }
    };

    this.tabSnapshots.addEventListener('click', () => switchTab('snapshots'));
    this.tabRepo.addEventListener('click', () => switchTab('repo'));
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
      this.testStatus.textContent = 'Updating volume list...';
      this.testStatus.style.color = '';
      this.testVolumeInput.value = '';
      this.pillsMgr.showSkeleton();
      try { await fetch('/api/stats/refresh', { method: 'POST' }); } catch {}
      await this.pillsMgr.load();
      this.state.selectedVolume = name;
      this.pillsMgr.render();
      this.onVolumeChange(name);
    } catch (err) {
      this.testStatus.textContent = (err as Error).message;
      this.testStatus.style.color = 'var(--red)';
    } finally {
      this.testCreateBtn.disabled = false;
      this.testCreateBtn.textContent = 'Create & back up';
    }
  }

  private onVolumeChange(vol: string): void {
    document.dispatchEvent(new CustomEvent('close-snapshot-viewer'));
    this.deleteModal.style.display = 'none';
    this.landingPanel.style.display = vol ? 'none' : '';
    if (vol) {
      this.volumeView.style.display = 'block';
      this.snapshotsTab.style.display = '';
      this.repoTab.style.display = 'none';
      this.tabSnapshots.classList.add('tab-active');
      this.tabRepo.classList.remove('tab-active');
      this.snapMgr.load(vol).catch(() => {});
      this.lockMgr.refresh();
      this.statsGrid.innerHTML = '';
      this.statsMgr.load(vol).catch(() => {});
    } else {
      this.volumeView.style.display = 'none';
      this.snapshotsTab.style.display = 'none';
      this.repoTab.style.display = 'none';
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
