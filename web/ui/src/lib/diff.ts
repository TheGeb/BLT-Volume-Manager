export interface DiffLine {
  type: 'add' | 'del' | 'ctx';
  oldLineNo: number;
  newLineNo: number;
  content: string;
}

export interface DiffHunk {
  oldStart: number;
  oldLen: number;
  newStart: number;
  newLen: number;
  lines: DiffLine[];
}

const CONTEXT_LINES = 3;

interface DpTable {
  get(i: number, j: number): number;
  set(i: number, j: number, val: number): void;
}

function makeDpTable(rows: number, cols: number): DpTable {
  const data: number[][] = Array.from({ length: rows }, (): number[] => new Array(cols).fill(0) as number[]);
  return {
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    get(i: number, j: number): number { return data[i]![j]!; },
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    set(i: number, j: number, val: number): void { data[i]![j] = val; },
  };
}

function lcsTable(a: string[], b: string[]): DpTable {
  const m = a.length, n = b.length;
  const dp = makeDpTable(m + 1, n + 1);
  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
      const ai = a[i - 1]!;
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
      const bj = b[j - 1]!;
      dp.set(i, j, ai === bj ? dp.get(i - 1, j - 1) + 1 : Math.max(dp.get(i - 1, j), dp.get(i, j - 1)));
    }
  }
  return dp;
}

function backtrack(dp: DpTable, a: string[], b: string[], i: number, j: number, lines: DiffLine[]): void {
  if (i === 0 && j === 0) return;
  if (i > 0 && j > 0 && a[i - 1] === b[j - 1]) {
    backtrack(dp, a, b, i - 1, j - 1, lines);
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    lines.push({ type: 'ctx', oldLineNo: i, newLineNo: j, content: a[i - 1]! });
  } else if (j > 0 && (i === 0 || dp.get(i, j - 1) >= dp.get(i - 1, j))) {
    backtrack(dp, a, b, i, j - 1, lines);
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    lines.push({ type: 'add', oldLineNo: 0, newLineNo: j, content: b[j - 1]! });
  } else if (i > 0) {
    backtrack(dp, a, b, i - 1, j, lines);
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    lines.push({ type: 'del', oldLineNo: i, newLineNo: 0, content: a[i - 1]! });
  }
}

function groupHunks(lines: DiffLine[]): DiffHunk[] {
  const hunks: DiffHunk[] = [];
  let cur: DiffLine[] = [];
  let ctxCount = 0;
  for (const line of lines) {
    if (line.type === 'ctx') {
      ctxCount++;
      if (ctxCount > CONTEXT_LINES) {
        if (cur.length >= CONTEXT_LINES) {
          const ctxBefore = cur.slice(-CONTEXT_LINES);
          const firstNonCtx = ctxBefore.findIndex(l => l.type !== 'ctx');
          const keepCtx = firstNonCtx === -1 ? CONTEXT_LINES : CONTEXT_LINES - firstNonCtx;
          const hunkLines = cur.slice(0, -keepCtx);
          if (hunkLines.length > 0) {
            hunks.push(buildHunk(hunkLines));
          }
        }
        cur = [];
        ctxCount = 1;
        cur.push(line);
      } else {
        cur.push(line);
      }
    } else {
      ctxCount = 0;
      cur.push(line);
    }
  }
  if (cur.length > 0 && cur.some(l => l.type !== 'ctx')) {
    hunks.push(buildHunk(cur));
  }
  return hunks;
}

function buildHunk(lines: DiffLine[]): DiffHunk {
  let oldStart = 0, newStart = 0;
  for (const l of lines) {
    if (l.type !== 'add' && oldStart === 0) oldStart = l.oldLineNo;
    if (l.type !== 'del' && newStart === 0) newStart = l.newLineNo;
    if (oldStart > 0 && newStart > 0) break;
  }
  const oldLen = lines.filter(l => l.type !== 'add').length;
  const newLen = lines.filter(l => l.type !== 'del').length;
  return { oldStart, oldLen, newStart, newLen, lines };
}

export function computeDiff(oldLines: string[], newLines: string[]): DiffHunk[] {
  const dp = lcsTable(oldLines, newLines);
  const lines: DiffLine[] = [];
  backtrack(dp, oldLines, newLines, oldLines.length, newLines.length, lines);
  return groupHunks(lines);
}