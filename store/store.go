package store

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MetricRecord holds a single metric sample to persist.
type MetricRecord struct {
	HostKey     string
	HostName    string
	HostAddress string
	Plugin      string
	Name        string
	Category    string
	MetricType  string
	Value       string
	ValueNum    *float64
	Instance    string                 // which interface/CPU/disk/etc. — empty for scalar metrics
	Extra       map[string]interface{} // optional plugin-specific metadata (OID, …) stored as JSON
	CollectedAt time.Time
}

// InterfaceRecord holds entity-level data for a network interface.
// This is slowly-changing metadata (name, type, speed, MAC) as opposed to
// per-poll time-series counters which go into MetricRecord.
type InterfaceRecord struct {
	HostKey     string
	HostName    string
	HostAddress string
	IfIndex     int
	Name        string // ifDescr
	Alias       string // ifAlias (may be empty)
	Type        int    // ifType integer (6=ethernet, 24=loopback, …)
	Speed       *int64 // ifSpeed in bps; nil when unknown
	MACAddress  string // formatted xx:xx:xx:xx:xx:xx
	AdminStatus string // "up", "down", "testing"
	OperStatus  string // "up", "down", "testing", "unknown", "dormant", "notPresent", "lowerLayerDown"
}

// Store is the abstraction for persisting collected metrics.
// Implementations must be safe for concurrent use.
type Store interface {
	WriteBatch(records []MetricRecord) error
	UpsertInterfaces(records []InterfaceRecord) error
	Close() error
}

// Open returns a Store for the given connection URL.
//
// Supported schemes:
//
//	sqlite://data/nord.db
//	mysql://user:pass@host:3306/dbname
//	postgres://user:pass@host:5432/dbname
//
// Returns nil, nil when rawURL is empty — callers skip writes safely.
func Open(rawURL string) (Store, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("store: invalid URL %q: %w", rawURL, err)
	}

	// Apply default ports before dispatching.
	applyDefaultPort(u, map[string]string{
		"mysql":      "3306",
		"postgres":   "5432",
		"postgresql": "5432",
	})

	switch strings.ToLower(u.Scheme) {
	case "sqlite", "sqlite3":
		// sqlite://data/nord.db  → host="data" path="/nord.db" → "data/nord.db"
		// sqlite:///tmp/nord.db  → host=""     path="/tmp/..."  → "/tmp/nord.db"
		path := u.Host + u.Path
		if path == "" || path == "/" {
			path = ":memory:"
		}
		return openSQL("sqlite", path, dialectSQLite)

	case "mysql":
		return openSQL("mysql", toMySQLDSN(u), dialectMySQL)

	case "postgres", "postgresql":
		// lib/pq accepts the postgres:// URL directly; rebuild with defaulted port.
		return openSQL("postgres", u.String(), dialectPostgres)

	default:
		return nil, fmt.Errorf("store: unsupported scheme %q (supported: sqlite, mysql, postgres)", u.Scheme)
	}
}

// applyDefaultPort sets the port on u when none is present and the scheme
// has a known default. The defaults map is keyed by scheme.
func applyDefaultPort(u *url.URL, defaults map[string]string) {
	scheme := strings.ToLower(u.Scheme)
	def, ok := defaults[scheme]
	if !ok {
		return
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		u.Host = host + ":" + def
	}
}

// toMySQLDSN converts a mysql:// URL to the go-sql-driver/mysql DSN format:
// user:pass@tcp(host:port)/dbname?params
// The port is already defaulted by applyDefaultPort before this is called.
func toMySQLDSN(u *url.URL) string {
	host := u.Host

	var creds string
	if u.User != nil {
		creds = u.User.String() + "@"
	}

	dbname := strings.TrimPrefix(u.Path, "/")
	dsn := fmt.Sprintf("%stcp(%s)/%s", creds, host, dbname)
	if u.RawQuery != "" {
		dsn += "?" + u.RawQuery
	}
	return dsn
}

// ParseValueNum attempts to extract a numeric representation of a string metric value.
// Returns nil when the value cannot be meaningfully expressed as a number.
func ParseValueNum(value string) *float64 {
	v := strings.ToLower(strings.TrimSpace(value))

	// status strings → 1 / 0.5 / 0
	switch v {
	case "up", "ok", "running", "active", "online", "reachable", "open":
		f := 1.0
		return &f
	case "down", "critical", "error", "offline", "inactive", "unreachable", "closed":
		f := 0.0
		return &f
	case "warning", "degraded", "paused":
		f := 0.5
		return &f
	}

	// percentage: "9%" → 9.0
	if strings.HasSuffix(v, "%") {
		if n, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64); err == nil {
			return &n
		}
	}

	// plain number: "1024", "3.14"
	if n, err := strconv.ParseFloat(v, 64); err == nil {
		return &n
	}

	// uptime/duration: "2d 3h 0m 4s" → total seconds
	if secs, ok := parseDuration(v); ok {
		f := float64(secs)
		return &f
	}

	return nil
}

var durationRe = regexp.MustCompile(`(\d+)\s*([dhms])`)

func parseDuration(s string) (int64, bool) {
	matches := durationRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return 0, false
	}
	var total int64
	for _, m := range matches {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		switch m[2] {
		case "d":
			total += n * 86400
		case "h":
			total += n * 3600
		case "m":
			total += n * 60
		case "s":
			total += n
		}
	}
	return total, true
}
