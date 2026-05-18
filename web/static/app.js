const state = {
  snapshots: [],
  volumes: [],
  selectedVolume: '',
  query: '',
  sortNewestFirst: true,
  hostname: 'webadmin',
};

const snapshotTable = document.getElementById('snapshotTable');
const searchInput = document.getElementById('searchInput');
const refreshButton = document.getElementById('refreshButton');
const sortButton = document.getElementById('sortButton');
const themeToggle = document.getElementById('themeToggle');
const statusMessage = document.getElementById('statusMessage');
const volumeSelect = document.getElementById('volumeSelect');
const createLockButton = document.getElementById('createLockButton');
const deleteLocksButton = document.getElementById('deleteLocksButton');
const lockStatus = document.getElementById('lockStatus');
const repoInitBanner = document.getElementById('repoInitBanner');
const initRepoButton = document.getElementById('initRepoButton');

window.addEventListener('load', () => {
  searchInput.addEventListener('input', () => {
    state.query = searchInput.value.trim().toLowerCase();
    renderTable();
  });

  volumeSelect.addEventListener('change', () => {
    state.selectedVolume = volumeSelect.value;
    renderTable();
    refreshLockStatus();
  });

  createLockButton.addEventListener('click', () => {
    if (!state.selectedVolume) {
      alert('Select a volume first.');
      return;
    }
    createVolumeLock(state.selectedVolume);
  });

  refreshButton.addEventListener('click', loadSnapshots);
  sortButton.addEventListener('click', () => {
    state.sortNewestFirst = !state.sortNewestFirst;
    sortButton.textContent = state.sortNewestFirst ? 'Sort by newest' : 'Sort by oldest';
    renderTable();
  });

  deleteLocksButton.addEventListener('click', () => {
    if (!state.selectedVolume) {
      alert('Select a volume before deleting locks.');
      return;
    }
    if (!confirm(`Delete host-lock objects for volume ${state.selectedVolume}?`)) {
      return;
    }
    deleteVolumeLocks(state.selectedVolume);
  });

  initRepoButton.addEventListener('click', initRepo);

  themeToggle.addEventListener('click', () => {
    document.body.classList.toggle('light');
  });

  checkRepoStatus();
  loadSnapshots();
});

async function loadSnapshots() {
  showStatus('Loading snapshots...');

  try {
    const response = await fetch('/api/snapshots');
    if (!response.ok) {
      throw new Error('Unable to load snapshots');
    }

    state.snapshots = await response.json();
    state.snapshots = state.snapshots.map(snapshot => ({
      ...snapshot,
      tags: Array.isArray(snapshot.tags) ? snapshot.tags : [],
      paths: Array.isArray(snapshot.paths) ? snapshot.paths : [],
    }));
    state.volumes = computeVolumes(state.snapshots);
    renderVolumeOptions();
    renderTable();
    showStatus(`Loaded ${state.snapshots.length} snapshots.`);
  } catch (error) {
    showStatus(error.message, true);
  }
}

function computeVolumes(snapshots) {
  const set = new Set();
  snapshots.forEach(snapshot => {
    snapshot.paths.forEach(path => {
      const volume = extractVolumeName(path);
      if (volume) {
        set.add(volume);
      }
    });
  });
  return Array.from(set).sort();
}

function renderVolumeOptions() {
  volumeSelect.innerHTML = '<option value="">All volumes</option>';
  state.volumes.forEach(volume => {
    const option = document.createElement('option');
    option.value = volume;
    option.textContent = volume;
    if (volume === state.selectedVolume) {
      option.selected = true;
    }
    volumeSelect.appendChild(option);
  });
}

function extractVolumeName(path) {
  const marker = '/volumes/';
  const idx = path.indexOf(marker);
  if (idx >= 0) {
    const subpath = path.slice(idx + marker.length).replace(/^\//, '');
    const parts = subpath.split('/');
    return parts[0] || '';
  }
  const parts = path.split('/').filter(Boolean);
  return parts.length ? parts[parts.length - 1] : '';
}

function renderTable() {
  snapshotTable.innerHTML = '';

  const filtered = state.snapshots
    .filter(snapshot => {
      if (state.selectedVolume && !snapshot.paths.some(path => extractVolumeName(path) === state.selectedVolume)) {
        return false;
      }
      if (!state.query) {
        return true;
      }
      const pathText = snapshot.paths.join(' ');
      const tagText = snapshot.tags.join(' ');
      return [snapshot.id, snapshot.short_id, pathText, tagText]
        .join(' ')
        .toLowerCase()
        .includes(state.query);
    })
    .sort((a, b) => {
      const aTime = new Date(a.time).getTime();
      const bTime = new Date(b.time).getTime();
      return state.sortNewestFirst ? bTime - aTime : aTime - bTime;
    });

  if (filtered.length === 0) {
    snapshotTable.innerHTML = '<tr><td colspan="5">No backups match the current filter.</td></tr>';
    return;
  }

  filtered.forEach(snapshot => {
    const row = document.createElement('tr');

    const idCell = document.createElement('td');
    idCell.className = 'copy-id';
    idCell.title = 'Click to copy full snapshot ID';
    idCell.textContent = snapshot.short_id;
    idCell.addEventListener('click', () => {
      navigator.clipboard.writeText(snapshot.id);
      showStatus('Snapshot ID copied to clipboard');
      idCell.classList.add('copied');
      setTimeout(() => idCell.classList.remove('copied'), 1500);
    });
    row.appendChild(idCell);

    const pathsCell = document.createElement('td');
    pathsCell.textContent = snapshot.paths.join(', ');
    row.appendChild(pathsCell);

    const tagsCell = document.createElement('td');
    const tagList = document.createElement('div');
    tagList.className = 'tag-list';
    const visibleTags = snapshot.tags.filter(t => t === 'hot' || t === 'cold');
    const isExcluded = snapshot.tags.includes('excluded');
    if (visibleTags.length === 0) {
      tagList.textContent = 'No tags';
    } else {
      visibleTags.forEach(tag => {
        const tagItem = document.createElement('span');
        tagItem.className = 'tag tag-readonly';
        tagItem.textContent = tag;
        tagList.appendChild(tagItem);
      });
    }
    const label = document.createElement('label');
    label.className = 'tag-checkbox';
    const cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.checked = isExcluded;
    cb.addEventListener('change', () => {
      if (cb.checked) {
        addTag(snapshot.id, 'excluded');
      } else {
        removeTag(snapshot.id, 'excluded');
      }
    });
    label.appendChild(cb);
    label.appendChild(document.createTextNode(' excluded'));
    tagList.appendChild(label);
    tagsCell.appendChild(tagList);
    row.appendChild(tagsCell);

    const timeCell = document.createElement('td');
    timeCell.textContent = new Date(snapshot.time).toLocaleString();
    row.appendChild(timeCell);

    row.appendChild(document.createElement('td'));
    snapshotTable.appendChild(row);
  });
}

async function addTag(snapshotID, tag) {
  showStatus(`Adding tag “${tag}”...`);
  try {
    const response = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}`, {
      method: 'POST',
    });
    if (!response.ok) {
      throw new Error('Failed to add tag');
    }
    await loadSnapshots();
  } catch (error) {
    showStatus(error.message, true);
  }
}

async function removeTag(snapshotID, tag) {
  showStatus(`Removing tag “${tag}”...`);
  try {
    const response = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      throw new Error('Failed to remove tag');
    }
    await loadSnapshots();
  } catch (error) {
    showStatus(error.message, true);
  }
}

async function refreshLockStatus() {
  if (!state.selectedVolume) {
    lockStatus.textContent = '';
    return;
  }
  try {
    const response = await fetch(`/api/volume/${encodeURIComponent(state.selectedVolume)}/locks`);
    if (!response.ok) {
      lockStatus.textContent = '';
      return;
    }
    const data = await response.json();
    if (data.locked) {
      const remaining = data.expires_in > 0 ? `${data.expires_in}s remaining` : 'expired';
      lockStatus.textContent = `Locked by ${data.owner} (${remaining})`;
      lockStatus.className = 'lock-status locked';
    } else {
      lockStatus.textContent = 'Unlocked';
      lockStatus.className = 'lock-status unlocked';
    }
  } catch {
    lockStatus.textContent = '';
  }
}

async function createVolumeLock(volumeName) {
  const ownerName = prompt('Lock owner name:', state.hostname);
  if (!ownerName) {
    return;
  }
  showStatus(`Creating lock for volume ${volumeName}...`);
  try {
    const response = await fetch(`/api/volume/${encodeURIComponent(volumeName)}/locks`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ owner: ownerName }),
    });
    if (!response.ok) {
      const body = await response.json();
      throw new Error(body.error || 'Failed to create lock');
    }
    showStatus(`Lock created for volume ${volumeName}.`);
    await refreshLockStatus();
  } catch (error) {
    showStatus(error.message, true);
  }
}

async function deleteVolumeLocks(volumeName) {
  showStatus(`Deleting locks for volume ${volumeName}...`);
  try {
    const response = await fetch(`/api/volume/${encodeURIComponent(volumeName)}/locks`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      const body = await response.json();
      throw new Error(body.error || 'Failed to delete locks');
    }
    showStatus(`Deleted locks for volume ${volumeName}.`);
    await refreshLockStatus();
  } catch (error) {
    showStatus(error.message, true);
  }
}

async function checkRepoStatus() {
  try {
    const response = await fetch('/api/repo/status');
    if (!response.ok) return;
    const data = await response.json();
    repoInitBanner.style.display = data.initialized ? 'none' : 'flex';
    if (data.hostname) state.hostname = data.hostname;
  } catch {
    // ignore
  }
}

async function initRepo() {
  initRepoButton.disabled = true;
  initRepoButton.textContent = 'Initializing...';
  try {
    const response = await fetch('/api/repo/init', { method: 'POST' });
    if (!response.ok) {
      const body = await response.json();
      throw new Error(body.error || 'Failed to initialize repository');
    }
    repoInitBanner.style.display = 'none';
    loadSnapshots();
  } catch (error) {
    showStatus(error.message, true);
  } finally {
    initRepoButton.disabled = false;
    initRepoButton.textContent = 'Initialize Repository';
  }
}

function showStatus(message, isError = false) {
  statusMessage.textContent = message;
  statusMessage.style.color = isError ? '#f87171' : '';
}
