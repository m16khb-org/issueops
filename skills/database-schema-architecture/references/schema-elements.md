# Schema Elements Reference

Use this file after the first-turn Database Schema Architecture contract confirms a
relational schema is in scope. Every rule names its engine when engines differ.
Verify version-specific behavior against the engine's documentation before
recommending it.

## Measure Row Counts And Resource Usage

Decide from numbers, not table names. For each table in scope, record:

- Row count, and whether it is exact (`COUNT(*)`) or a catalog estimate. Estimates are cheaper and usually enough for design; say which one you used.
- Heap, index, and TOAST/LOB size, and bytes per row (total size / rows).
- Growth per day or month, and the retention rule if one exists.
- Read/write mix: inserts, updates, deletes per second or per day.
- Index usage: scans per index, and indexes that are never used but still cost every write.
- Cache behavior: buffer or buffer-pool hit ratio for the hot tables.

PostgreSQL:

```sql
SELECT relname,
       reltuples::bigint AS estimated_rows,
       pg_size_pretty(pg_relation_size(oid)) AS heap,
       pg_size_pretty(pg_indexes_size(oid)) AS indexes,
       pg_size_pretty(pg_total_relation_size(oid)) AS total
FROM pg_class
WHERE relkind IN ('r', 'p') AND relnamespace = 'public'::regnamespace
ORDER BY pg_total_relation_size(oid) DESC
LIMIT 20;

SELECT relname, n_tup_ins, n_tup_upd, n_tup_del, n_dead_tup, last_autovacuum
FROM pg_stat_user_tables
ORDER BY n_tup_ins + n_tup_upd + n_tup_del DESC
LIMIT 20;
```

MySQL/InnoDB:

```sql
SELECT table_name, table_rows AS estimated_rows,
       ROUND(data_length / 1024 / 1024) AS data_mb,
       ROUND(index_length / 1024 / 1024) AS index_mb
FROM information_schema.tables
WHERE table_schema = DATABASE()
ORDER BY data_length + index_length DESC
LIMIT 20;
```

Avoid full `COUNT(*)` scans on large production tables; a heavy query can drain
the connection pool. Row-count classes and their consequences are in
`deep-database-review.md`.

## Primary Keys

State the key type, width, and insert order for every table.

- Surrogate integer (`bigint` identity or sequence): narrow, ordered inserts, cheap joins. Use `bigint` for tables that can outgrow 2^31 rows.
- Random UUID (v4): 16 bytes, random insert position. On a clustered table this spreads inserts across pages, raises page splits, and enlarges every secondary index that stores the key.
- Time-ordered identifiers (UUIDv7, ULID): globally unique like UUIDs with mostly ordered inserts. Check engine and driver support before choosing one.
- Natural key: use only when the value is immutable and short; a changing natural key forces cascading updates.
- Composite key: fine for join tables; put the most selective or most queried column first and check that dependent foreign keys stay readable.

Engine notes:

- MySQL/InnoDB stores rows in the clustered primary key index, and each secondary index record contains the primary key columns. A long primary key makes every secondary index larger.
- PostgreSQL stores rows in a heap, so the primary key is an ordinary unique index. Key width still costs index size and join memory, but insert order matters less for the table itself.

## Foreign Keys

For each relationship, decide enforcement, action, and index together.

- Enforce in the database unless a measured write path cannot afford it; application-only enforcement must name who repairs orphans.
- Choose `ON DELETE`/`ON UPDATE` from the domain: `RESTRICT`/`NO ACTION` by default, `CASCADE` only for true ownership, `SET NULL` only when the column is nullable by design.
- PostgreSQL does not create an index on the referencing columns when a foreign key is declared. Deleting or updating a referenced row then scans the referencing table; index the referencing columns unless the parent is never deleted or updated.
- MySQL/InnoDB requires an index whose leading columns are the foreign key columns and creates one on the referencing table if none exists. Check that the index it created is the one you want, and that a later index does not silently replace it.
- Foreign key checks take locks on the referenced row; heavy concurrent inserts into children of one hot parent can contend.
- Partitioned tables limit foreign key support differently by engine and version; confirm before combining them.

## Indexes And Covering Indexes

Start from the query: equality predicates, then range, then sort (detail in
`deep-database-review.md`). A covering index lets the engine answer from the
index without reading the table row.

- PostgreSQL: add non-key columns with `INCLUDE (...)` to allow an index-only scan. An index-only scan skips the heap only for pages marked all-visible in the visibility map, so a table that VACUUM cannot keep up with gains little. Check `Heap Fetches` in `EXPLAIN (ANALYZE, BUFFERS)`.
- MySQL/InnoDB: secondary indexes already carry the primary key, so a query that needs only indexed columns plus the primary key is covered. `EXPLAIN` shows `Using index`.
- Every covering column widens the index and adds write cost. Cover only a measured hot query, and record the size before and after.
- Remove indexes with no scans after a representative period instead of adding more; each one costs every insert and update.

## Partitioning

Partition for data lifecycle or pruning, not as a first answer to slow queries.

- Justify with row count, size, and growth, plus the query shape that prunes partitions (the partition key appears in the `WHERE`).
- Common keys: time range for append-heavy event data with retention; list or hash for tenant or ID spread.
- PostgreSQL: a unique or primary key constraint on a partitioned table must include all partition key columns, and the partition key cannot be an expression in that case. Global uniqueness on another column needs a different design.
- Plan partition creation ahead of time, retention by detaching or dropping partitions, and how existing indexes are built on each partition.
- Too many small partitions raise planning time and catalog overhead; compare partition count with query latency.

## ORM Characteristics

Review the SQL and DDL the ORM produces, not only the entity code.

- Fetching: lazy loading over a list causes N+1 queries; eager loading or joins can over-fetch wide rows. Capture the query count and SQL for one representative request.
- Selected columns: entities that load every column defeat covering indexes. Use projections for hot read paths.
- ID generation: an identity column assigns the key on insert, so some ORMs cannot batch inserts for it (Hibernate disables JDBC insert batching for `IDENTITY`). Sequence or client-generated keys keep batching available.
- Types and lengths: check generated column types, string lengths, numeric precision, timestamp time-zone handling, and enum storage (stored ordinal values break when the enum order changes).
- Constraints: confirm the ORM migration actually creates the foreign keys, unique constraints, and indexes the model declares, with explicit names.
- Soft delete: a `deleted_at` filter needs indexes and unique constraints that match it (for example a partial unique index on live rows in PostgreSQL).
- Transactions: know where the ORM opens and commits transactions, and keep external calls outside them.
- Migrations: read generated migrations before applying; autogenerated drops, renames, and type changes can rewrite or lock large tables.
