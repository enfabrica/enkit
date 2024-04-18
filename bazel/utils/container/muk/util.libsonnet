{
  // AstoreFile returns an astore_file, inferring filename via the last
  // component of astore_path.
  AstoreFile:: function(uid, astore_path) {
    filename: std.splitLimitR(astore_path, '/', 1)[1],
    astore_path: astore_path,
    uid: uid,
  },

  // Command shortens the spelling of command actions somewhat.
  Command:: function(cmd) {
    command: {
      command: cmd,
    },
  },
}
