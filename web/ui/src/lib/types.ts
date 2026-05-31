export interface Snapshot {
  id: string;
  short_id: string;
  time: string;
  tree?: string;
  tags: string[];
  paths: string[];
  hostname: string;
  volume?: string;
  fallbackHash?: string;
}

export interface LockStatus {
  volume: string;
  locked: boolean;
  owner?: string;
  expires_in?: number;
}

export interface VolumeLockInfo {
  locked: boolean;
  owner: string;
  expiresIn: number;
  status: 'locked' | 'unlocked';
}

export interface RepoStatus {
  initialized: boolean;
  hostname: string;
}

export interface StatsResponse {
  snapshots: {
    total: number;
    hot: number;
    cold: number;
    excluded: number;
    volumes: number;
    newest: string;
    oldest: string;
    hot_volumes?: string[];
    cold_volumes?: string[];
    excluded_volumes?: string[];
    other_volumes?: string[];
  };
  repo: {
    total_size?: number;
    total_file_count?: number;
    total_blob_count?: number;
    total_uncompressed_size?: number;
    unique_blob_count?: number;
    unique_blob_size?: number;
    error?: string;
  };
  locks: {
    total_volumes: number;
    active: number;
    expired: number;
    unlocked: number;
    active_volumes?: string[];
    expired_volumes?: string[];
  };
  total_volumes?: number;
  cached_at?: string;
}

export interface FileNode {
  name: string;
  type: string;
  path: string;
  full_path?: string;
  size?: number;
  children?: Record<string, FileNode>;
  dirDiffType?: string;
}

export interface DiffChange {
  type: string;
  paths: string[];
}

export interface DiffResult {
  change_sets: DiffChange[];
}

export interface AppState {
	snapshots: Snapshot[];
	volumes: string[];
	selectedVolume: string;
	volumeFilter: string;
	query: string;
	sortNewestFirst: boolean;
	hostname: string;
	prevStats: StatsResponse | null;
	volumesCachedAt?: string;
	typeFilter: string;
	hostFilter: string;
}

export interface SnapshotsResponse {
	snapshots: Snapshot[];
	restorePointID?: string;
}
