export function computeDateLabel(
  tf: number | undefined, tt: number | undefined,
  tdf: number | undefined, tdt: number | undefined,
): string {
  if (tdf !== undefined || tdt !== undefined) {
    return `${fmtSOT(tdf)}\u2013${fmtSOT(tdt)}`;
  }
  if (tf !== undefined || tt !== undefined) {
    const from = fmtTS(tf);
    const to = fmtTS(tt);
    if (tf && tt && sameUTCDay(tf, tt)) {
      const ft = timePart(tf);
      const ttPart = timePart(tt);
      if (!ft && !ttPart) return from;
      const date = from.split(' (')[0] ?? from;
      return `${date} (${ft || '12 AM'} \u2013 ${ttPart || '12 AM'})`;
    }
    return from + ' \u2013 ' + to;
  }
  return 'Any date';
}

function sameUTCDay(a: number, b: number): boolean {
  const da = new Date(a);
  const db = new Date(b);
  return da.getUTCFullYear() === db.getUTCFullYear() && da.getUTCMonth() === db.getUTCMonth() && da.getUTCDate() === db.getUTCDate();
}

function timePart(ts: number | undefined): string {
  if (ts === undefined) return '';
  const d = new Date(ts);
  const h = d.getUTCHours();
  const m = d.getUTCMinutes();
  const s = d.getUTCSeconds();
  if (h === 0 && m === 0 && s === 0) return '';
  if (h === 23 && m === 59 && s === 59) return '';
  return formatTime(h, m, s);
}

function fmtTS(ts: number | undefined): string {
  if (ts === undefined) return '\u2026';
  const d = new Date(ts);
  const date = String(d.getUTCMonth() + 1) + '/' + String(d.getUTCDate());
  const h = d.getUTCHours();
  const m = d.getUTCMinutes();
  const s = d.getUTCSeconds();
  const isDefault = (h === 0 && m === 0 && s === 0) || (h === 23 && m === 59 && s === 59);
  if (isDefault) return date;
  return date + ' (' + formatTime(h, m, s) + ')';
}

function fmtSOT(s: number | undefined): string {
  if (s === undefined) return '--:--';
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  return formatTime(h, m, sec);
}

function formatTime(h: number, m: number, s: number): string {
  const ampm = h >= 12 ? 'PM' : 'AM';
  const h12 = h % 12 || 12;
  let result = String(h12);
  if (m !== 0 || s !== 0) result += ':' + String(m).padStart(2, '0');
  if (s !== 0) result += ':' + String(s).padStart(2, '0');
  return result + ' ' + ampm;
}

export function computeVersionLabel(fm: string, fn: string, tm: string, tn: string): string {
  const from = fmtVersion(fm, fn);
  const to = fmtVersion(tm, tn);
  const fromEmpty = !from || from === '0';
  const toEmpty = !to || to === '0';
  if (!fromEmpty && !toEmpty) return `v${from} - v${to}`;
  if (!fromEmpty) return `v${from}`;
  if (!toEmpty) return `v${to}`;
  return 'Any version';
}

function fmtVersion(major: string, minor: string): string {
  const mNum = parseInt(major || '0', 10);
  const nStr = minor || '0';
  const nNum = parseInt(nStr, 10);
  if (nNum === 0) return String(mNum);
  const trimmed = nStr.replace(/0+$/, '');
  return trimmed ? `${String(mNum)}.${trimmed}` : String(mNum);
}
