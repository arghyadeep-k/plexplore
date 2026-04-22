# Codex Tasks

Important working rules:

1. Maintain PROJECT_LOG.md and NEXT_STEPS.md exactly as previously instructed.
2. Read PROJECT_LOG.md and NEXT_STEPS.md first before making changes.

## Task 1

Task:
Add an API endpoint to inspect generated visits.

Requirements:
- Add an endpoint such as GET /api/v1/visits
- Support query params:
  - device_id optional
  - from optional
  - to optional
  - limit optional
- Return visits in a compact JSON format
- Include fields:
  - id
  - device_id
  - start_at
  - end_at
  - centroid_lat
  - centroid_lon
  - point_count
- Add tests for:
  - listing visits
  - filtering by device
  - filtering by time range
  - invalid params
- Update README with curl examples
- Update PROJECT_LOG.md
- Update NEXT_STEPS.md


## Task 2

Task:
Extend the map UI to show visits.

Requirements:
- Use the visits endpoint to load visits for the selected range/device
- Display visits on the map in a lightweight way
- Keep it simple:
  - marker or circle for each visit centroid
  - popup or label showing start/end time and point count
- Do not add heavy client-side state management
- Keep the existing track rendering intact
- Add graceful fallback if no visits exist
- Update README with a short note about visit display
- Update PROJECT_LOG.md
- Update NEXT_STEPS.md

## Task 3

Task:
Add a lightweight visits summary section to the existing UI.

Requirements:
- Show a simple visits list or summary below or beside the map
- For each visit show:
  - start time
  - end time
  - duration if practical
  - device id
- Keep presentation minimal and readable
- Do not build a heavy timeline component
- Add a basic route/render test if practical
- Update README
- Update PROJECT_LOG.md
- Update NEXT_STEPS.md

