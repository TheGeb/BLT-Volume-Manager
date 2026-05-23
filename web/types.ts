interface Snapshot {
  id: string;
  short_id: string;
  time: string;
  tags: string[];
  paths: string[];
  hostname: string;
  volume: string;
}

interface LockStatus {
  volume: string;
  locked: boolean;
  owner?: string;
  expires_in?: number;
}

interface RepoStatus {
  initialized: boolean;
  hostname: string;
}

interface StatsResponse {
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
    compressed_size?: number;
    unique_blob_count?: number;
    unique_blob_size?: number;
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

interface AppState {
  snapshots: Snapshot[];
  volumes: string[];
  selectedVolume: string;
  volumeFilter: string;
  query: string;
  sortNewestFirst: boolean;
  hostname: string;
  prevStats: StatsResponse | null;
  pillsCachedAt?: string;
  showHot: boolean;
}
