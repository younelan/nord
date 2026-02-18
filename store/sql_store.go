package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
)

type sqlStore struct {
	db        *sql.DB
	d         dialect
	mu        sync.Mutex
	hostCache map[string]int64 // key → id, populated on first write per run
}

func openSQL(driver, dsn string, d dialect) (Store, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", driver, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: connect %s: %w", driver, err)
	}

	s := &sqlStore{db: db, d: d, hostCache: make(map[string]int64)}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// migrate creates the schema_migrations tracker table, then applies any
// pending migrations in version order. Each migration runs in its own
// transaction so a failure leaves previous migrations intact.
func (s *sqlStore) migrate() error {
	// Always create the tracker table first — it is not itself versioned.
	if _, err := s.db.Exec(schemaMigrationsDDL(s.d)); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}

	// Load already-applied versions.
	applied := make(map[int]bool)
	rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("store: query schema_migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("store: scan schema_migrations: %w", err)
		}
		applied[v] = true
	}
	rows.Close()

	// Apply pending migrations.
	for _, m := range migrations(s.d) {
		if applied[m.version] {
			continue
		}
		if err := s.applyMigration(m); err != nil {
			return fmt.Errorf("store: migration v%d %q: %w", m.version, m.description, err)
		}
		fmt.Printf("  |_ store: applied migration v%d: %s\n", m.version, m.description)
	}
	return nil
}

// applyMigration runs a single migration in a transaction.
// If the schema already exists (upgrading from a pre-versioning binary),
// v1 is stamped as applied without re-running its DDL so we don't hit
// MySQL's lack of CREATE INDEX IF NOT EXISTS.
func (s *sqlStore) applyMigration(m migration) error {
	if m.version == 1 && s.schemaExists() {
		return s.recordMigration(m.version, m.description)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, stmt := range m.up {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("statement failed: %w\nSQL: %s", err, stmt)
		}
	}
	if err := s.recordMigrationTx(tx, m.version, m.description); err != nil {
		return err
	}
	return tx.Commit()
}

// schemaExists returns true when the hosts table is already present,
// meaning the database was initialised before migration tracking was added.
func (s *sqlStore) schemaExists() bool {
	var exists bool
	var err error
	switch s.d {
	case dialectPostgres:
		err = s.db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='hosts')`,
		).Scan(&exists)
	case dialectMySQL:
		// SHOW TABLES LIKE returns a row when the table exists.
		var name string
		err = s.db.QueryRow(`SHOW TABLES LIKE 'hosts'`).Scan(&name)
		exists = err == nil
		return exists
	default: // SQLite
		err = s.db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='hosts')`,
		).Scan(&exists)
	}
	return err == nil && exists
}

func (s *sqlStore) recordMigration(version int, description string) error {
	q, args := s.insertMigrationSQL(version, description)
	_, err := s.db.Exec(q, args...)
	return err
}

func (s *sqlStore) recordMigrationTx(tx interface{ Exec(string, ...interface{}) (sql.Result, error) }, version int, description string) error {
	q, args := s.insertMigrationSQL(version, description)
	_, err := tx.Exec(q, args...)
	return err
}

func (s *sqlStore) insertMigrationSQL(version int, description string) (string, []interface{}) {
	if s.d == dialectPostgres {
		return `INSERT INTO schema_migrations (version, description) VALUES ($1, $2)`,
			[]interface{}{version, description}
	}
	return `INSERT INTO schema_migrations (version, description) VALUES (?, ?)`,
		[]interface{}{version, description}
}

// ph returns the positional placeholder for the nth argument (1-based).
// PostgreSQL uses $1, $2, …; everything else uses ?.
func (s *sqlStore) ph(n int) string {
	if s.d == dialectPostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// ensureHost upserts a host row and returns its id.
// Results are cached in hostCache so each key hits the DB at most once per WriteBatch call.
func (s *sqlStore) ensureHost(key, name, address string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.hostCache[key]; ok {
		return id, nil
	}

	var upsertQ, selectQ string
	switch s.d {
	case dialectPostgres:
		upsertQ = `INSERT INTO hosts (key, name, address)
			VALUES ($1, $2, $3)
			ON CONFLICT (key) DO UPDATE
			SET name=EXCLUDED.name, address=EXCLUDED.address, last_seen=NOW()`
		selectQ = `SELECT id FROM hosts WHERE key = $1`

	case dialectMySQL:
		upsertQ = "INSERT INTO hosts (`key`, name, address, first_seen, last_seen) " +
			"VALUES (?, ?, ?, NOW(), NOW()) " +
			"ON DUPLICATE KEY UPDATE name=VALUES(name), address=VALUES(address), last_seen=NOW()"
		selectQ = "SELECT id FROM hosts WHERE `key` = ?"

	default: // SQLite
		upsertQ = `INSERT INTO hosts (key, name, address)
			VALUES (?, ?, ?)
			ON CONFLICT(key) DO UPDATE
			SET name=excluded.name, address=excluded.address,
			    last_seen=CURRENT_TIMESTAMP`
		selectQ = `SELECT id FROM hosts WHERE key = ?`
	}

	if _, err := s.db.Exec(upsertQ, key, name, address); err != nil {
		return 0, fmt.Errorf("store: upsert host %q: %w", key, err)
	}

	var id int64
	if err := s.db.QueryRow(selectQ, key).Scan(&id); err != nil {
		return 0, fmt.Errorf("store: query host id %q: %w", key, err)
	}

	s.hostCache[key] = id
	return id, nil
}

// WriteBatch persists a slice of metric records in a single transaction.
func (s *sqlStore) WriteBatch(records []MetricRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Resolve all host IDs before opening the transaction.
	hostIDs := make(map[string]int64, len(records))
	for _, r := range records {
		if _, ok := hostIDs[r.HostKey]; ok {
			continue
		}
		id, err := s.ensureHost(r.HostKey, r.HostName, r.HostAddress)
		if err != nil {
			fmt.Printf("  !_ store: skip host %q: %v\n", r.HostKey, err)
			continue
		}
		hostIDs[r.HostKey] = id
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var insertQ string
	if s.d == dialectPostgres {
		insertQ = "INSERT INTO metrics " +
			"(host_id, plugin, name, category, metric_type, value, value_num, instance, extra, collected_at) " +
			"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"
	} else {
		insertQ = "INSERT INTO metrics " +
			"(host_id, plugin, name, category, metric_type, value, value_num, instance, extra, collected_at) " +
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	}

	stmt, err := tx.Prepare(insertQ)
	if err != nil {
		return fmt.Errorf("store: prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, r := range records {
		hostID, ok := hostIDs[r.HostKey]
		if !ok {
			continue
		}
		var instance interface{} = nil
		if r.Instance != "" {
			instance = r.Instance
		}
		if _, err := stmt.Exec(
			hostID, r.Plugin, r.Name, r.Category, r.MetricType,
			r.Value, r.ValueNum, instance, marshalExtra(r.Extra), r.CollectedAt,
		); err != nil {
			fmt.Printf("  !_ store: insert %q/%q: %v\n", r.HostKey, r.Name, err)
		}
	}

	return tx.Commit()
}

// UpsertInterfaces upserts interface entity records — one row per (host, ifIndex).
// Static fields (name, type, speed, MAC) are updated on every call; first_seen is preserved.
func (s *sqlStore) UpsertInterfaces(records []InterfaceRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Resolve host IDs.
	hostIDs := make(map[string]int64, len(records))
	for _, r := range records {
		if _, ok := hostIDs[r.HostKey]; ok {
			continue
		}
		id, err := s.ensureHost(r.HostKey, r.HostName, r.HostAddress)
		if err != nil {
			fmt.Printf("  !_ store: skip host %q (interfaces): %v\n", r.HostKey, err)
			continue
		}
		hostIDs[r.HostKey] = id
	}

	var upsertQ string
	switch s.d {
	case dialectPostgres:
		upsertQ = `INSERT INTO interfaces
			(host_id, if_index, name, alias, type, speed, mac_address, admin_status, oper_status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (host_id, if_index) DO UPDATE SET
				name=EXCLUDED.name, alias=EXCLUDED.alias, type=EXCLUDED.type,
				speed=EXCLUDED.speed, mac_address=EXCLUDED.mac_address,
				admin_status=EXCLUDED.admin_status, oper_status=EXCLUDED.oper_status,
				last_seen=NOW()`
	case dialectMySQL:
		upsertQ = "INSERT INTO interfaces " +
			"(host_id, if_index, name, alias, type, speed, mac_address, admin_status, oper_status, last_seen) " +
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW()) " +
			"ON DUPLICATE KEY UPDATE " +
			"name=VALUES(name), alias=VALUES(alias), type=VALUES(type), speed=VALUES(speed), " +
			"mac_address=VALUES(mac_address), admin_status=VALUES(admin_status), " +
			"oper_status=VALUES(oper_status), last_seen=NOW()"
	default: // SQLite
		upsertQ = `INSERT INTO interfaces
			(host_id, if_index, name, alias, type, speed, mac_address, admin_status, oper_status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(host_id, if_index) DO UPDATE SET
				name=excluded.name, alias=excluded.alias, type=excluded.type,
				speed=excluded.speed, mac_address=excluded.mac_address,
				admin_status=excluded.admin_status, oper_status=excluded.oper_status,
				last_seen=CURRENT_TIMESTAMP`
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin tx (interfaces): %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(upsertQ)
	if err != nil {
		return fmt.Errorf("store: prepare interface upsert: %w", err)
	}
	defer stmt.Close()

	for _, r := range records {
		hostID, ok := hostIDs[r.HostKey]
		if !ok {
			continue
		}
		var speed interface{} = nil
		if r.Speed != nil {
			speed = *r.Speed
		}
		args := []interface{}{hostID, r.IfIndex, r.Name, r.Alias, r.Type, speed, r.MACAddress, r.AdminStatus, r.OperStatus}
		if s.d == dialectMySQL {
			// MySQL upsert includes last_seen=NOW() as a literal — no extra arg needed.
		}
		if _, err := stmt.Exec(args...); err != nil {
			fmt.Printf("  !_ store: upsert interface %q idx %d: %v\n", r.HostKey, r.IfIndex, err)
		}
	}

	return tx.Commit()
}

// marshalExtra serialises the Extra map to a JSON string for storage.
// Returns nil (SQL NULL) when the map is empty.
func marshalExtra(extra map[string]interface{}) interface{} {
	if len(extra) == 0 {
		return nil
	}
	b, err := json.Marshal(extra)
	if err != nil {
		return nil
	}
	return string(b)
}

func (s *sqlStore) Close() error {
	return s.db.Close()
}
