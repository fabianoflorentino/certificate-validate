// Package history provides certificate check history recording and querying.
//
// The package maintains a JSONL (JSON Lines) file that records certificate check
// results over time. Each entry contains the host, days left, and timestamp.
// The history file is automatically rotated based on configurable limits for
// maximum entries and maximum age.
//
// # Store Interface
//
// The Store interface defines the contract for history operations:
//
//   - Record: appends certificate check results to the history file
//   - GetHistory: retrieves history entries for a specific host
//
// Consumers depend on the Store interface, enabling alternative implementations
// (e.g., in-memory for testing).
//
// # Rotation
//
// The history file is rotated when either limit is exceeded:
//
//   - MaxEntries: maximum number of entries (default: 10000)
//   - MaxDays: maximum age of entries in days (default: 90)
//
// Rotation preserves recent entries and discards older ones.
//
// # Usage
//
// Create a recorder and record results:
//
//	rec := history.New(history.Config{
//		FilePath:   "data/history.jsonl",
//		MaxEntries: 10000,
//		MaxDays:    90,
//	})
//	rec.Record(certs)
//
// Query history for a host:
//
//	entries, err := rec.GetHistory("example.com")
package history
