-- 002_trigram_search_indexes.sql: accelerate the ⌘K global search's substring
-- ILIKE '%...%' queries (internal/dashboardapi/search.go).
--
-- A plain B-tree index only helps a prefix match ('term%'); every '%term%'
-- predicate here forced a full table scan on transactions.name_orig/name_dest
-- (the largest table by row count in the system) on every keystroke.
-- CockroachDB's trigram index (GIN, gin_trgm_ops) is the documented mechanism
-- for exactly this: "Pattern matching with LIKE and ILIKE is accelerated by a
-- trigram index" (cockroachlabs.com/docs/stable/trigram-indexes). It only
-- helps a search term of 3+ characters — search.go already requires at least
-- 2, so a 2-character search still falls back to a scan, which is an
-- acceptable, documented limitation rather than a bug.
CREATE INDEX IF NOT EXISTS transactions_name_orig_trgm_idx ON transactions USING GIN (name_orig gin_trgm_ops);
CREATE INDEX IF NOT EXISTS transactions_name_dest_trgm_idx ON transactions USING GIN (name_dest gin_trgm_ops);
CREATE INDEX IF NOT EXISTS case_memory_summary_trgm_idx ON case_memory USING GIN (summary gin_trgm_ops);
