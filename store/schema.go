package store

type dialect int

const (
	dialectSQLite   dialect = iota
	dialectPostgres
	dialectMySQL
)

// migration represents a single, ordered schema change.
// Migrations are applied exactly once and recorded in schema_migrations.
// To add a new migration: append an entry to the migrations() slice.
type migration struct {
	version     int
	description string
	up          []string // SQL statements to apply, run in a single transaction
}

// migrations returns all migrations in ascending version order for the given dialect.
// New migrations must be appended at the end — never inserted or renumbered.
func migrations(d dialect) []migration {
	return []migration{
		{
			version:     1,
			description: "initial schema: hosts and metrics tables",
			up:          v1Schema(d),
		},
		{
			version:     2,
			description: "add extra JSON column to metrics",
			up:          v2Schema(d),
		},
		{
			version:     3,
			description: "add instance column to metrics; add interfaces entity table",
			up:          v3Schema(d),
		},
		// Append future migrations here, e.g.:
		// {
		//     version:     3,
		//     description: "add category index",
		//     up: []string{
		//         `CREATE INDEX idx_metrics_category ON metrics (category, collected_at DESC)`,
		//     },
		// },
	}
}

// schemaMigrationsDDL returns the CREATE TABLE for the migrations tracker.
// This table is created before any migration runs — it is not itself versioned.
func schemaMigrationsDDL(d dialect) string {
	switch d {
	case dialectPostgres:
		return `CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER PRIMARY KEY,
			description TEXT    NOT NULL DEFAULT '',
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`
	case dialectMySQL:
		// MySQL does not allow DEFAULT values on TEXT/BLOB columns — use VARCHAR instead.
		return `CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER PRIMARY KEY,
			description VARCHAR(255) NOT NULL DEFAULT '',
			applied_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`
	default: // SQLite
		return `CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER PRIMARY KEY,
			description TEXT    NOT NULL DEFAULT '',
			applied_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`
	}
}

// v1Schema returns the initial DDL for the given dialect.
// Rules:
//   - CREATE TABLE uses IF NOT EXISTS (idempotent across dialects)
//   - CREATE INDEX omits IF NOT EXISTS — MySQL <8.0.12 doesn't support it.
//     The migration runner detects an existing schema and skips to avoid re-running.
func v1Schema(d dialect) []string {
	switch d {
	case dialectPostgres:
		return []string{
			`CREATE TABLE IF NOT EXISTS hosts (
				id         BIGSERIAL PRIMARY KEY,
				key        TEXT UNIQUE NOT NULL,
				name       TEXT NOT NULL DEFAULT '',
				address    TEXT NOT NULL DEFAULT '',
				first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				last_seen  TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS metrics (
				id           BIGSERIAL PRIMARY KEY,
				host_id      BIGINT NOT NULL REFERENCES hosts(id),
				plugin       TEXT NOT NULL DEFAULT '',
				name         TEXT NOT NULL DEFAULT '',
				category     TEXT NOT NULL DEFAULT '',
				metric_type  TEXT NOT NULL DEFAULT '',
				value        TEXT NOT NULL DEFAULT '',
				value_num    DOUBLE PRECISION,
				collected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX idx_metrics_host_time ON metrics (host_id, collected_at DESC)`,
			`CREATE INDEX idx_metrics_host_name ON metrics (host_id, plugin, name, collected_at DESC)`,
		}

	case dialectMySQL:
		// Notes:
		//   - `key` is a reserved word — must be back-tick quoted in DDL and queries.
		//   - FOREIGN KEY requires InnoDB (default in MySQL 5.5+).
		//   - Index prefix lengths are required for TEXT columns (max key length 767/3072 bytes).
		//   - No TIMESTAMPTZ; DATETIME stores without timezone. Use UTC at the application layer.
		return []string{
			"CREATE TABLE IF NOT EXISTS hosts (" +
				"  id         BIGINT AUTO_INCREMENT PRIMARY KEY," +
				"  `key`      VARCHAR(255) UNIQUE NOT NULL," +
				"  name       VARCHAR(255) NOT NULL DEFAULT ''," +
				"  address    VARCHAR(255) NOT NULL DEFAULT ''," +
				"  first_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP," +
				"  last_seen  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
			"CREATE TABLE IF NOT EXISTS metrics (" +
				"  id           BIGINT AUTO_INCREMENT PRIMARY KEY," +
				"  host_id      BIGINT NOT NULL," +
				"  plugin       VARCHAR(100) NOT NULL DEFAULT ''," +
				"  name         VARCHAR(255) NOT NULL DEFAULT ''," +
				"  category     VARCHAR(100) NOT NULL DEFAULT ''," +
				"  metric_type  VARCHAR(50)  NOT NULL DEFAULT ''," +
				"  value        TEXT         NOT NULL," +
				"  value_num    DOUBLE," +
				"  collected_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP," +
				"  CONSTRAINT fk_metrics_host FOREIGN KEY (host_id) REFERENCES hosts(id)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
			// No IF NOT EXISTS on indexes — not supported before MySQL 8.0.12.
			// The migration runner checks for an existing schema before applying v1
			// so these only run on a fresh database.
			"CREATE INDEX idx_metrics_host_time ON metrics (host_id, collected_at)",
			"CREATE INDEX idx_metrics_host_name ON metrics (host_id, plugin, name(100), collected_at)",
		}

	default: // SQLite
		return []string{
			`CREATE TABLE IF NOT EXISTS hosts (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				key        TEXT UNIQUE NOT NULL,
				name       TEXT NOT NULL DEFAULT '',
				address    TEXT NOT NULL DEFAULT '',
				first_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				last_seen  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS metrics (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				host_id      INTEGER NOT NULL REFERENCES hosts(id),
				plugin       TEXT NOT NULL DEFAULT '',
				name         TEXT NOT NULL DEFAULT '',
				category     TEXT NOT NULL DEFAULT '',
				metric_type  TEXT NOT NULL DEFAULT '',
				value        TEXT NOT NULL DEFAULT '',
				value_num    REAL,
				collected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX idx_metrics_host_time ON metrics (host_id, collected_at DESC)`,
			`CREATE INDEX idx_metrics_host_name ON metrics (host_id, plugin, name, collected_at DESC)`,
		}
	}
}

// v2Schema adds the extra column to metrics for plugin-specific metadata (e.g. OID, port).
// ALTER TABLE ADD COLUMN is safe across all three dialects when the column is nullable.
func v2Schema(d dialect) []string {
	switch d {
	case dialectPostgres:
		return []string{`ALTER TABLE metrics ADD COLUMN extra JSONB`}
	case dialectMySQL:
		return []string{`ALTER TABLE metrics ADD COLUMN extra JSON`}
	default: // SQLite
		return []string{`ALTER TABLE metrics ADD COLUMN extra TEXT`}
	}
}
// v3Schema adds the instance column to metrics and creates the interfaces entity table.
// instance identifies which interface/CPU/disk/etc. a metric belongs to (NULL for scalars).
// interfaces stores slowly-changing entity metadata discovered via SNMP table walks.
func v3Schema(d dialect) []string {
	switch d {
	case dialectPostgres:
		return []string{
			`ALTER TABLE metrics ADD COLUMN instance TEXT`,
			`CREATE TABLE IF NOT EXISTS interfaces (
				id           BIGSERIAL PRIMARY KEY,
				host_id      BIGINT NOT NULL REFERENCES hosts(id),
				if_index     INTEGER NOT NULL,
				name         TEXT NOT NULL DEFAULT '',
				alias        TEXT NOT NULL DEFAULT '',
				type         INTEGER NOT NULL DEFAULT 0,
				speed        BIGINT,
				mac_address  TEXT NOT NULL DEFAULT '',
				admin_status TEXT NOT NULL DEFAULT '',
				oper_status  TEXT NOT NULL DEFAULT '',
				first_seen   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				last_seen    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(host_id, if_index)
			)`,
			`CREATE INDEX idx_interfaces_host ON interfaces (host_id)`,
		}
	case dialectMySQL:
		return []string{
			`ALTER TABLE metrics ADD COLUMN instance VARCHAR(255)`,
			"CREATE TABLE IF NOT EXISTS interfaces (" +
				"  id           BIGINT AUTO_INCREMENT PRIMARY KEY," +
				"  host_id      BIGINT NOT NULL," +
				"  if_index     INT NOT NULL," +
				"  name         VARCHAR(255) NOT NULL DEFAULT ''," +
				"  alias        VARCHAR(255) NOT NULL DEFAULT ''," +
				"  type         INT NOT NULL DEFAULT 0," +
				"  speed        BIGINT," +
				"  mac_address  VARCHAR(17) NOT NULL DEFAULT ''," +
				"  admin_status VARCHAR(20) NOT NULL DEFAULT ''," +
				"  oper_status  VARCHAR(20) NOT NULL DEFAULT ''," +
				"  first_seen   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP," +
				"  last_seen    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP," +
				"  CONSTRAINT fk_interfaces_host FOREIGN KEY (host_id) REFERENCES hosts(id)," +
				"  UNIQUE KEY uk_interfaces_host_index (host_id, if_index)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
			"CREATE INDEX idx_interfaces_host ON interfaces (host_id)",
		}
	default: // SQLite
		return []string{
			`ALTER TABLE metrics ADD COLUMN instance TEXT`,
			`CREATE TABLE IF NOT EXISTS interfaces (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				host_id      INTEGER NOT NULL REFERENCES hosts(id),
				if_index     INTEGER NOT NULL,
				name         TEXT NOT NULL DEFAULT '',
				alias        TEXT NOT NULL DEFAULT '',
				type         INTEGER NOT NULL DEFAULT 0,
				speed        INTEGER,
				mac_address  TEXT NOT NULL DEFAULT '',
				admin_status TEXT NOT NULL DEFAULT '',
				oper_status  TEXT NOT NULL DEFAULT '',
				first_seen   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				last_seen    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(host_id, if_index)
			)`,
			`CREATE INDEX idx_interfaces_host ON interfaces (host_id)`,
		}
	}
}
