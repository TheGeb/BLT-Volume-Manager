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

export interface OwnerStatus {
  volume: string;
  owned: boolean;
  owner?: string;
  expires_in?: number;
}

export interface VolumeOwnerInfo {
  owned: boolean;
  owner: string;
  expiresIn: number;
  status: 'owned' | 'unclaimed';
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
  owners: {
    total_volumes: number;
    active: number;
    expired: number;
    unclaimed: number;
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

export interface SnapshotsResponse {
	snapshots: Snapshot[];
	restorePointID?: string;
	hasMore?: boolean;
}

export interface BatchDeleteResponse {
	deleted: number;
	failed: number;
	errors: { id: string; error: string }[];
}
