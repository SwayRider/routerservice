# Code Review — 2026-08-19

Follow-up security audit of the full current codebase (not diff-based), cross-checked against [`review/CODE_REVIEW_2026-08.md`](CODE_REVIEW_2026-08.md). See [`Docs/REVIEW.md`](../../Docs/REVIEW.md) for how findings in this file are tracked.

**Verification of prior review:** all 18 previously-tracked items are marked FIXED; the security-relevant ones were verified against current code with no regressions — the auth interceptor is present and correctly wired on both REST and gRPC paths (`cmd/routerservice/main.go:98`, `internal/server/server.go:14-18`, `Route` declared `UserOrServiceEndpoint`), the two process-crash bugs are fixed with proper guards (`internal/logic/routing_request.go:245,410-417`), Valhalla call timeouts are restored (`internal/server/route.go:92`), `parseHosts`/`parsePorts` validate format instead of panicking (`internal/valhalla/config.go:55-59`), and `validateLocations` rejects nil/NaN/Inf/out-of-range coordinates on `req.Locations` (`internal/server/route.go:127-141`).

**SSRF: confirmed non-issue.** Valhalla region names used in `ResolveHost` (`internal/valhalla/resolver.go:9-19`) originate only from regionservice's own responses, never directly from client-supplied strings — a caller cannot redirect the hostname template to an arbitrary host.

The prior review focused on crashes/correctness; this pass found new issues in information disclosure and request-size amplification that it didn't examine.

### 1. Valhalla error responses leak raw upstream content to external callers

`restclients/valhalla/client.go:79-82,119-122,158-161`, `internal/server/status.go:21-30`. Non-2xx Valhalla responses are wrapped as `fmt.Errorf("unexpected status code: %d: %s", resp.StatusCode, body)`, embedding the full raw HTTP response body from Valhalla. This error is not a `net.Error`, so it skips the generic-message path in `route.go:96-99` and falls through `grpcStatus`'s default branch (`status.go:29`), which returns `codes.Internal` with `err.Error()` verbatim as the gRPC status message. **Scenario:** Valhalla returns a 500 with a stack trace, debug page, or internal hostname/path (e.g. a misconfigured or crashing Valhalla instance) — that content lands directly in the gRPC status message returned to the caller, and per the gateway's proxying behavior is plausibly surfaced to the external client through swayrider-api. Severity: Medium.

### 2. Downstream gRPC error messages from regionservice are passed through unfiltered

`internal/server/status.go:16-19`. `grpcStatus` forwards any downstream gRPC status message from regionservice verbatim (`status.Error(st.Code(), st.Message())`) with no sanitization. If regionservice's own error messages ever include internal detail (DB errors, internal addresses), routerservice propagates them unchanged to the external caller. This compounds finding #1 — routerservice has no defense-in-depth error-sanitization layer for either of its two upstreams. Severity: Medium.

### 3. No limit on request size enables resource-exhaustion / amplification

`internal/server/route.go:30` (only checks `< 2`), `internal/server/route.go:353-388`, `internal/logic/resolve_region.go:34-45`. `Route` never bounds `len(req.Locations)`, `len(req.ExcludeLocations)`, or the point count of `req.ExcludePolygons` entries. Each location triggers one sequential, synchronous `regionservice.SearchPoint` call, and `exclude_locations`/`exclude_polygons` are forwarded to Valhalla with no cap. **Scenario:** an authenticated client (any valid user JWT — no extra privilege needed) submits a request with thousands of waypoints or a huge exclude-polygon list; this ties up a goroutine issuing thousands of sequential regionservice round-trips and amplifies the payload forwarded to backend Valhalla instances, well beyond what one request should cost. Self-inflicted DoS reachable by any authorized caller, not just an operator. Severity: Medium.

### 4. `ExcludeLocations`/`ExcludePolygons` coordinates are unvalidated

`internal/server/route.go:353-388`. Unlike `req.Locations` (validated via `validateLocations`), points in `ExcludeLocations` and `ExcludePolygons` are forwarded to Valhalla with no NaN/Inf/range check. Low impact (likely just rejected or ignored by Valhalla) but inconsistent with the validation added for the primary locations list. Severity: Low.

### 5. Minor internal-topology disclosure via "region not found" errors

`restclients/valhalla/client.go:54,101,133` — `fmt.Errorf("region %s not found", region)` leaks that a resolved region has no configured Valhalla client, but not host/port. Negligible alone, only relevant combined with findings #1–2. Severity: Info.
