/* Uncomment this line to drop + recreate the table. This will generate new UUIDs
 * for each license instance, which could be disruptive. */
-- DROP TABLE IF EXISTS license_state;
CREATE TABLE IF NOT EXISTS license_state (
  /* This should be an opaque ID, but is stably generated from:
   * - the vendor
   * - the feature
   * - an "index"/license number, if there are multiple
   * ...and then hashed into a 64-bit number via fnv-1a and base64-encoded.
   * Creating such an ID will reduce the need to log tuples of (vendor, feature,
   * license#) everywhere - we can use a single ID instead. Avoiding purely
   * random IDs like UUIDs could bring about a situation where all the IDs need
   * to change when clients still remember old IDs.
   */
  id TEXT NOT NULL,
  vendor TEXT NOT NULL,
  feature TEXT NOT NULL,
  usage_state TEXT NOT NULL,
  last_state_change TIMESTAMPTZ NOT NULL,
  /* Null iff usage_state is not FREE; indicates which node has made the reservation */
  reserved_by_node TEXT,
  /* The process/job ID of the current license user. This may only be available
   * when the license is actually in use, not when its reserved.
   */
  used_by_process TEXT,
  /* Escape hatch for additional info that can be added later to aid debugging.
   * These elements should be for human eyes only; if anything programmatic
   * depends on them, they should be promoted into their own fields.
   */
  metadata JSONB,
  PRIMARY KEY (id)
);

-- DROP TABLE IF EXISTS license_state_log
CREATE TABLE IF NOT EXISTS license_state_log (
  id BIGSERIAL PRIMARY KEY,
  license_id VARCHAR(16) NOT NULL REFERENCES license_state(id),
  node TEXT NOT NULL,
  ts TIMESTAMPTZ NOT NULL,
  previous_state TEXT NOT NULL,
  current_state TEXT NOT NULL,
  reason TEXT NOT NULL,
  metadata JSONB
);
