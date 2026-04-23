Multi-User Authentication Milestone Prompts
Admin-created users only. No public self-signup.

Overview
This prompt plan converts the current single-user/minimal-auth instance into a multi-user instance where:
- an admin can create users
- users can sign in and sign out
- each user only sees their own devices and location data
- device API keys continue to work for ingest
- one shared instance can safely host multiple users

General implementation rules to include at the start of each task 
- Maintain PROJECT_LOG.md and NEXT_STEPS.md exactly as previously instructed.
- Read PROJECT_LOG.md and NEXT_STEPS.md first before making changes.
- Keep changes small and atomic
- Do not refactor unrelated code
- Keep the stack lightweight and Raspberry Pi friendly
- Prefer standard library where practical
- Keep device API key ingest auth intact while adding browser/user auth

Task 1: Extend user model and add migrations for account auth

Prompt:
Add the database and store-layer foundations for account-based user authentication with admin-created users only.

Requirements:
- Extend the users table as needed to support account auth
- Add fields at minimum:
  - id
  - email unique
  - password_hash
  - is_admin
  - created_at
  - updated_at
- If any of these fields already exist, migrate safely instead of duplicating
- Add a new migration rather than rewriting old migrations in place
- Add store-layer methods for:
  - CreateUser(...)
  - GetUserByEmail(...)
  - GetUserByID(...)
  - ListUsers(...)
- Keep implementation SQLite-friendly and lightweight
- Add tests for migrations and store methods
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 1:
- Run: go test ./internal/store ./...
- Run migrations on a fresh DB: make migrate
- Validate schema manually with sqlite3:
  - sqlite3 ./data/plexplore.db ".schema users"
- Confirm users table contains email, password_hash, is_admin, created_at, updated_at


Task 2: Add password hashing helpers

Prompt:
Implement password hashing and verification helpers for account login.

Requirements:
- Add lightweight password hashing helpers using a standard secure approach such as bcrypt or argon2id
- Keep dependencies minimal
- Add functions such as:
  - HashPassword(plain string) (string, error)
  - VerifyPassword(hash, plain string) error or bool
- Enforce basic input validation for empty passwords
- Keep code isolated in an auth or security package
- Add focused unit tests
- Update PROJECT_LOG.md and NEXT_STEPS.md

Validation after Task 2:
- Run: go test ./...
- Confirm unit tests cover:
  - valid password hash/verify
  - wrong password fails
  - empty password rejected


Task 3: Add admin bootstrap path

Prompt:
Add an admin bootstrap path so the first admin account can be created without public self-signup.

Requirements:
- Do not add public signup
- Add one safe bootstrap mechanism, choose one simple approach and document it clearly:
  - CLI command such as cmd/createadmin
  - or environment-driven one-time admin creation on startup if no users exist
- Preferred approach: add a small CLI tool such as cmd/createuser or cmd/createadmin
- CLI should support at minimum:
  - email
  - password
  - is_admin true for bootstrap admin
- Prevent accidental duplicate admin creation for the same email
- Add tests if practical, otherwise document manual validation steps clearly
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 3:
- Run migrations: make migrate
- Run admin creation command, for example:
  - go run ./cmd/createadmin --email admin@example.com --password 'testpass'
- Verify in DB:
  - sqlite3 ./data/plexplore.db "SELECT id,email,is_admin FROM users;"
- Confirm admin row exists and password is stored hashed, not plaintext


Task 4: Add user session model and session middleware

Prompt:
Add browser/session authentication for signed-in users.

Requirements:
- Add session support for web/API user authentication
- Keep it lightweight and Raspberry Pi friendly
- Preferred approach:
  - server-side sessions stored in SQLite
  - secure random session token in an HttpOnly cookie
- Add session table and migration if needed
- Add session store methods such as:
  - CreateSession(userID)
  - GetSession(token)
  - DeleteSession(token)
- Add middleware/helper to load the current signed-in user from the session cookie
- Keep device API key auth separate and unchanged for ingest endpoints
- Add tests for session creation, lookup, expiration behavior if implemented, and middleware loading
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 4:
- Run: go test ./...
- Confirm new session table exists if implemented in DB:
  - sqlite3 ./data/plexplore.db ".tables"
- Confirm middleware tests pass for valid and invalid session cookies


Task 5: Add login and logout endpoints plus minimal sign-in page

Prompt:
Add user sign-in and sign-out flows for admin-created users only.

Requirements:
- Do not add self-signup
- Add endpoints:
  - GET /login
  - POST /login
  - POST /logout
- Add a minimal sign-in page served directly by backend templates/HTML
- On login:
  - look up user by email
  - verify password hash
  - create session
  - set secure session cookie
- On logout:
  - clear/delete session
  - expire session cookie
- Keep UI lightweight, no frontend framework
- Add tests for:
  - successful login
  - invalid credentials
  - logout clears session
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 5:
- Run: go test ./internal/api ./...
- Manual test:
  - start server
  - open /login
  - sign in with admin account
  - verify a session cookie is set
  - sign out and verify access is removed where appropriate


Task 6: Add authenticated user helper and route protection middleware

Prompt:
Add middleware/helpers for authenticated user access control in browser/API routes.

Requirements:
- Add helpers such as:
  - RequireUserSessionAuth(...)
  - CurrentUserFromContext(...)
- Protect relevant user-facing routes so they require login
- Do not apply this to device ingest endpoints that use API key auth
- Add redirect behavior for HTML pages and 401/403 behavior for JSON endpoints as appropriate
- Keep logic simple and documented
- Add tests for protected route access with and without session
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 6:
- Run: go test ./...
- Manual test:
  - access protected UI/API route without session and confirm redirect or unauthorized response
  - access same route with valid login and confirm success


Task 7: Add admin-only user management endpoints

Prompt:
Add admin-only user management for creating and listing users. No public signup.

Requirements:
- Add admin-protected endpoints:
  - GET /api/v1/users
  - POST /api/v1/users
- Optional:
  - GET /api/v1/users/{id}
- Only authenticated admins may access these routes
- POST /api/v1/users should allow admin to create non-admin or admin accounts
- Validate unique email and required password
- Do not expose password hashes in responses
- Add tests for:
  - admin can create user
  - non-admin cannot create user
  - unauthenticated request denied
  - list users excludes password hashes
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 7:
- Run: go test ./...
- Manual test:
  - login as admin
  - create a second user
  - verify DB rows:
    - sqlite3 ./data/plexplore.db "SELECT id,email,is_admin FROM users;"
  - verify user list response does not expose password_hash


Task 8: Scope device listing and device read endpoints by current signed-in user

Prompt:
Convert device read/list APIs from minimal single-user behavior to authenticated per-user behavior.

Requirements:
- Update device list/read routes so they require a signed-in user session
- Only return devices owned by the current signed-in user unless the current user is admin and explicit admin-wide behavior is intentionally supported
- Keep behavior simple; safest default is user-only scope even for admins unless an explicit admin view is added later
- Ensure device create behavior associates created devices with the correct user
- Keep API key rotation protected by ownership or admin rights
- Add tests for:
  - user sees only own devices
  - user cannot fetch another user’s device
  - rotate-key denied for non-owner
  - admin behavior if implemented
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 8:
- Run: go test ./...
- Manual test with two users:
  - login as user A, create/list devices
  - login as user B, confirm user A devices are not visible
  - attempt direct GET on another user device id and confirm denial


Task 9: Ensure device creation is user-owned and admin-managed where appropriate

Prompt:
Finalize device ownership rules for a multi-user instance.

Requirements:
- Ensure each device belongs to exactly one user
- Decide and implement one clear model:
  - users create their own devices after login
  - or admins create devices for users
- Recommended simple model:
  - signed-in user can create their own devices
  - admin can also create devices for a specified user if needed
- Update device creation endpoint accordingly
- Validate ownership rules consistently in store and API layers
- Add tests for ownership assignment and admin override if implemented
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 9:
- Run: go test ./...
- Manual test:
  - create devices as different users
  - confirm device rows have correct user_id in DB:
    - sqlite3 ./data/plexplore.db "SELECT id,user_id,name FROM devices;"


Task 10: Scope recent points endpoint by signed-in user

Prompt:
Add user-based authorization to the recent points endpoint.

Requirements:
- Protect GET /api/v1/points/recent with user session auth
- Return only points belonging to the current signed-in user
- Preserve device filter and limit behavior, but only within the user’s own data
- Add tests for:
  - user sees only own recent points
  - user cannot retrieve another user’s points via device filter tricks
  - unauthenticated access denied
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 10:
- Run: go test ./...
- Manual test:
  - ingest points for user A device and user B device
  - query /api/v1/points/recent as each user and verify isolation


Task 11: Scope point history/map endpoints by signed-in user

Prompt:
Add user-based authorization to point history query endpoints used for map and browsing.

Requirements:
- Protect GET /api/v1/points and any similar map/history endpoints with user session auth
- Scope all results to the current signed-in user
- Preserve date range and device filters only within the user’s own device set
- Add tests for cross-user isolation and invalid access attempts
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 11:
- Run: go test ./...
- Manual test:
  - login as each user and query date ranges
  - confirm only own track history is returned


Task 12: Scope export endpoints by signed-in user

Prompt:
Add user-based authorization to export endpoints.

Requirements:
- Protect:
  - GET /api/v1/exports/geojson
  - GET /api/v1/exports/gpx
- Ensure exports only include the current signed-in user’s data
- Preserve from/to/device filters, but only within the user’s own devices
- Add tests for:
  - user export contains only own points
  - cross-user data is not leaked
  - unauthenticated export denied
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 12:
- Run: go test ./...
- Manual test:
  - export as user A and user B
  - inspect outputs and confirm each contains only that user’s devices/points


Task 13: Scope visits endpoints and future visit generation by signed-in user

Prompt:
Add user-based authorization to visits APIs and related visit queries.

Requirements:
- Protect visit query endpoints with user session auth
- Ensure visit rows are user-owned either directly or through device ownership
- Scope filters to the current user only
- Add tests for isolation across users
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 13:
- Run: go test ./...
- Manual test:
  - generate/query visits for multiple users
  - confirm no cross-user leak


Task 14: Protect UI pages and make them session-aware

Prompt:
Convert the lightweight web UI to authenticated multi-user behavior.

Requirements:
- Protect relevant UI pages so they require login
- Add a small signed-in header or indicator showing current user email
- Add a logout control in the UI
- Ensure UI data fetches work with session-based auth and only show current user data
- Keep the frontend lightweight and server-rendered/plain JS
- Add or update tests for protected page behavior and basic signed-in rendering
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 14:
- Run: go test ./internal/api ./...
- Manual test:
  - open UI while logged out and confirm redirect to login or access denied
  - login as user A and confirm only user A data appears
  - login as user B and confirm isolation


Task 15: Keep device API key ingest auth working in multi-user mode

Prompt:
Audit and finalize ingest behavior so device API key auth still works correctly in a multi-user instance.

Requirements:
- Device ingest endpoints must continue using device API key auth
- Each device API key must resolve to exactly one device and one owning user
- Ingested points must persist under the owning user/device relationship correctly
- Browser session auth should not be required for device ingest
- Add tests for:
  - valid device key ingests under correct user/device ownership
  - invalid key rejected
  - cross-user contamination does not occur
- Update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 15:
- Run: go test ./...
- Manual test:
  - create two users, one device each
  - ingest using each device key
  - verify raw_points and related queries stay associated with the correct user/device


Task 16: Add authorization audit tests across the full app

Prompt:
Add full authorization and isolation test coverage for the new multi-user model.

Requirements:
- Add integration tests covering at minimum:
  - admin creates users
  - users log in separately
  - each user only sees own devices
  - each user only sees own points
  - each user only exports own data
  - device API key ingest persists to correct owner
  - non-owner cannot rotate another user’s device key
- Keep tests deterministic and use temporary DB/spool directories
- Prefer real route wiring over mocks where practical
- Update PROJECT_LOG.md and NEXT_STEPS.md

Validation after Task 16:
- Run: go test ./...
- Specifically run integration/auth-related suites if separated
- Confirm no cross-user leakage paths remain in tests


Task 17: Add admin user management page (optional but recommended)

Prompt:
Add a lightweight admin-only user management page to the existing UI.

Requirements:
- Keep UI minimal and consistent with the project’s lightweight approach
- Add an admin-only page or section where admins can:
  - list users
  - create users
- Do not add self-signup
- Do not expose password hashes
- Keep forms simple and readable
- Add tests where practical and update README, PROJECT_LOG.md, NEXT_STEPS.md

Validation after Task 17:
- Run: go test ./...
- Manual test:
  - login as admin and create a user through the UI
  - confirm non-admin cannot access the admin user page


Task 18: Final hardening and docs pass

Prompt:
Do a final hardening pass for the new multi-user auth model.

Requirements:
- Review cookies and session settings for secure defaults appropriate to the app
- Add CSRF protection for login/logout and admin user creation forms if form-based POST endpoints are used
- Review error handling to avoid leaking sensitive details
- Review API responses to ensure password hashes and full secrets are never exposed
- Update README with a clear multi-user admin-created-users-only setup guide
- Update PROJECT_LOG.md and NEXT_STEPS.md with the new steady state

Validation after Task 18:
- Run: go test ./...
- Manual checklist:
  - admin bootstrap works
  - admin can create users
  - users can log in and log out
  - each user sees only own data
  - device ingest still works by API key
  - exports are user-scoped
  - UI is protected and user-aware
  - no password hash or secret leakage in normal responses


Recommended execution order
1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 5
6. Task 6
7. Task 7
8. Task 8
9. Task 9
10. Task 10
11. Task 11
12. Task 12
13. Task 13
14. Task 14
15. Task 15
16. Task 16
17. Task 17
18. Task 18

Practical note
The biggest risk is not login itself. The biggest risk is missing one read/export/query path that still returns unscoped data. Treat Tasks 10 through 16 as the security-critical part of the milestone.
