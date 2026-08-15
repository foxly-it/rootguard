# Performance and memory baseline

Part of `ROADMAP.md`'s 0.9 checklist: "Performance and memory baselines for
small and medium networks." RootGuard's guided setup has a `resource_profile`
setting (`small`/`medium`/`large`, default `medium`) that only tunes Unbound's
`rrset-cache-size`/`msg-cache-size` (32/16 MB, 64/32 MB, 128/64 MB) - there's
no other codified definition of what traffic level each label represents, so
the numbers below define that explicitly rather than assuming a shared
understanding.

## Test environment

`v0.1.0-beta.1`, default `medium` resource profile, on a deliberately
constrained single-node host: 1 vCPU, 2 GB RAM (the dedicated 0.9
endurance-test host, `debian-test2`). This is well below what most real
deployments will run on - the numbers below are a conservative floor, not a
capacity ceiling for typical hardware. `nproc` confirms the single-core
limit; the medium-network throughput ceiling measured here is a property of
that one core, not of Unbound/AdGuard's own efficiency.

## Small network - passive baseline from the 0.9 endurance test

`scripts/soak/probe.sh` samples `docker stats` for all five managed
containers every ~10 minutes as a side effect of its regular DNS/filtering
health checks (light, bursty query load - a handful of devices, occasional
lookups). First 60 samples (~10.5 hours, 2026-08-14 through 2026-08-15),
before any synthetic load:

| Container | Steady-state memory |
| --- | --- |
| `rootguard-unbound` | ~12 MB, growing slowly with cache fill (RSS, not the 64 MB `msg-cache-size` ceiling - caches don't start full) |
| `rootguard-adguard` | ~47-48 MB |
| `rootguard-core` | ~4 MB (one 264 MB outlier immediately after a live backup-restore-drill run - expected, not steady-state) |
| `rootguard-webapp` | ~2-2.4 MB |
| `rootguard-updater` | ~1.5-2.6 MB |

Probe pass rate over this window: 60/60 (100%), zero incidents. This table
will be refreshed with the full 30-day sample at endurance-test close-out
([rootguard#271](https://github.com/foxly-it/rootguard/issues/271)).

## Medium network - synthetic sustained load

Defined here as 20 sustained queries/second (≈1,200/minute) against a mix of
20 real second-level domains, using `dnsperf` (`dnsperf -s 127.0.0.1 -p 53
-d queries.txt -l 60 -Q 20`), run once against the same live instance without
disturbing it (read-only from RootGuard's perspective - no teardown, no
config change):

- **Completed:** 1,151 / 1,200 queries (95.9%); the rest timed out under the
  host's single-core contention, not a RootGuard-side rejection (response
  codes were 100% `NOERROR` for every completed query - nothing was refused
  or errored, queries that timed out never got a response in the allotted
  window at all).
- **Latency:** average 0.77 ms, min 0.14 ms, max 227 ms, stddev 8.06 ms - the
  long tail comes from cache misses needing a real recursive lookup;
  cache-hit responses are sub-millisecond.
- **Memory after the run:** `rootguard-unbound` 15.5 MB, `rootguard-adguard`
  62.6 MB, `rootguard-core` 4.1 MB - a few MB above the small-network
  baseline, not a leak signature (no unbounded growth across the run).

An unthrottled max-throughput run (no `-Q` cap) against the same one-vCPU
host achieved only ~4 real QPS with 83% timeouts - included here as a
data point about the *test host's* ceiling, not RootGuard's; a synthetic
open-loop flood against a single shared core isn't a realistic multi-device
network's actual query pattern (real devices don't all fire simultaneously
without waiting for a reply), which the 20 QPS closed-loop-ish result above
reflects more honestly.

## Follow-up

These numbers satisfy the 0.9 checklist item's letter, but a rerun on
representative hardware (2+ vCPU, matching what the Quick Start's minimum
requirements actually recommend) would make the medium-network ceiling
finding more useful than "one vCPU is the bottleneck, not RootGuard" - noted
as a nice-to-have, not blocking 0.9.
