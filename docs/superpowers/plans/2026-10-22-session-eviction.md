# Session Eviction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement session eviction (kicking the oldest session when `max_sessions` limit is reached) in Abdal 4iProto Server, with a global default in `server_config.json` and optional per-user overrides in `users.json`, then build and deploy the updated server on `vds_tl`.

**Architecture:** Extend `ServerConfig` and `User` structs with `EvictOnMaxSessions` (`bool` on server level, `*bool` on user level to support inheritance). In `session.Manager.CreateSession`, when `len(activeSessions) >= maxSessions`, if eviction is enabled for the user, sort active sessions by age, close the active SSH connection for the oldest session, revoke it in BoltDB, and permit the new session. Add automated unit tests, synchronize sources to `vds_tl`, compile with Go on the remote host, and deploy the new binary with service restart.

**Tech Stack:** Go 1.26+, BoltDB (bbolt), crypto/ssh, systemd, rsync/SSH.

**Spec:** User request: "внедить вытестнение в исходник, пересобрать на удаленном сервере. должен параметр быть в конфигурации сервера, для задания значения по умолчанию и отдельный параметр для каждого пользователя".

## Global Constraints

- Do NOT execute `git commit` or `git push` automatically (strictly prohibited by user global rule).
- Do NOT modify `~/.ssh/config` (strictly prohibited by user global rule).
- Default SSH key for remote operations: `~/.ssh/home_openclaw`.
- Maintain existing codebase comments and docstrings.
- Remote server target: `root@141.98.166.10:52242` (`vds_tl`), service `abdal-4iproto-server`.

## Review Focus

1. **Deadlock prevention:** `CreateSession` holds `sm.mu.Lock()`. Eviction must NOT call methods that attempt to re-acquire `sm.mu.Lock()` (e.g. `CloseSession`), but must close `conn` directly and invoke `sm.revokeSessionLocked`.
2. **Per-user override priority:** If user defines `evict_on_max_sessions: false` and server defines `evict_on_max_sessions: true`, eviction MUST NOT happen for that user. If user defines `evict_on_max_sessions: true` and server defines `false`, eviction MUST happen. If user leaves it omitted/nil, it MUST inherit the server config default.
3. **Correct oldest session selection:** Active sessions must be sorted deterministically by creation time (`CreatedAt`) ascending so the oldest session is evicted first.
4. **Graceful connection termination:** Evicting a session must call `conn.Close()` on the registered SSH connection so the dead peer/socket is disconnected immediately.
5. **No downtime regression:** Build on remote server in `/root/agy/git_project/4iproto-server`, verify tests pass, backup existing binary to `.bak` before replacing, then restart systemd service and verify running state.

---

### Task 1: Add configuration fields and helper function

**Files:**
- Modify: `core/config/config-constants.go`
- Modify: `core/config/types.go`
- Modify: `server_config.json`

**Interfaces:**
- Produces:
  - `config.DefaultEvictOnMaxSessions` (`bool`)
  - `config.ServerConfig.EvictOnMaxSessions` (`bool`, json: `"evict_on_max_sessions"`)
  - `config.User.EvictOnMaxSessions` (`*bool`, json: `"evict_on_max_sessions,omitempty"`)

- [ ] **Step 1: Add `DefaultEvictOnMaxSessions` to `core/config/config-constants.go`**
  Add `DefaultEvictOnMaxSessions = false` in session defaults section.

- [ ] **Step 2: Add `EvictOnMaxSessions` to `ServerConfig` and `User` in `core/config/types.go`**
  Add `EvictOnMaxSessions bool` in `ServerConfig` with tag `` `json:"evict_on_max_sessions"` ``.
  Add `EvictOnMaxSessions *bool` in `User` with tag `` `json:"evict_on_max_sessions,omitempty"` ``.

- [ ] **Step 3: Update local `server_config.json` template**
  Add `"evict_on_max_sessions": true` to `server_config.json`.

---

### Task 2: Implement eviction logic and helper in `core/session/manager.go`

**Files:**
- Modify: `core/session/manager.go`

**Interfaces:**
- Consumes: `config.Server().EvictOnMaxSessions`, `config.GetUser(username)`
- Produces: `userEvictOnMaxSessions(username string) bool`

- [ ] **Step 1: Add `userEvictOnMaxSessions(username string) bool` helper in `core/session/manager.go`**
  Checks if `user.EvictOnMaxSessions != nil`, returns `*user.EvictOnMaxSessions`. Otherwise returns `config.Server().EvictOnMaxSessions`.

- [ ] **Step 2: Implement eviction loop in `CreateSession` in `core/session/manager.go`**
  Inside `CreateSession` when `len(activeSessions) >= maxSessions`:
  Check `if userEvictOnMaxSessions(username)`:
  Sort `activeSessions` by `CreatedAt` ascending.
  Iterate while `len(activeSessions) >= maxSessions && len(activeSessions) > 0`:
  - Take `oldest := activeSessions[0]`
  - Log eviction notice: `⚠️ Max sessions reached for %s (%d/%d), evicting oldest session %s (IP: %s)`
  - Close connection if present: `if conn, exists := sm.activeConnections[oldest.SessionID]; exists { conn.Close(); delete(sm.activeConnections, oldest.SessionID) }`
  - Revoke session: `sm.revokeSessionLocked(oldest.SessionID)`
  - Slice `activeSessions = activeSessions[1:]`
  If not evicting, log error and return `fmt.Errorf("maximum concurrent sessions limit reached...")` as before.

---

### Task 3: Write unit tests for session eviction

**Files:**
- Create: `core/session/eviction_test.go`

**Interfaces:**
- Tests `CreateSession` with:
  1. Eviction enabled (`EvictOnMaxSessions = true`), maxSessions = 1: creating 2nd session evicts 1st and succeeds.
  2. Eviction disabled (`EvictOnMaxSessions = false`), maxSessions = 1: creating 2nd session fails with error.
  3. User override: server default false, user override true -> evicts. Server default true, user override false -> rejects.

- [ ] **Step 1: Create `core/session/eviction_test.go`**
  Write isolated test using temporary bbolt database verifying all eviction combinations and connection closure.

---

### Task 4: Synchronize to remote server `vds_tl`, compile and run tests

**Files:**
- Synchronize modified files to `root@141.98.166.10:/root/agy/git_project/4iproto-server` using rsync.

- [ ] **Step 1: Sync modified files to `/root/agy/git_project/4iproto-server`**
  Execute `rsync` over SSH key `~/.ssh/home_openclaw -p 52242`.

- [ ] **Step 2: Run unit tests on remote server**
  Execute `go test -v ./core/session/...` on `vds_tl`.
  Verify all tests pass cleanly.

- [ ] **Step 3: Compile new binary on remote server**
  Execute `go build -o abdal_4iproto_server_linux_amd64 main.go` in `/root/agy/git_project/4iproto-server`.

---

### Task 5: Deploy to production and verify

**Files:**
- `/usr/local/abdal-4iproto-server/server_config.json`
- `/usr/local/abdal-4iproto-server/users.json`
- `/usr/local/abdal-4iproto-server/abdal_4iproto_server_linux_amd64`

- [ ] **Step 1: Update production configuration files on `vds_tl`**
  Update `/usr/local/abdal-4iproto-server/server_config.json` with `"evict_on_max_sessions": true`.
  Check and optionally configure `users.json` for specific user overrides if desired.

- [ ] **Step 2: Deploy new binary**
  Backup current binary: `cp /usr/local/abdal-4iproto-server/abdal_4iproto_server_linux_amd64 /usr/local/abdal-4iproto-server/abdal_4iproto_server_linux_amd64.bak`
  Install new binary: `cp /root/agy/git_project/4iproto-server/abdal_4iproto_server_linux_amd64 /usr/local/abdal-4iproto-server/`

- [ ] **Step 3: Restart systemd service and verify**
  Restart service: `systemctl restart abdal-4iproto-server.service`
  Check status and journalctl logs: `systemctl status abdal-4iproto-server.service`
  Confirm that service starts and handles connections properly.
