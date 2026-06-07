<script lang="ts">
  import { RangeCalendar, Portal } from 'bits-ui';
  import { CalendarDate, type DateValue, Time } from '@internationalized/date';
  import DropSelect from '../../components/DropSelect.svelte';

  export let timeFrom: number | undefined = undefined;
  export let timeTo: number | undefined = undefined;
  export let sortNewestFirst = true;
  export let onToggleSort: () => void = () => {};
  export let onTimeFilter: (from?: number, to?: number) => void = () => {};
  export let timeOfDayFrom: number | undefined = undefined;
  export let timeOfDayTo: number | undefined = undefined;
  export let onTimeOfDayFilter: (from?: number, to?: number) => void = () => {};

  function dateValueToTs(dv: DateValue | undefined): number | undefined {
    if (!dv) return undefined;
    return Date.UTC(dv.year, dv.month - 1, dv.day, 23, 59, 59, 999);
  }

  function tsToDateValue(ts: number | undefined): DateValue | undefined {
    if (ts === undefined) return undefined;
    const d = new Date(ts);  
    return new CalendarDate(d.getUTCFullYear(), d.getUTCMonth() + 1, d.getUTCDate());
  }

  function timeToSeconds(t: Time | undefined): number | undefined {
    if (!t) return undefined;
    return t.hour * 3600 + t.minute * 60 + t.second;
  }

  function secondsToTime(s: number | undefined, def: Time): Time {
    if (s === undefined) return def;
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    return new Time(h, m, sec);
  }

  let dateRange: { start: DateValue | undefined; end: DateValue | undefined } = {
    start: undefined,
    end: undefined,
  };
  let timeRange: { start: Time | undefined; end: Time | undefined } = {
    start: undefined,
    end: undefined,
  };

  let appliedDateFrom: number | undefined = timeFrom;
  let appliedDateTo: number | undefined = timeTo;
  let appliedTimeFrom: number | undefined = timeOfDayFrom;
  let appliedTimeTo: number | undefined = timeOfDayTo;

  $: hasAppliedFilter = timeFrom !== undefined || timeTo !== undefined
    || timeOfDayFrom !== undefined || timeOfDayTo !== undefined;

  $: hasStagedChanges = dateRange.start !== undefined || dateRange.end !== undefined
    || timeRange.start !== undefined || timeRange.end !== undefined;

  function loadFromProps() {
    dateRange = { start: tsToDateValue(timeFrom), end: tsToDateValue(timeTo) };
    timeRange = {
      start: timeOfDayFrom !== undefined ? secondsToTime(timeOfDayFrom, new Time(0, 0, 0)) : undefined,
      end: timeOfDayTo !== undefined ? secondsToTime(timeOfDayTo, new Time(23, 59, 59)) : undefined,
    };
    populateTimeFields();
    appliedDateFrom = timeFrom;
    appliedDateTo = timeTo;
    appliedTimeFrom = timeOfDayFrom;
    appliedTimeTo = timeOfDayTo;
  }

  function populateTimeFields() {
    const fill = (t: Time | undefined, setH: (v: string) => void, setM: (v: string) => void, setS: (v: string) => void, setA: (v: string) => void) => {
      if (!t) { setH(''); setM(''); setS(''); setA('AM'); return; }
      const h12 = t.hour % 12 || 12;
      setH(String(h12).padStart(2, '0'));
      setM(String(t.minute).padStart(2, '0'));
      setS(String(t.second).padStart(2, '0'));
      setA(t.hour >= 12 ? 'PM' : 'AM');
    };
    fill(timeRange.start, (v) => fromH = v, (v) => fromM = v, (v) => fromS = v, (v) => fromA = v);
    fill(timeRange.end, (v) => toH = v, (v) => toM = v, (v) => toS = v, (v) => toA = v);
  }

  function apply() {
    appliedDateFrom = dateValueToTs(dateRange.start);
    appliedDateTo = dateValueToTs(dateRange.end);
    appliedTimeFrom = timeToSeconds(timeRange.start);
    appliedTimeTo = timeToSeconds(timeRange.end);
    onTimeFilter(appliedDateFrom, appliedDateTo);
    onTimeOfDayFilter(appliedTimeFrom, appliedTimeTo);
    if (hasStagedChanges) {
      open = false;
    }
  }

  function clear() {
    dateRange = { start: undefined, end: undefined };
    timeRange = { start: undefined, end: undefined };
    fromH = ''; fromM = ''; fromS = ''; fromA = 'AM';
    toH = ''; toM = ''; toS = ''; toA = 'AM';
    apply();
  }

  function updateTimeRange() {
    const makeTime = (h: string, m: string, s: string, a: string): Time | undefined => {
      const hr = parseInt(h);
      if (isNaN(hr) || h === '') return undefined;
      const mn = parseInt(m) || 0;
      const sc = parseInt(s) || 0;
      let h24 = hr;
      if (a === 'PM' && hr !== 12) h24 += 12;
      if (a === 'AM' && hr === 12) h24 = 0;
      return new Time(h24, mn, sc);
    };
    timeRange = { start: makeTime(fromH, fromM, fromS, fromA), end: makeTime(toH, toM, toS, toA) };
  }

  function clampNum(v: string, max: number): string {
    const n = parseInt(v);
    if (v === '' || isNaN(n)) return v;
    return String(Math.min(n, max));
  }

  function handleH(v: string, setM: (v: string) => void, setS: (v: string) => void): string {
    const cleaned = v.replace(/[^0-9]/g, '').slice(0, 2);
    if (cleaned.length === 2) { setM('00'); setS('00'); }
    return clampNum(cleaned, 12);
  }
  function handleMS(v: string): string {
    return clampNum(v.replace(/[^0-9]/g, '').slice(0, 2), 59);
  }

  let fromH = ''; let fromM = ''; let fromS = ''; let fromA = 'AM';
  let toH = ''; let toM = ''; let toS = ''; let toA = 'AM';

  let open = false;
  let triggerEl: HTMLElement;
  let panelEl: HTMLElement;

  function toggle() {
    if (!open) loadFromProps();
    open = !open;
  }

  function position() {
    if (!panelEl || !triggerEl) return;
    const b = triggerEl.getBoundingClientRect();
    panelEl.style.top = Math.max(8, b.top - panelEl.offsetHeight - 6) + 'px';
    panelEl.style.left = Math.max(8, Math.min(b.left + b.width / 2 - panelEl.offsetWidth / 2, window.innerWidth - panelEl.offsetWidth - 8)) + 'px';
  }

  $: if (open) requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      position();
      if (panelEl) panelEl.style.opacity = '1';
    });
  });

  function onBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) open = false;
  }
</script>

<div class="filter-wrap">
  <button class="th-label sort-btn" on:click={onToggleSort}>
    <svg width="14" height="8" viewBox="0 0 16 10" fill="currentColor" class="sort-chevron" class:sort-desc={sortNewestFirst}>
      <path d="M3 2l5 6 5-6H3z"/>
    </svg>
    Date
  </button>
  <button
    bind:this={triggerEl}
    class={"filter-btn" + (hasAppliedFilter ? ' active' : '')}
    on:click={toggle}
    aria-label="Filter by date"
  >
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
    </svg>
  </button>
</div>

{#if open}
  <Portal>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="filter-backdrop" on:click={onBackdropClick} on:keydown={(e) => e.key === 'Escape' && (open = false)}>
      <div class="date-filter-content" bind:this={panelEl}>
        <RangeCalendar.Root
          value={dateRange}
          onValueChange={(v) => v !== undefined && (dateRange = v)}
          weekdayFormat="short"
          fixedWeeks={true}
          class="date-filter-calendar"
        >
          {#snippet children({ months, weekdays })}
            <RangeCalendar.Header class="cal-header">
              <RangeCalendar.PrevButton class="cal-nav-btn">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="15 18 9 12 15 6" />
                </svg>
              </RangeCalendar.PrevButton>
              <RangeCalendar.Heading class="cal-heading" />
              <RangeCalendar.NextButton class="cal-nav-btn">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="9 18 15 12 9 6" />
                </svg>
              </RangeCalendar.NextButton>
            </RangeCalendar.Header>
            {#each months as month (month.value)}
              <RangeCalendar.Grid class="cal-grid">
                <RangeCalendar.GridHead>
                  <RangeCalendar.GridRow class="cal-weekday-row">
                    {#each weekdays as day (day)}
                      <RangeCalendar.HeadCell class="cal-weekday">{day.slice(0, 2)}</RangeCalendar.HeadCell>
                    {/each}
                  </RangeCalendar.GridRow>
                </RangeCalendar.GridHead>
                <RangeCalendar.GridBody>
                  {#each month.weeks as weekDates (weekDates)}
                    <RangeCalendar.GridRow class="cal-week-row">
                      {#each weekDates as date (date)}
                        <RangeCalendar.Cell {date} month={month.value} class="cal-cell">
                          <RangeCalendar.Day class="cal-day">{date.day}</RangeCalendar.Day>
                        </RangeCalendar.Cell>
                      {/each}
                    </RangeCalendar.GridRow>
                  {/each}
                </RangeCalendar.GridBody>
              </RangeCalendar.Grid>
            {/each}
          {/snippet}
        </RangeCalendar.Root>
        <div class="time-of-day-section">
          <div class="date-filter-label">Time of day</div>
          <div class="time-range-row">
              <div class="time-range-input-group">
                <span class="time-range-label">from</span>
                <div class="timerangefield-input">
                  <input type="text" placeholder="--" maxlength="2" class="time-segment time-input" bind:value={fromH} on:input={(e) => { fromH = handleH(e.currentTarget.value, (v) => fromM = v, (v) => fromS = v); updateTimeRange(); }}>
                  <span class="time-segment time-literal">:</span>
                  <input type="text" placeholder="00" maxlength="2" class="time-segment time-input" bind:value={fromM} on:input={() => { fromM = handleMS(fromM); updateTimeRange(); }}>
                  <span class="time-segment time-literal">:</span>
                  <input type="text" placeholder="00" maxlength="2" class="time-segment time-input" bind:value={fromS} on:input={() => { fromS = handleMS(fromS); updateTimeRange(); }}>
                  <span class="time-ampm">
                    <DropSelect
                      options={[
                        { value: 'AM', label: 'AM' },
                        { value: 'PM', label: 'PM' },
                      ]}
                      value={fromA}
                      onValueChange={(v) => { fromA = v; updateTimeRange(); }}
                    />
                  </span>
                </div>
              </div>
              <div class="time-range-input-group">
                <span class="time-range-label">to</span>
                <div class="timerangefield-input">
                  <input type="text" placeholder="--" maxlength="2" class="time-segment time-input" bind:value={toH} on:input={(e) => { toH = handleH(e.currentTarget.value, (v) => toM = v, (v) => toS = v); updateTimeRange(); }}>
                  <span class="time-segment time-literal">:</span>
                  <input type="text" placeholder="00" maxlength="2" class="time-segment time-input" bind:value={toM} on:input={() => { toM = handleMS(toM); updateTimeRange(); }}>
                  <span class="time-segment time-literal">:</span>
                  <input type="text" placeholder="00" maxlength="2" class="time-segment time-input" bind:value={toS} on:input={() => { toS = handleMS(toS); updateTimeRange(); }}>
                  <span class="time-ampm">
                    <DropSelect
                      options={[
                        { value: 'AM', label: 'AM' },
                        { value: 'PM', label: 'PM' },
                      ]}
                      value={toA}
                      onValueChange={(v) => { toA = v; updateTimeRange(); }}
                    />
                  </span>
                </div>
              </div>
            </div>
          <div class="filter-actions">
            <button class="apply-btn" class:apply-btn-active={hasStagedChanges} on:click={apply}>Apply</button>
            <button class="clear-btn" class:clear-btn-active={hasStagedChanges} on:click={clear}>Clear</button>
          </div>
        </div>
      </div>
    </div>
  </Portal>
{/if}

<style>
  .sort-btn {
    position: relative;
    font-size: 0.95rem;
    font-weight: 600;
    letter-spacing: 0.01em;
    color: var(--muted);
    background: none;
    border: none;
    cursor: pointer;
    font-family: inherit;
    padding: 0;
    white-space: nowrap;
    display: inline;
    appearance: none;
  }

  .sort-btn:hover {
    color: var(--text);
  }

  .sort-chevron {
    position: absolute;
    left: -18px;
    top: 50%;
    transform: translateY(-50%);
    opacity: 0.8;
    transition: transform 0.15s;
  }

  .sort-desc {
    transform: translateY(-50%) rotate(180deg);
  }

  :global(.filter-btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    padding: 0;
    line-height: 0;
  }

  :global(.filter-btn.active) {
    background: var(--hover-bg);
    color: var(--text);
  }

  :global(.filter-btn:hover),
  :global(.filter-btn[data-state="open"]) {
    background: var(--hover-bg);
    color: var(--text);
  }

  .filter-backdrop {
    position: fixed;
    inset: 0;
    z-index: 50;
  }

  :global(.date-filter-content) {
    position: fixed;
    opacity: 0;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 16px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    display: flex;
    flex-direction: row;
    gap: 16px;
    max-height: calc(var(--visual-viewport-height, 100vh) - 16px);
    overflow: hidden;
  }

  :global(.date-filter-calendar) {
    border: none;
    box-shadow: none;
    padding: 0;
  }

  :global(.cal-header) {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
  }

  :global(.cal-heading) {
    font-size: 0.9rem;
    font-weight: 600;
  }

  :global(.cal-nav-btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    padding: 0;
  }

  :global(.cal-nav-btn:hover) {
    background: var(--hover-bg);
    color: var(--text);
  }

  :global(.cal-grid) {
    width: 100%;
    border-collapse: collapse;
  }

  :global(.cal-weekday-row) {
    display: flex;
  }

  :global(.cal-weekday) {
    flex: 1;
    text-align: center;
    font-size: 0.7rem;
    color: var(--muted);
    padding: 4px 0;
    font-weight: 600;
    text-transform: uppercase;
  }

  :global(.cal-week-row) {
    display: flex;
  }

  :global(.cal-cell) {
    flex: 1;
    text-align: center;
    padding: 0;
  }

  :global(.cal-day) {
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text);
    cursor: pointer;
    font-size: 0.85rem;
    font-family: inherit;
    padding: 0;
  }

  :global(.cal-day:hover) {
    background: var(--hover-bg);
  }

  :global(.cal-day[data-selected]) {
    background: var(--accent);
    color: #fff;
  }

  :global(.cal-day[data-disabled]) {
    color: var(--border);
    pointer-events: none;
  }

  :global(.cal-day[data-outside-month]) {
    opacity: 0.3;
  }

  .time-of-day-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
    justify-content: center;
    min-width: 140px;
  }

  .time-range-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .time-range-input-group {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .time-range-label {
    font-size: 0.65rem;
    color: var(--muted);
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  :global(.timerangefield-input) {
    display: inline-flex;
    align-items: center;
    gap: 1px;
    padding: 5px 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--surface-strong);
    transition: border-color 0.15s, box-shadow 0.15s;
  }

  :global(.timerangefield-input:focus-within) {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px rgb(124 58 237 / 20%);
  }

  :global(.time-segment) {
    padding: 1px 2px;
    border-radius: 3px;
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.85rem;
    color: var(--text);
    white-space: pre;
  }

  :global(.time-segment:hover) {
    background: var(--hover-bg);
  }

  :global(.time-segment:focus) {
    background: var(--hover-bg);
    color: var(--text);
  }

  :global(.time-segment[aria-valuetext="Empty"]) {
    color: var(--muted);
  }

  :global(.time-input) {
    width: 22px;
    background: transparent;
    border: none;
    color: inherit;
    font: inherit;
    text-align: center;
    padding: 0;
  }

  :global(.time-input::placeholder) {
    color: var(--muted);
  }

  :global(.time-input:focus) {
    outline: none;
    background: var(--hover-bg);
    border-radius: 3px;
  }

  :global(.time-literal) {
    color: var(--muted);
    padding: 0 1px;
  }

  :global(.time-ampm .drop-select-trigger) {
    background: transparent;
    border: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
    padding: 0 2px;
    border-radius: 3px;
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.85rem;
  }

  :global(.time-ampm .drop-select-trigger:hover) {
    background: var(--hover-bg);
  }

  :global(.time-ampm .drop-select-trigger.open) {
    background: var(--hover-bg);
  }

  .filter-actions {
    display: flex;
    gap: 6px;
    margin-top: 4px;
  }

  .apply-btn {
    background: var(--hover-bg);
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 12px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.15s, color 0.15s;
  }

  .apply-btn-active {
    background: var(--accent);
    color: #fff;
    border-color: transparent;
  }

  .apply-btn:hover {
    background: var(--hover-bg);
  }

  .apply-btn-active:hover {
    background: color-mix(in srgb, var(--accent) 80%, #000);
  }

  .clear-btn {
    background: var(--hover-bg);
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 10px;
    font-size: 0.75rem;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
  }

  .clear-btn-active {
    background: var(--red-bg);
    color: var(--red);
    border-color: var(--red);
  }

  .clear-btn:hover {
    background: var(--hover-bg);
  }

  .clear-btn-active:hover {
    background: rgb(248 113 113 / 20%);
  }
</style>
