package cache

type Store interface {
	Exists(key string) (bool, error)
	Get(key string) (string, bool, error)

	// Commit ensures that the changes made to the cache at location are saved.
	//
	// Calling Commit multiple times on the same location, committed or not, is a noop.
	//
	// If multiple Get() calls for the same key are performed, and all are Committed in parallel,
	// only one of the commits will be saved, which one is undetermined.
	Commit(location string) (string, error)

	// Purge will remove the cache entry at location.
	// Purge works on both committed and uncommitted locations.
	//
	// Use Purge when either a cache key has to be removed, or when an uncommitted key needs to be removed.
	Purge(location string) error

	// Rollback will leave a committed location in place, but will purge a location that has not been committed yet.
	// If the location has been committed, Rollback is a noop.
	//
	// Use Rollback when a cache key was retrieved with Get, an error occurred, and you want to leave an
	// old cache entry in place (if it exists) or delete the entry if nothing was ever committed.
	Rollback(location string) error
}
