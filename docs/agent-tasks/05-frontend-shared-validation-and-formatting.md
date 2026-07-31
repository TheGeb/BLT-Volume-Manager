# Frontend Shared Validation and Formatting

## Goal

Protect snapshot-picker requests and remove duplicated formatting logic.

## Intended change

- Make version parsing reject partially valid strings such as `12x`.
- Make invalid version range bounds explicit instead of silently ignoring them.
- Extract duplicated date/time/version labels from `SnapshotPicker.svelte` and `SnapshotSearch.svelte` into typed utilities.
- Add a request generation check and `AbortController` to `SnapshotPicker.svelte`.
- Do not show an error toast for an expected picker-request abort.
- Preserve existing display strings and formatting unless a test demonstrates an existing bug.

## Scope ownership

- `web/ui/src/lib/util.ts`
- New shared utility files under `web/ui/src/lib/`
- `web/ui/src/components/SnapshotPicker.svelte` for picker request freshness and formatting calls
- `web/ui/src/routes/snapshots/SnapshotSearch.svelte`
- Related unit tests

Do not modify `web/ui/src/lib/api.ts`. The request-state task owns that file, including response parsing and runtime validation. Do not modify main snapshot-list request handling; this task owns picker request freshness only.

## Verification

- Test strict version parsing and invalid bounds.
- Test picker responses cannot overwrite newer picker state.
- Test that picker and search formatting remains visually identical.
- Run `npm test` and `npm run check`.
