# Accessibility and Native Interactive Controls

## Goal

Replace custom interactive spans/divs with native buttons without changing the visual appearance or interaction layout.

## Required replacements

- `web/ui/src/components/DateTimeRange.svelte:277-296`: replace the AM/PM interactive span with `<button type="button">` and add `aria-pressed`.
- `web/ui/src/components/VolumeTree.svelte:356-370`: replace actionable volume-control spans with buttons and preserve their existing classes, icons, sizing, and event behavior.
- `web/ui/src/routes/snapshots/FileTreeNode.svelte:133-138`: replace the actionable directory row/control with a button where practical and add `aria-expanded`.
- `web/ui/src/routes/snapshots/FileTreeNode.svelte:169-173`: replace the actionable file row/control with a button where practical.

## Visual-preservation rules

- Keep the existing class names and DOM nesting unless a minimal structural change is required for valid button behavior.
- Add CSS resets only where native button styles would alter the current appearance.
- Preserve padding, borders, colors, hover states, focus layout, icon placement, sizing, and transitions.
- Do not convert layout-only spans or divs.
- Do not change existing icon-only buttons except to add accessible names where needed.
- Ensure both Enter and Space activate controls through native button behavior.

## Scope ownership

This task owns only the accessibility markup and CSS needed to preserve the current appearance in the listed components. Do not modify data loading, API behavior, or shared formatting utilities.

## Verification

- Run `npm run check` and `npm run lint`.
- Manually verify each listed control in normal, hover, focus, disabled, and active states.
- Verify keyboard activation with Enter and Space.
