# Frontend Request Freshness and Error States

## Goal

Prevent older asynchronous responses from overwriting newer UI state and distinguish unavailable data from valid empty/unclaimed data.

## Intended change

- Add a request generation counter to snapshot list loading.
- Abort the previous fetch when starting a new one using `AbortController`.
- Treat aborts as expected cancellation, not user-facing errors.
- Apply a response only if its generation is still current.
- Include volume, filters, page, and page size in the request state that determines freshness.
- Centralize response parsing in this task's API helper, including JSON/text fallback and consistent HTTP errors.
- Add narrow runtime validation for the response shapes used by snapshot, owner, and file-loading requests.
- Keep owner status as `null` or an explicit error state when the request fails; never represent failure as an empty owner.
- Store file-content errors separately from file content and render a dedicated error state.
- Use `unknown` in catches and a local safe error-message helper where needed.

## API helper ownership

This task owns all changes to `web/ui/src/lib/api.ts`, including fetch signal plumbing, response parsing, and runtime validation. Preserve existing URL construction and successful response shapes.

## Scope ownership

- `web/ui/src/lib/api.ts` for request signals, response parsing, and runtime validation
- `web/ui/src/lib/stores/snapshots.ts`
- `web/ui/src/lib/stores/repo.ts`
- `web/ui/src/routes/snapshots/SnapshotViewer.svelte`
- Focused frontend tests

This task also owns removal of the empty `reconcileViewerSnapshots` helper if no real reconciliation behavior is added.

Do not modify shared formatting utilities or accessibility markup outside the file-content controls.

## Verification

- Test that an older snapshot response cannot replace a newer response.
- Test aborts do not create error toasts.
- Test owner-status failures are not rendered as unclaimed.
- Run `npm test` and `npm run check`.
