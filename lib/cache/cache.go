package cache

type Store interface {
	// Get creates or returns an existing location where data corresponding to key is stored.
	//
	// The first value returned is a path, the path where data is to be stored, or has been stored before.
	// The second value returned is true if the path was existing already, false otherwise.
	// An error is returned if for some reason the key could not be created or looked up.
	//
	// If the location returned by Get was newly created, this location must either be Purged,
	// Rolled back, or Committed, otherwise your code will leak cache keys.
	//
	// If a location was already existing, data should be accessed in a read only way, unless
	// the application is capable of handling concurrent writes on its own data.
	//
	// Both Commit and Rollback can safely be called on an existing location, in which case
	// nothing will be done. This is convenient to use with defer.
	//
	// Purge can be called on existing as well as new locations, and will result in the data
	// being forever deleted.
	Get(key string) (string, bool, error)

	// Exists check if there is an entry in the cache corresponding to key.
	//
	// If there is, it returns the path on disk. If there is not, it returns the empty string.
	// Error is returned if the location could not be found.
	Exists(key string) (string, error)

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
