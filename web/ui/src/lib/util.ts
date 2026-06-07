export function formatBytes(b: number): string {
  if (b <= 0) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), units.length - 1);
  const val = b / Math.pow(1024, i);
  const formatted = val.toFixed(2).replace(new RegExp('\\.?0+$'), '');
  // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
  return `${formatted} ${units[i]!}`;
}

export function formatDuration(totalSeconds: number): string {
  if (totalSeconds <= 0) return 'expired';
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const secs = totalSeconds % 60;
  const parts: string[] = [];
  if (days > 0) parts.push(`${String(days)}d`);
  if (hours > 0 || days > 0) parts.push(`${String(hours)}h`);
  if (minutes > 0 || hours > 0 || days > 0) parts.push(`${String(minutes)}m`);
  parts.push(`${String(secs)}s`);
  return parts.join(' ') + ' remaining';
}

export function formatExpiration(totalSeconds: number): string {
  if (totalSeconds <= 0) return 'Expired';
  const expDate = new Date(Date.now() + totalSeconds * 1000);
  return `Expires: ${expDate.toLocaleString()}`;
}

export function escapeHtml(s: string | undefined): string {
	if (!s) return '';
	const div = document.createElement('div');
	div.textContent = s;
	return div.innerHTML;
}

export function versionTag(tags: string[]): string | undefined {
	return tags.find(t => /^v\d+\.\d+$/.test(t));
}

export function parseVersion(v: string): { major: number; minor: number } | null {
	const cleaned = v.replace(/^v/, '');
	const parts = cleaned.split('.');
	const major = parseInt(parts[0] ?? '');
	const minor = parts[1] ? parseInt(parts[1]) : 0;
	if (isNaN(major) || isNaN(minor)) return null;
	return { major, minor };
}

export function matchesVersionRange(
	tags: string[],
	from?: string,
	to?: string,
): boolean {
	if (!from && !to) return true;
	const vt = tags.find(t => /^v\d+\.\d+$/.test(t));
	if (!vt) return false;
	const sv = parseVersion(vt);
	if (!sv) return false;
	if (from) {
		const fv = parseVersion(from);
		if (fv && (sv.major < fv.major || (sv.major === fv.major && sv.minor < fv.minor))) return false;
	}
	if (to) {
		const tv = parseVersion(to);
		if (tv && (sv.major > tv.major || (sv.major === tv.major && sv.minor > tv.minor))) return false;
	}
	return true;
}