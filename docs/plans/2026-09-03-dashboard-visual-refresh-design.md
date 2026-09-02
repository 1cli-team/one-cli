# Dashboard visual refresh

## Design read

One CLI Dashboard is a local developer control surface for technical users. The redesign should feel precise, calm, and trustworthy while keeping the existing logo, orange brand color, routes, content hierarchy, and interaction model.

Design dials:

- `DESIGN_VARIANCE: 5` - structured product UI with a few asymmetric emphasis points
- `MOTION_INTENSITY: 3` - hover, focus, and pressed feedback only
- `VISUAL_DENSITY: 6` - efficient enough for configuration work without sacrificing readability

## Direction

Use a restrained engineering-console language rather than a marketing-page treatment. Keep the cool neutral palette and the existing orange accent. Increase the typographic floor, reduce micro-labels, and create hierarchy through spacing and surface contrast instead of repeated borders.

The shape system is explicit: controls use an 8px radius, panels use 12px, and small badges use 6px. Light and dark modes share the same hierarchy. Orange is reserved for selected state, primary actions, and focus indicators.

## Scope

1. Recalibrate global typography, surfaces, radii, and shadows.
2. Increase the top bar and page spacing so the application chrome feels deliberate.
3. Recompose the Workspace home cards for faster scanning.
4. Improve the Workspace detail hierarchy: workspace header, project rail, tabs, forms, and secret management.
5. Apply the same system to machine-level settings.

No routes, field names, APIs, data flow, or destructive-action behavior change in this pass.
