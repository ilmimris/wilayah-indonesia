# N-Gram Search Optimization Plan

## 1. Benchmark Baseline
- Run the current accuracy suite on both `main` and `feat/ngram-search` to capture district precision/recall along with latency and memory profiles.
- Record representative queries that currently fail or spike resource usage so fixes can be validated against them.

## 2. Fragment Coverage Fix
- Restore richer query fragments by setting the default `wordComboSize` back to 3 and lifting the fragment cap to 8–10 within the matcher.
- Re-run `scripts/evaluate_fragment_generation.py` to confirm long multi-word district names are covered.
- Add regression tests in the matcher package to lock in fragment coverage for the problematic examples.

## 3. Suggestion Confidence Guard
- Raise `matcherMinScore` (target ≥ 0.8) and require per-level similarity checks before auto-populating request filters.
- Instrument matcher outputs to log suggestion score distributions, helping calibrate thresholds.

## 4. Match Quality Tuning
- Increase district (and optionally city) similarity thresholds to ~0.55–0.60 to reduce ambiguous hits.
- Add a province-aware tie-break when duplicate district names appear in different regions.
- Extend `internal/usecase/region/matcher/percolator_test.go` with ambiguity scenarios to prove correctness.

## 5. Resource Controls
- Implement bounded top-K selection inside `pkg/utils/ngram.Search` so only the top matches are materialised.
- Disable or cap result caching for truncated queries to keep memory usage predictable.
- Stress-test with worst-case fragment sets while monitoring RSS/CPU to confirm improvements.

## 6. Validation & Rollout
- Re-run the full evaluation suite (accuracy, latency, resource metrics) after each change set.
- Summarise before/after metrics and update `OPTIMIZATIONS.md` with the new defaults and tuning guidance.
- Document any new configuration knobs (thresholds, fragment settings) in README and deployment manifests.
