# N-gram Percolator Implementation Plan

## Objective
- Deliver hierarchical-aware auto-suggestions so search queries can be pre-filtered with `province`, `city`, `district`, and `subdistrict` parameters.
- Run n-gram matching for each administrative level in parallel, then reconcile results with a percolator that enforces the `VV.XX.YY.ZZZZ` region ID hierarchy.
- Provide a best-effort fallback to the existing sequential matching approach when the optimistic parallel pass cannot produce a coherent hierarchy.

## Current State Snapshot
- `pkg/utils/ngram` exposes a synchronous `Search` with caching; reads are safe but there is no higher-level orchestration.
- `internal/usecase/region/matcher` already builds per-level indices and sequentially loops through candidate fragments before harmonising results.
- Region IDs encode hierarchy but we do not yet parse or validate component prefixes when surfacing suggestions.

## Proposed Architecture
- Keep per-level n-gram indices (`province`, `city`, `district`, `subdistrict`) but read them concurrently per query.
- Introduce a percolator component that inspects the top-k matches per level and selects combinations with shared prefixes (`VV`, `VV.XX`, etc.).
- Expose percolated suggestions through the matcher so API handlers can plug them into repository search parameters.
- Preserve the sequential matcher as a fallback path for difficult inputs or misaligned caches.

## Implementation Steps
1. **Baseline audit & config**
   - Anchor the n-gram source data on `data/wilayah.sql`; document the exact SQL exports used to hydrate indices so rebuilds remain reproducible.
   - Confirm the SQL dump exposes stable `RegionID` values in the `VV.XX.YY.ZZZZ` pattern and add validation in the ingestion path if any rows deviate.
   - Capture desired search thresholds, top-k counts per level, and timeout budget in `internal/config` so behaviour is tunable.

2. **Serialized index bootstrap**
   - Extend the ingestor pipeline (see `cmd/ingestor/main.go`) so it reads `data/wilayah.sql`, materialises per-level (`province`, `city`, `district`, `subdistrict`) n-gram indices, and writes each snapshot to a Pickle file.
   - Store metadata alongside the Pickle payloads (dataset version, build timestamp) so stale caches can be invalidated deterministically.
   - Ensure the ingestor overwrites Pickle snapshots after every successful rebuild while the API process simply loads the most recent artefacts.

3. **Hierarchy utilities**
   - Add helper in a new `pkg/regionhierarchy` (or similar) to parse IDs into `{Province, City, District, Subdistrict}` segments and compare prefixes.
   - Provide helpers to compute parent keys (`VV`, `VV.XX`, `VV.XX.YY`) and to detect mismatches.

4. **Parallel n-gram query fan-out**
   - Extend `internal/usecase/region/matcher` with a `searchLevel(level, fragment string, k int) []Match` helper that wraps `NGram.Search` and returns the top-k sorted matches.
   - Launch goroutines per level (bounded with a worker group and context) for each candidate fragment, collect results through channels, and deduplicate per level.
   - Ensure caching in `NGram` remains safe under concurrent reads; guard any new shared state with mutexes.

5. **Candidate aggregation**
   - For each level, keep the best `k` matches across all fragments (e.g. `k=5`) and normalise names (trim, lowercase) for downstream comparison.
   - Record metadata needed by the percolator (region ID, parent names, similarity score, fragment origin).

6. **Percolator implementation**
   - Create `internal/usecase/region/matcher/percolator.go` that:
     - Accepts the per-level match slices.
     - Groups matches by hierarchy prefix (province code, city code, etc.).
     - Scores candidate paths using weighted similarity (e.g. province 0.2, city 0.3, district 0.25, subdistrict 0.25) and rewards consistent prefixes.
     - Returns the highest scoring coherent path (or multiple ranked suggestions if needed).
   - Allow configuration of weights and the minimum combined score to accept.

7. **Fallback sequential path**
   - If the percolator yields no valid path (e.g. conflicting prefixes, all below threshold), run the existing sequential `CandidateLoop` logic as a fallback.
   - Consider instrumenting when the fallback is triggered for observability.

8. **API integration**
   - Update the region use case to surface percolated suggestions alongside existing search results (e.g. extend `model.SearchResponse` or keep suggestions internal to set `req.Province`, etc.).
   - Ensure the ingestor or repository layer can accept the narrowed filters without additional round trips.
   - Expose optional query parameters so API clients can inspect what the percolator chose (e.g. `suggested_province`).

9. **Testing & verification**
   - Write table-driven unit tests for hierarchy parsing and percolator scoring (cover exact match, mismatched prefixes, partial data).
   - Add concurrency-safe tests ensuring parallel search honours timeouts and does not leak goroutines.
   - Extend integration tests in `pkg/service` or `internal/usecase` to confirm suggestions drive correct repository queries and fallback triggers when expected.
   - Benchmark parallel vs sequential paths with representative inputs to validate performance gains.

## Observability & Performance
- Add logging and counters (behind `slog` and possibly `expvar`/prometheus hooks) for cache hits, percolator success rate, and fallback usage.
- Surface trace spans or structured logs with query fragments, selected hierarchy, and timing to aid debugging.

## Risks & Decisions
- Enforce an end-to-end latency budget under 100 ms per API request; parallel fan-out and percolator timeouts must honour this cap.
- Return only the top-ranked percolated path, keeping the current API contract stable.
- Serve read-only snapshots of the n-gram indices to the API; trigger rebuilds via the ingestor to avoid mid-query mutations.
- Accept partial hierarchies—if only province (or similar) matches, return that without forcing deeper levels.
