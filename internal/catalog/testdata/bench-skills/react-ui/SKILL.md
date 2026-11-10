---
name: react-ui
description: Build accessible React screens with a shared component layer and predictable state.
---

# React UI

Use this skill when adding a screen, changing a component, or fixing a layout
issue in the web app.

## Quick Start

Create the route, add the screen component, and register the navigation entry.
Start the dev server and confirm the screen renders with empty, loading, and
error states before wiring real data.

## Component Rules

Every interactive element has a visible focus state, a hover state, and a
disabled state. Buttons show a pending state while an action runs and cannot be
submitted twice. Use the shared primitives instead of raw elements so the
spacing scale and radius stay consistent.

## State

Server state lives in the query layer. Local UI state stays inside the
component that owns it. Never copy server data into component state, because
the copy goes stale after a mutation. Forms keep their own draft state and
submit through one mutation with optimistic rollback on failure.

## Accessibility

Labels are programmatically associated with inputs. Error text is announced
with a live region. Keyboard order follows visual order. Dialogs trap focus,
close on Escape, and return focus to the trigger. Check contrast at 4.5 to 1
for body text and 3 to 1 for boundaries.

## Layout

The layout uses a small spacing scale. Sections align to the grid, and long
content is capped at a readable measure. Test at 360px, 768px, and 1440px.
Nothing scrolls horizontally, and touch targets stay at least 44px.

## Testing

Component tests cover behavior, not implementation. Assert on roles and
visible text. Add one test per state transition and one for the error path.
Run the type checker and the full test suite before opening a pull request.

## Performance

Measure before optimizing. Split routes, defer heavy charts, and keep images
sized and lazy. A screen that loads in under a second on a normal connection
does not need memoization added by reflex.
