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
  owner: string;
  expiry?: number;
}

export interface VolumeOwnerInfo {
  owner: string;
  expiry: number;
  status: 'owned' | 'unclaimed';
}

export interface RepoStatus {
  initialized: boolean;
  hostname: string;
}

export interface StatsResponse {
  repo: {
    total_size?: number;
    total_file_count?: number;
    total_blob_count?: number;
    total_uncompressed_size?: number;
    unique_blob_count?: number;
  };
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
