# Codex Tasks

Important working rules:

1. Maintain PROJECT_LOG.md and NEXT_STEPS.md exactly as previously instructed.
2. Read PROJECT_LOG.md and NEXT_STEPS.md first before making changes.

## Task 1

Task:
Add practical client setup documentation for real device testing.

Requirements:
- Add a README section for connecting OwnTracks
- Add a README section for connecting Overland
- Include:
  - endpoint URLs
  - authentication method
  - required headers or tokens
  - example payload expectations
  - known caveats
  - troubleshooting tips for 400, 401, and 500 responses
- Keep instructions concise and actionable
- Do not add unsupported claims about exact client UI fields unless already known from the current code expectations

At the end:
- update PROJECT_LOG.md
- update NEXT_STEPS.md


## Task 2

Task:
Add a recent points inspection endpoint.

Requirements:
- Add an endpoint such as GET /api/v1/points/recent
- Support simple query params such as:
  - device_id optional
  - limit optional
- Return recent stored points from SQLite
- Keep output compact and useful for debugging
- Add tests
- Update README with curl examples

Do not build map or timeline features yet.
At the end:
- update PROJECT_LOG.md
- update NEXT_STEPS.md


## Task 3

Task:
Add GeoJSON export for stored points.

Requirements:
- Add an endpoint such as GET /api/v1/exports/geojson
- Support filtering by:
  - from timestamp optional
  - to timestamp optional
  - device_id optional
- Return valid GeoJSON for stored points
- Keep implementation lightweight
- Add tests for valid GeoJSON structure
- Update README with usage examples

Do not add a frontend map yet.
At the end:
- update PROJECT_LOG.md
- update NEXT_STEPS.md


## Task 4

Task:
Add GPX export for stored points.

Requirements:
- Add an endpoint such as GET /api/v1/exports/gpx
- Support filtering by:
  - from timestamp optional
  - to timestamp optional
  - device_id optional
- Return valid GPX output for location history
- Keep output simple and compatible
- Add tests for basic structure and content
- Update README with usage examples

At the end:
- update PROJECT_LOG.md
- update NEXT_STEPS.md


## Task 5

Task:
Add a very lightweight admin/status web page.

Requirements:
- Keep it simple and low-resource
- Prefer server-rendered HTML or a very small frontend
- Show:
  - service health
  - devices
  - buffer stats
  - spool stats
  - checkpoint value
  - last flush result
  - recent points preview
- Do not build a full SPA
- Do not build the full Dawarich-like UI yet
- Update README with where to access the page

At the end:
- update PROJECT_LOG.md
- update NEXT_STEPS.md


## Task 6

Task:
Prepare the service for Raspberry Pi deployment.

Requirements:
- Add a sample systemd service file
- Add a sample environment file
- Add install or setup script if appropriate
- Document:
  - where DB lives
  - where spool lives
  - how to start/stop service
  - how to inspect logs
  - how to back up DB and spool
- Keep packaging lightweight and easy to understand

At the end:
- update PROJECT_LOG.md
- update NEXT_STEPS.md

## Task 7

Task:
Dockerize the tracker service in a lightweight way.

Requirements:
- Add a multi-stage Dockerfile for the Go service
- Add a .dockerignore
- Add an optional minimal compose.yaml
- Persist SQLite DB and spool directory via mounted /data volume
- Use environment variables for runtime config
- Keep the setup single-container only
- Do not add Redis, PostgreSQL, or extra services
- Ensure the setup is suitable for ARM systems and document Raspberry Pi considerations
- Update README with:
  - docker build
  - docker run
  - compose up/down
  - volume mapping
  - environment variables
  - caveats for Pi Zero 2 W

Please keep it simple and production-practical.
At the end:
- update PROJECT_LOG.md
- update NEXT_STEPS.md