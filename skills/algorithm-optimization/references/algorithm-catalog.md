# Algorithm selection catalog

Optional background for [Algorithm Optimization](../SKILL.md). The entrypoint owns
activation, proof, measurement, and stopping rules; these tables do not authorize a rewrite.
Complexity depends on assumptions and implementation. Use measured workload bounds.

## Profiling suggestions

| Language | CPU | Memory/allocation | Blocking/contention |
|---|---|---|---|
| Go | `go test -bench=. -cpuprofile=cpu.out`; `go tool pprof -top` | `-memprofile=mem.out`; pprof | `-blockprofile=block.out`; pprof |
| Python | `python -m cProfile -s cumtime script.py`; `py-spy top` | memory_profiler; tracemalloc | threading; `py-spy dump --native` |
| Node.js | `node --prof app.js`; `node --prof-process`; clinic doctor | `node --inspect`; DevTools Memory | trace events; clinic bubbleprof |
| Rust | perf; cargo flamegraph | heaptrack; dhat | perf lock; kernel lockdep |
| Java | jstack; async-profiler | `jmap -histo`; Eclipse MAT | jstack; AsyncGetCallTrace |
| C/C++ | perf record/report | valgrind massif | valgrind helgrind |

## Problem classes

| Class | Approach | Complexity/assumptions |
|---|---|---|
| Unsorted search | Hash map | O(1) average lookup after indexing |
| Sorted search | Binary search | O(log n) |
| Prefix/range search | Trie / B-tree / segment tree | O(k) / O(log n), plus output |
| Sorting | Comparison / counting / radix | O(n log n) / O(n+k), subject to key assumptions |
| Unweighted shortest path | BFS | O(V+E) |
| Non-negative weighted shortest path | Dijkstra with heap | O((V+E) log V) |
| Negative-edge shortest path | Bellman-Ford | O(VE) |
| All-pairs shortest path | Floyd-Warshall / Johnson | O(V³) / O(V² log V + VE), implementation-dependent |
| Minimum spanning tree | Prim / Kruskal | O(E log V) |
| Topological order | Kahn / DFS postorder | O(V+E), DAG |
| Connected components | Union-Find | O(V + E α(V)) |
| Subset/combination optimization | Knapsack DP | O(n × capacity) |
| Sequence alignment/LCS | DP / Hirschberg | O(mn) time; Hirschberg reduces space to O(min(m,n)) |
| String matching | KMP / Boyer-Moore / Rabin-Karp | O(n+m) for KMP; O(nm) worst case for some variants |
| Static range queries | Prefix sum / sparse table | O(1) query; preprocessing depends on operation |
| Dynamic range queries | Fenwick / segment tree | O(log n) update/query |
| Nearest neighbor | KD-tree / ball tree | Average behavior depends on dimension/distribution |
| Interval scheduling | Greedy earliest finish | O(n log n) |
| Flow / matching | Ford-Fulkerson / Edmonds-Karp / Hopcroft-Karp | Capacity-dependent / O(VE²) / O(E√V) |

## Access patterns and transformations

| Access pattern | Candidate |
|---|---|
| Unordered key/value | Hash map: O(1) average lookup |
| Ordered key/value | B-tree / skip list: O(log n) |
| Prefix/pattern | Trie / suffix tree: query-length dependent |
| Repeated min/max | Heap: O(log n) extraction, variant-dependent insertion |
| FIFO/LIFO | Queue / stack: O(1) operations |
| Static ranges | Prefix sum / sparse table |
| Dynamic ranges | Fenwick / segment tree |
| Disjoint sets | Union-Find with path compression: O(α(n)) amortized |
| LRU eviction | Doubly linked list + hash map: O(1) |
| Sorted rank | Order-statistic tree / skip list: O(log n) |
| Spatial queries | Quad tree / KD-tree / R-tree |

| Current cost/pattern | Candidate transformation |
|---|---|
| Nested linear scans, O(n²) | Two passes with map lookup, O(n) average |
| m linear queries, O(nm) | O(n) preprocessing then O(1) lookup where applicable |
| Sorted-slice insertion, O(n) | Heap/tree insertion, O(log n), if access semantics fit |
| Linear search | Sort once, then binary search |
| Exponential overlapping recursion | Memoization or bottom-up DP |
| Repeated sort + range scan | Fenwick/segment tree; account for query as well as update costs |
| Loop string concatenation | Builder, O(n) total output work |
| Repeated regex compilation | Compile once outside loop |
| Contended channel pipeline | Bounded worker pool, batching, sized buffers |
| Hot-path deep copy | Copy-on-write or versioned immutable references with synchronization |

## Growth and space trade-offs

Operation counts are not wall-clock guarantees:

| Class | N=100 | N=10,000 | N=1,000,000 |
|---|---|---|---|
| O(1) | 1 | 1 | 1 |
| O(log₂ n) | ~7 | ~13 | ~20 |
| O(√n) | 10 | 100 | 1,000 |
| O(n) | 100 | 10,000 | 1,000,000 |
| O(n log₂ n) | ~700 | ~130,000 | ~20,000,000 |
| O(n²) | 10,000 | 100,000,000 | 10¹² |
| O(n³) | 1,000,000 | 10¹² | 10¹⁸ |

Exponential and factorial growth become prohibitive even at small N; measure actual bounds.

| Strategy | Time benefit | Space/behavior cost |
|---|---|---|
| Memoization | Avoid exponential repeated work | Stored states, often O(n) or O(n²) |
| Precomputed lookup | O(1) queries | O(n) or O(key range) storage |
| Bloom filter | Fast membership rejection | False positives; not an exact membership replacement |
| Copy vs reference | Avoid copies | Ownership, mutation, and synchronization obligations |
| In-place vs out-of-place | Reduce allocation | Verify ordering/aliasing behavior remains identical |

## Worked documentation example

For a measured search bottleneck, a useful record distinguishes index construction,
lookup, and output iteration rather than calling the entire operation "O(1)":

```text
Algorithm: pre-built map[normalizedQueryPrefix][]User
Build: O(n) time and space under fixed prefix bounds
Query: O(1) average lookup + O(k) output iteration
Alternative: sorted slice, O(log n + k) lookup but O(n) insertion shifts
Invariant: rebuild index on every write; queries are read-only
Decision: map is simpler than trie for this measured prefix length and population
```

Dijkstra's structured-programming argument is about provable, visible control flow,
not banning multiple explicit returns. Preconditions, invariants, postconditions,
and termination make the correctness argument; a benchmark does not replace it.
