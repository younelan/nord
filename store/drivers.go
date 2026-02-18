package store

// Import database drivers as side effects so they register themselves
// with database/sql. All three are pure Go — no CGO required.
import (
	_ "github.com/go-sql-driver/mysql" // mysql://
	_ "github.com/lib/pq"              // postgres://
	_ "modernc.org/sqlite"             // sqlite://
)
