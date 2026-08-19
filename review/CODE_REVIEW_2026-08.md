# Code Review — `routerservice`

**Date:** 2026-08
**Scope:** Full review of `routerservice/` — the multi-region routing service for the SwayRider platform (Valhalla fan-out, region assignment, border-crossing stitching, leg merging).
**Reviewed:** `cmd/routerservice/main.go`, `internal/server/*` (server, route, merge, ping, status), `internal/logic/*` (routing_request, region_assignment, resolve_region, errors), `internal/valhalla/*`, `internal/pelias/*`, `restclients/valhalla/*` (client + all `types`), all test files, `protos/router/v1/router.proto`, `protos/health/v1/health.proto`, `Dockerfile`, `Makefile`, `env.example`, `local.env`, `.github/workflows/ci.yml`, the consuming `grpcclients/regionclient`, the `regionservice` implementation of `FindCrossingLocations`, and the shared `swlib` app/security/logger machinery.
**Verification performed:** `go build ./...`, `go vet ./...`, and `go test ./... -count=1` all pass (with the existing tests). Per-package coverage measured with `go test -cover`. Three suspected crash/correctness bugs were confirmed empirically with throwaway reproduction tests (removed afterwards; the repo is unmodified). No code changes were made.

---

## Summary

The service has a sound architecture: a clean `server → logic → valhalla/pelias config → restclients` split, a tasteful option-pattern abstraction for translating `RouteRequest` fields into Valhalla costing options, a centralized `grpcStatus` error mapper, and — unlike `regionservice` — the auth interceptor is **correctly enabled** (`app.AuthInterceptor | app.ClientInfoInterceptor` with a JWT key fetch loop), so the declared `routing:execute` / user-JWT endpoint profile is actually enforced. The pure-function unit tests (road-type orderings, corridor math, merge sub-functions) are well written and give `internal/valhalla` 82.5% and `internal/pelias` 74.3% coverage.

However, the **entire end-to-end routing path is untested (0%)**, and it contains **two process-crashing bugs** in the code a cross-border route hits on essentially every request:

1. **`CreateRoutingRequests` panics on transfer-region assignments.** Multi-region routes that pass through an intermediate region (e.g. NL → BE → FR, with BE a transfer region) produce a `RegionAssignment` with `FromIndex = ToIndex = -1` (`IsEmpty = true`). `CreateRoutingRequests` iterates `for i := assignment.FromIndex; i <= assignment.ToIndex; i++` with no empty check and indexes `routeLocations[-1]` → **`index out of range [-1]` panic**. This is the *normal* case for any European cross-border route with an intermediate region — verified empirically. The process dies (no recover interceptor).
2. **`AddBorderCrossings` panics on an empty crossing list.** It unconditionally does `selectedBc := &crossings[0]`. `regionservice`'s `FindCrossingLocations` can legally return an empty list with no error (the closest crossing may not match any road type in the configured order, or be dropped by `RoadTypeDelta`/`DropDistance`), so a plausible route request crashes the process — verified by tracing `regionservice/internal/server/find_crossing_locations.go`.

Both are reachable by any authenticated user (or unauthenticated if auth is ever regressed) and kill the whole service. The root cause of both is the same: the orchestration layer is completely untested.

Beyond the crashes, the two other notable issues are a **missing timeout on all Valhalla HTTP calls** (a hung Valhalla hangs the request forever — the intended `context.WithTimeout` is commented out) and an **off-by-one in the merged-shape maneuver indices** at border crossings (verified empirically), which makes navigation clients highlight the wrong polyline segment after every border.

---

## Critical

### 1. `CreateRoutingRequests` panics on transfer-region assignments — crashes on normal cross-border routes ✅ FIXED (commit `35a7d89`)
`internal/logic/routing_request.go:221`, `internal/logic/region_assignment.go:139-150`

`injectTransferRegions` injects intermediate regions along the route path as empty assignments:

```go
finalizedList = append(finalizedList, &RegionAssignment{
    Region:    path[j],
    FromIndex: -1,
    ToIndex:   -1,
    IsEmpty:   true,
})
```

`CreateRoutingRequests` then iterates them with no `IsEmpty` guard:

```go
for _, assignment := range assignmentList {
    req := vhtypes.NewRouteRequest(model)
    for i := assignment.FromIndex; i <= assignment.ToIndex; i++ {   // -1 <= -1 → one iteration
        routeLoc := routeLocations[i]                                // routeLocations[-1] → PANIC
        ...
    }
```

A route whose region path has more than two regions (e.g. Amsterdam → Paris with Belgium as a transfer region) sets `FromIndex = ToIndex = -1`, and the loop body executes once with `i = -1`, indexing `routeLocations[-1]`. **Confirmed empirically:**

```
go test ... TestCreateRoutingRequests_EmptyTransferAssignment
panic: runtime error: index out of range [-1]
```

The `IsEmpty` flag is set by the producer but never consumed by the consumer. There is no `recover` interceptor, so the entire process dies on a completely ordinary request. This is the single most serious bug in the service: the feature it exists to provide (multi-region routing) is the trigger.

Fix direction: in `CreateRoutingRequests`, skip `assignment.IsEmpty` entries (or give them explicit handling — e.g. route through the transfer region with the previous/next waypoints as through points), and add a regression test. Also consider validating `FromIndex`/`ToIndex` bounds defensively.

### 2. `AddBorderCrossings` panics when no border crossings are found ✅ FIXED
`internal/logic/routing_request.go:350`, `grpcclients/regionclient/client.go` (`FindCrossingLocations`), `regionservice/internal/server/find_crossing_locations.go`

```go
crossings, err = regionClnt.FindCrossingLocations(...)
if err != nil { ... return }

// By default select the first crossing result,
// unless there is an exact match with the preferred road type
selectedBc := &crossings[0]    // PANIC if crossings is empty
```

`FindCrossingLocations` on the regionservice side returns **empty-without-error** whenever the closest crossing exists but none of the crossings match the road-type order / survive the `RoadTypeDelta`/`DropDistance` filtering. Concretely, `getRoadType` returns `nil` for tertiary/lower roads, so `closeRoadTypeOrder(nil, nil, ...)` only lists `secondary/primary/trunk/motorway` (or `secondary/primary` for motor-scooters); if the actual border has only tertiary/residential crossings, the result is an empty slice with `err == nil`. `crossings[0]` then panics — another uncaught process crash. (Note this also means the "preferred road type" loop is dead in that case.)

Fix direction: guard `len(crossings) == 0` (return a clean error, e.g. `ErrNoBorderCrossings`), and add a regression test with an empty crossing list.

---

## High

### 3. No timeout on any Valhalla HTTP call — a hung Valhalla hangs the request forever ✅ FIXED
`internal/server/route.go:87-90`, `internal/logic/routing_request.go:407-411`, `restclients/valhalla/client.go:73,121`

Every Valhalla call discards the incoming RPC context and uses a fresh background context with **no deadline**:

```go
//ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)   // commented out
ctx, cancel := context.WithCancel(context.Background())
resp, err := vhClient.Route(ctx, routeReq.Region, routeReq.RequestData)
defer cancel()
```

Combined with `http.DefaultClient` (which has **no** default timeout), a Valhalla instance that hangs or stalls leaves the gRPC handler blocked indefinitely:
- The caller's cancellation/deadline is never propagated to the Valhalla HTTP request (the incoming `ctx` is shadowed).
- There is no server-side bound, so a slow/hung Valhalla can pin gRPC goroutines indefinitely — a resource-exhaustion risk under load.
- The commented-out `1*time.Second` also suggests the intended behavior was a timeout, and it was disabled.

The same pattern exists in `getRoadType` (`routing_request.go:407-411`), which does a synchronous `/locate` HTTP call per border segment.

Fix direction: restore a real deadline — derive `ctx` from the incoming RPC context with `context.WithTimeout(ctx, ...)` (or a per-call timeout), and/or configure an `http.Client` with a `Timeout`. Also move the `defer cancel()` out of the loop body (it currently accumulates until the function returns).

---

## Medium

### 4. Off-by-one in merged-shape maneuver indices at border crossings ✅ FIXED
`internal/server/merge.go:156`

```go
offsets[i] = len(flat) / 2
flat = append(flat, points[overlap:]...)
```

`offsets[i]` is recorded **before** the overlapping shared point is stripped, so for an overlapping pair of legs the offset is one too large. When two region sub-routes share a border point `B` (leg0 `… → B`, leg1 `B → …`), the merged shape is `[A, B, C]` and leg1's point 0 (`B`) lives at merged index **1**, but `offsets[1]` is computed as `2`. `flattenManeuvers` then adds 2 to every leg1 maneuver index, so all maneuvers of the second (and later) leg point one position too far into the merged polyline — the client will highlight the wrong segment after every border crossing. **Confirmed empirically:**

```
TestManeuverOffsetOverlap: offsets: [0 2]  →  OFF-BY-ONE: leg1 offset = 2, want 1
```

The existing `TestMergeShapes` asserts the buggy value (`offsets[1] != 2`) and documents it as intended, so the error is baked into the test suite. Fix direction: subtract the stripped overlap (`offsets[i] = len(flat)/2 - (overlap > 0 ? 1 : 0)`), and correct/expand the tests to assert that a leg1 maneuver's `BeginShapeIndex` maps to the actual merged point.

### 5. Overlap tolerance in `mergeShapes` is far too tight to ever match real border points ✅ FIXED
`internal/server/merge.go:151`

```go
dx := prevLat - curLat
dy := prevLon - curLon
if math.Sqrt(dx*dx+dy*dy) < 1e-6 {   // ~0.1 m, in raw degrees
    overlap = 2
}
```

The dedup only triggers when the two sub-routes' shared border point matches to within ~0.1 m in degree space. But the two legs are produced by **independent Valhalla instances** snapping to their own regional road networks, so the same nominal border crossing regularly resolves to coordinates differing by more than 0.1 m. When the overlap isn't detected:
- The shared point is duplicated in the merged shape (a visual glitch).
- Elevation arrays go out of sync with shape points (the `skip = 1` logic in the elevation merge only fires when overlap was detected).

Fix direction: dedupe by a realistic tolerance (e.g. a few meters via `haversine`, or compare against the *expected* shared endpoint rather than exact equality), and add a test with slightly-mismatched border coordinates.

### 6. `parseHosts` / `parsePorts` panic on a malformed config entry ✅ FIXED
`internal/valhalla/config.go:44-66`, `internal/pelias/config.go:44-66`

```go
parts := strings.Split(host, ":")
res[parts[0]] = parts[1]   // parts[1] → index out of range if no ':' present
```

A config value such as `VALHALLA_REGION_HOSTS=benelux` (missing the `:host` part) panics at bootstrap instead of failing cleanly. **Confirmed empirically** (`TestParseHostsNoColon` → `index out of range [1] with length 1`). Operator misconfiguration should produce a clean error, not a crash. Fix direction: validate `len(parts) == 2` (also note IPv6 host values would break this split) and return an error.

### 7. `exclude_locations` / `exclude_polygons` documented as implemented but ignored; `RouteResponse.summary` never populated ✅ FIXED
`internal/server/route.go:160` (`createRequestOptions`), `routerservice/README.md`, `protos/router/v1/router.proto`

- The README's request-options table marks `exclude_locations` and `exclude_polygons` as **Yes (implemented)**, but `createRequestOptions` never reads them — they are silently dropped and never forwarded to Valhalla (`avoid_locations` / `avoid_polygons`). ✅ FIXED — forwarding implemented, and note the review's own suggested Valhalla field names (`avoid_locations`/`avoid_polygons`) were checked against Valhalla's official docs and turned out to be outdated; the correct current wire names are `exclude_locations`/`exclude_polygons`, which is what got wired up (the Go client struct's JSON tags were also wrong and fixed as part of this).
- `RouteResponse` has a `summary` field (`RouteSummary{start_region, end_region}`), but the server never populates it — `buildCombinedRouteResponse`/`buildRouteResponse` leave it nil. ✅ FIXED — `Route` now derives it from the first/last entries of the already-computed region assignment path via a new `buildRouteSummary` helper.

Fix direction: ~~either implement forwarding of excluded locations/polygons (they map directly onto Valhalla's `avoid_locations`/`avoid_polygons`) or mark them "No" in the README; populate `RouteSummary` from the region assignment (the data is already computed in `Route`)~~ done.

### 8. Pelias is fully configured and wired but never used — dead integration ✅ FIXED

`cmd/routerservice/main.go:127,158,252`, `internal/server/server.go:43`, `internal/pelias/resolver.go:7`

The Pelias config is parsed at bootstrap, stored as app data, threaded into `NewRouterServer`, and `ResolveApiHost` is implemented — but `Route` never calls any geocoding, and `PeliasConfig()`/`ResolveApiHost` are never referenced from production code. The README lists Pelias as a dependency ("address resolution"), but no such feature exists. This is dead weight: two config tables, two env-var groups, and a resolver with zero callers. Fix direction: either implement the geocoding feature or remove the Pelias config/resolver and its README references.

✅ FIXED — confirmed no future routing scenario needs it: `RouteRequest` is coordinate-only by design (no free-text address field), `searchservice` is the platform's single geocoding owner (consumed by `swayrider-api`, never by routerservice), and region/border-crossing logic is pure lat/lon geometry. The dead `internal/pelias` package, its config wiring in `main.go`/`server.go`, README section, env-var templates, and the `PELIAS_*` env vars/network attachment in `infra/dev/layer-20/compose.yml` were all removed rather than implemented.

### 9. No input validation on request locations ✅ FIXED
`internal/server/route.go:23-100`

`Route` checks `len(req.Locations) < 2` but never validates individual entries. A request like `{"locations": [null, null]}` (or entries with a nil `location`) dereferences nil and panics; out-of-range or `NaN` coordinates flow straight to Valhalla. The proto is `proto3`, so empty messages are valid input. Fix direction: validate each `RouteLocation` (non-nil, non-nil `Location`, lat/lon in range, reject `NaN`/`Inf`) and return `InvalidArgument`.

✅ FIXED — a new `validateLocations` helper is called in `Route` right after the length check; it rejects nil entries, nil `Location`, `NaN`/`Inf` coordinates, and out-of-range lat/lon with `InvalidArgument`. Covered by `TestValidateLocations`.

### 10. Valhalla "no route" responses are returned as success, not mapped to `NotFound` ✅ FIXED
`internal/server/route.go:360-392` (`addTrip`), `routerservice/README.md` (Error Handling)

When Valhalla returns a trip with a non-zero `status` (no route found, etc.), the code copies `Status`/`StatusMessage` into the response and returns HTTP 200 with a non-zero `trip.status`, leaving the client to inspect the field. The README's error-handling table claims "No route found between points → `NotFound`", but that mapping only ever fires for `ErrNoRouteFound` from region assignment — never for a Valhalla-level no-route. Either surface the Valhalla status (map non-zero trip status to `NotFound`/`Unavailable`) or document that clients must check `trip.status`.

✅ FIXED — a new `tripStatusError` helper checks each Valhalla `Route` response in `Route`'s request loop and wraps `logic.ErrNoRouteFound` (already mapped to `NotFound` in `grpcStatus`) when `trip.Status != 0`, so the RPC now fails fast instead of returning a 200 with an unchecked status field. Covered by `TestTripStatusError`.

### 11. `getRoadType` can nil-deref on a sparse Valhalla locate response ✅ FIXED
`internal/logic/routing_request.go:400-436`

```go
resp, err := clnt.Locate(ctx, region, req)
...
switch resp.Edges[0].Edge.Classification.Classification {   // Edge is a *EdgeDetails
```

`resp.Edges[0].Edge` is a pointer (`*EdgeDetails`) with `omitempty`; if the locate response omits the `edge` details object, `resp.Edges[0].Edge.Classification` dereferences nil and panics. Errors from `Locate` are also silently swallowed (returns `nil`), which is at least safe. Fix direction: nil-check `resp.Edges[0].Edge` before dereferencing.

✅ FIXED — the existing empty-edges guard now also checks `resp.Edges[0].Edge == nil`. Confirmed empirically: reverting the guard and running `TestGetRoadType_SparseLocateResponse` (a new test using an `httptest.Server` returning `[{"edges":[{}]}]`) panics with `nil pointer dereference`; with the guard restored it passes.

---

## Low

### 12. Dead code and leftover scaffolding ✅ FIXED
- `_ "time"` blank imports in `internal/server/route.go:8` and `internal/logic/routing_request.go:6` — leftovers from the commented-out timeouts. ✅ Already resolved by commit `770d1cd` (the #3 timeout fix): `route.go` no longer imports `time` at all, and `routing_request.go`'s `"time"` import is a live, non-blank import used for the `timeout time.Duration` parameters on `AddBorderCrossings`/`getRoadType`.
- `internal/server/route.go:43` — `_ = locationList`; the whole `locationList` value returned by `assignRegionsToLocations` is unused. ✅ FIXED — dropped via `_, regionAssignment, err := s.assignRegionsToLocations(...)` and the now-redundant `_ = locationList` line removed.
- Commented-out blocks in `route.go` (`createLocation`'s `OriginalIndex`, `Route`'s timeout, `lg.Infof("Route possible…")`). ✅ FIXED — `Route`'s timeout comment was already removed by `770d1cd`; the dead `OriginalIndex` block (line 559) and the orphaned `lg.Infof("Route possible…")` line (line 119, referencing a `routePossible` variable that isn't even in scope in `Route()`) were deleted. The "`buildRegionList`'s old implementations" block was actually in `internal/logic/region_assignment.go` (not `route.go`) — two large commented-out old implementations after `buildRegionList`'s `return` statement — and were deleted there.

### 13. Preference precedence is implicit and surprising ✅ FIXED
`internal/server/route.go:160-300`

`createRequestOptions` applies options in append order: route-type presets, then `route_options`, then the top-level motorcycle fields (`scenic_preference`, `highway_avoidance`, `toll_avoidance`, `unpaved_handling`). Because the latter are appended last they silently **override** explicit `route_options` (e.g. `route_options.highway_preference=0.8` plus `highway_avoidance=0.9` → 0.1 wins). There is no documentation of this precedence and no test pinning it. Consider documenting it or making the precedence explicit.

✅ FIXED — the top-level motorcycle fields are now applied alongside the route-type preset, before `route_options`, so `route_options.*` always wins on any field it sets — the same "most specific wins" rule already established and tested for the route-type-preset boundary (`TestCreateRequestOptions_ScenicExplicitHighwayOverride`). Doc comments at both blocks make the precedence explicit. Covered by four new regression tests: `TestCreateRequestOptions_HighwayAvoidanceExplicitRouteOptionsOverride`, `TestCreateRequestOptions_TollAvoidanceExplicitRouteOptionsOverride`, `TestCreateRequestOptions_UnpavedHandlingExplicitRouteOptionsOverride`, `TestCreateRequestOptions_ScenicPreferenceExplicitRouteOptionsOverride`.

### 14. `buildCombinedRouteResponse` indexes `respList[0]` without a length guard ✅ FIXED
`internal/server/route.go:306`

```go
resp, err := buildRouteResponse(respList[0], l)
```

Currently unreachable (≥ 2 locations guarantees ≥ 1 assignment), but a one-line guard (`if len(respList) == 0`) would make the function robust to future changes.

✅ FIXED — `buildCombinedRouteResponse` now returns an error (mapped to `codes.Internal` via `grpcStatus`'s default case) when `respList` is empty, before indexing `respList[0]`.

### 15. `defer cancel()` inside the Valhalla loop accumulates until function return ✅ FIXED
`internal/server/route.go:90`

Each iteration defers a `cancel()` that only runs when `Route` returns. Harmless today but a footgun if the loop grows; cancel per-iteration (or use `defer` scoped to the loop body via a helper).

✅ FIXED — already resolved as a side effect of the #3 timeout fix: the loop now does `reqCtx, cancel := context.WithTimeout(ctx, s.valhallaConfig.RequestTimeout)` followed by an explicit `cancel()` call right after `vhClient.Route(...)` returns, not a deferred one.

### 16. Info-level logging of internal routing state ✅ FIXED
`internal/server/route.go:96` — `lg.Infof("regionAssignment: %v", regionAssignment)` logs the full region assignment on every request at info level. Prefer debug-level.

✅ FIXED — changed to `lg.Debugf(...)`.

### 17. Dockerfile hardening ✅ FIXED
`Dockerfile`

- `FROM golang:latest` and `FROM debian:bookworm-slim` — unpinned, mutable base tags; builds are not reproducible.
- `COPY . .` with **no `.dockerignore`** — the build context ships `.git/`, `local.env` (machine-specific paths), `.DS_Store`. Add a `.dockerignore`.
- `CGO_ENABLED=1` with cross-gcc toolchains for both arches — verified unnecessary: `CGO_ENABLED=0 go build ./cmd/routerservice/` succeeds, so the whole cross-compiler block can be dropped for a static binary.
- No `HEALTHCHECK` (the service exposes a `Ping` RPC that could drive one).

✅ FIXED — verified against the current `Dockerfile`: a `.dockerignore` exists (excludes `.git`, `.gitignore`, `.DS_Store`, `*.md`, `local.env`); `CGO_ENABLED=0` is set with no cross-gcc toolchain block; a `HEALTHCHECK` hitting the `Ping` endpoint is present; the builder stage is pinned to `golang:1.26-bookworm` (no longer `latest`). The runtime stage still uses the floating `debian:bookworm-slim` tag rather than a digest pin — reviewed and accepted as-is (not `latest`; digest-pinning trades reproducibility for losing automatic security patches).

### 18. `HealthService.Check` is unimplemented ✅ FIXED
`internal/server/server.go:60-77`, `protos/health/v1/health.proto`

The proto defines a `Check` RPC (`GET /api/v1/health`), but `HealthServer` embeds `UnimplementedHealthServiceServer` and implements only `Ping` — `Check` returns `codes.Unimplemented`. (Unlike regionservice, the README here doesn't document it, so it's lower priority, but the endpoint is still routed and broken.) Implement a trivial always-UP `Check` or remove it from the proto.

✅ FIXED — rather than a trivial always-UP stub, `Check` now performs a real dependency check: it pings regionservice (via the existing `regionClient.Ping()`) and, for every Valhalla instance explicitly configured via `-valhalla-region-hosts`/`-valhalla-region-ports`, calls a new `Status` method on the Valhalla REST client (`restclients/valhalla/client.go`, hitting `/status`) with a short 3s per-call timeout. `Check` returns `UP` only if all of these succeed, `DOWN` otherwise. `HealthServer` now carries `regionClient`/`valhallaConfig` (threaded through `NewHealthServer` and `main.go`'s `grpcHealthRegistrar`, mirroring `RouterServer`'s existing dependency wiring), and `Check` is registered as a public endpoint alongside `Ping`. Regions resolved via the default prefix/postfix naming convention (rather than explicit host/port overrides) are not covered — there's no RPC to enumerate all regions, and building one was judged out of scope for this item. Covered by `TestCheck_AllDependenciesUp`, `TestCheck_RegionserviceDown`, `TestCheck_ValhallaDown`, `TestCheck_NoValhallaHostsConfigured`, and `TestStatus_*` for the new Valhalla client method.

---

## Positive observations

- **Auth is correctly enforced** — `main.go` passes `app.AuthInterceptor | app.ClientInfoInterceptor` with a real `JWTPublicKeysFn` (background key-fetch loop into a cache), and `server.go`'s `init()` declares `PublicEndpoint(Ping)` / `UserOrServiceEndpoint(Route, ["routing:execute"])`. This is the right pattern — and the contrast with `regionservice` (which passes `NoInterceptor`) is worth keeping in mind.
- **Clean layering and small files** — `server → logic → config → restclients` with clear responsibilities; the option-pattern (`RoutingRequestOption`/`Apply`) is a tidy way to translate request fields into Valhalla costing options, and the `Set*Preference` helpers clamp to `[0,1]`.
- **Centralized error mapping** — `grpcStatus` maps sentinel errors to `Unavailable`/`NotFound`/`Internal` and passes through downstream gRPC statuses.
- **JWT forwarding** — the caller's token is propagated to every regionservice call with proper context threading (commit `866d3ec`/`7d08786`), so downstream auth works end to end.
- **Good pure-function tests** — `internal/valhalla` (82.5%) and `internal/pelias` (74.3%) are well covered; the road-type ordering, corridor math, and merge sub-function tests are genuinely useful and readable.
- **Graceful corridor fallback** — `CalculateRegionAssignment` warns and falls back to `FindRegionPath` when `FindRouteRegionPaths` fails, rather than failing the request.
- **Build/vet/tests clean** — `go build`, `go vet`, and the existing test suite all pass.

---

## Test-coverage gaps

Measured with `go test -cover`:

| Package | Coverage | Notes |
| ------- | -------- | ----- |
| `internal/valhalla` | 82.5% | Config parsing / host resolution — good. |
| `internal/pelias` | 74.3% | Config parsing — good (though the feature is dead, see #8). |
| `internal/logic` | 38.2% | **The orchestration is 0% covered** (see below). |
| `internal/server` | 23.1% | **`Route` and the whole response-building path are 0% covered** (see below). |
| `cmd/routerservice` | 0% | No startup/bootstrap/auth test. |
| `restclients/valhalla` | 0% | HTTP client untested. |
| `restclients/valhalla/types` | 0% | Untested. |

Specific gaps:

- **`Route` is 0% covered** (`internal/server/route.go:23`) — the single public endpoint, including `buildCombinedRouteResponse`, `addTrip`, `addLocations`, `createLocation`, `createLeg`, `createManeuver`, `createTripSummary`, `createLegSummary`, and `incomingToken`. None of the response-mapping code is exercised.
- **`CreateRoutingRequests` and `AddBorderCrossings` are 0% covered** (`internal/logic/routing_request.go:202,254`) — the two functions containing critical bugs #1 and #2. `getRoadType` (0%) and the option constructors (0%) are also untested.
- **`CalculateRegionAssignment` / `injectTransferRegions` / `buildRegionList` / `ResolveRegions` / `ResolveRegion` are 0% covered** (`internal/logic/region_assignment.go:26,103,192`; `resolve_region.go`) — the entire region-assignment orchestration is unverified.
- **`mergeRouteResponse` and `mergeLegGroup` are 0% covered** (`internal/server/merge.go:18,75`) — only the leaf helpers (`mergeShapes`, `flattenManeuvers`, `handleBorderManeuvers`, `mergeSummaries`, `findMergeGroups`) are tested. The top-level merge orchestration that calls them is not, and the existing `TestMergeShapes` asserts the buggy offset value (#4) rather than correct behavior.
- **No empty/edge-input tests** — no tests for empty crossing lists (#2), empty/transfer assignments (#1), nil locations (#9), malformed config entries (#6), or sparse locate responses (#11).
- **No e2e/integration test** — nothing boots the server with a fake regionservice/Valhalla to exercise the full route flow; both critical bugs would be caught immediately by such a test.

---

## Recommended fix order

1. **#1 (critical)** — ✅ FIXED (commit `35a7d89`) — handle `IsEmpty`/transfer-region assignments in `CreateRoutingRequests` (skip them or assign through-points); add a regression test. This is a guaranteed crash on ordinary cross-border routes.
2. **#2 (critical)** — ✅ FIXED — guard `len(crossings) == 0` in `AddBorderCrossings`; add a regression test with an empty crossing list.
3. **#3 (high)** — ✅ FIXED — restore timeouts on all Valhalla HTTP calls (derive from the incoming RPC ctx with a deadline, or an `http.Client` timeout); fix the `defer cancel()` accumulation.
4. **#4 (medium)** — ✅ FIXED — fix the off-by-one in `mergeShapes` offsets and correct the tests that pin the buggy value.
5. **#5 (medium)** — ✅ FIXED — relax the shape-overlap tolerance to a realistic distance so border points actually dedupe.
6. **#6 (medium)** — ✅ FIXED — validate `parseHosts`/`parsePorts` input (fail cleanly on malformed config).
7. **#7 (medium)** — ✅ FIXED — `exclude_locations`/`exclude_polygons` forwarding to Valhalla implemented; `RouteResponse.summary` populated from the region assignment. **#8 (medium)** — ✅ FIXED — removed the dead Pelias integration. **#9 (medium)** — ✅ FIXED — added request-location validation. **#10 (medium)** — ✅ FIXED — Valhalla no-route now maps to `NotFound`. **#11 (medium)** — ✅ FIXED — nil-guarded `getRoadType`.
8. **#12 (low)** — ✅ FIXED — removed dead code/blank imports. **#13 (low)** — ✅ FIXED — `route_options.*` now always takes precedence over the top-level motorcycle fields, with the rule documented and pinned by tests. **#14 (low)** — ✅ FIXED — guarded `respList[0]` against an empty list. **#15 (low)** — ✅ FIXED — Valhalla loop already cancels per-iteration rather than deferring. **#16 (low)** — ✅ FIXED — region-assignment logging dropped to debug level. **#17 (low)** — ✅ FIXED — `.dockerignore`, `CGO_ENABLED=0`, `HEALTHCHECK`, and a pinned builder image are all in place. **#18 (low)** — ✅ FIXED — `HealthService.Check` now performs a real dependency check against regionservice and configured Valhalla instances, returning `UP` only if all are reachable.

Items #1 and #2 are the priority: both are uncaught process crashes on the service's core multi-region feature, and both live in code with zero test coverage.
