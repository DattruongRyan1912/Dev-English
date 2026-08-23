CREATE TABLE migration_failure_probe (id INTEGER PRIMARY KEY);
INSERT INTO migration_failure_probe (id) VALUES (1);
SELECT 1 / 0;
INSERT INTO schema_migrations (version) VALUES ('003_failure.sql');
