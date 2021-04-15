#ifndef NSS_AUTOUSER_H
# define NSS_AUTOUSER_H

typedef enum {
  // The path of the home directory is the full path, no need to append /$USER.
  MATCH_USE_FULL_HOME = 1<<0,
  MATCH_SET_FULL_HOME = 1<<1,

  // If a user is found on the system already, keep the password configured on
  // the system rather than disabling it.
  MATCH_USE_PASSWORD = 1<<4,
  MATCH_SET_PASSWORD = 1<<5,
} match_flag_e;

typedef struct autouser_config_s autouser_config_t;

typedef struct {
  const char* argv;
  const char* suffix;

  const char* shell;
  const char* home;
  const char* gecos;

  uid_t min_uid;
  uid_t max_uid;
  gid_t gid;

  uint32_t flags;
} autouser_match_t;

struct autouser_config_s {
  const char* seed;
  const char* debug;

  autouser_match_t *match;
  size_t matchn;
};

#endif
