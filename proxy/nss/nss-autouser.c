#include <nss.h>
#include <pwd.h>
#include <shadow.h>
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <stddef.h>
#include <stdint.h>
#include <sys/types.h>
#include <pwd.h>
#include <errno.h>
#include <inttypes.h>
#include <assert.h>
#include <syslog.h>
#include <stdarg.h>
#include <sys/types.h>
#include <unistd.h>
#include <threads.h>
#include <stdbool.h>
#include <fnmatch.h>

#include "proxy/nss/confparse/confparse.h"
#include "proxy/nss/nss-autouser.h"

#ifndef AU_CONFIG_PATH
# define AU_CONFIG_PATH "/etc/nss-autouser.conf"
#endif

#ifndef AU_HASH_ATTEMPTS
# define AU_HASH_ATTEMPTS 10
#endif

#ifndef AU_LOG_BUFFER_SIZE
# define AU_LOG_BUFFER_SIZE 512
#endif

#ifndef AU_DEFAULT_SHELL
# define AU_DEFAULT_SHELL "/bin/bash"
#endif

static char** process_argv = NULL;
static char** process_env = NULL;
static int process_argc = 0;

/* vdebug saves a log line in a debug file in a way that's safe
 * even if multiple processes are accessing the file. */
static void vdebug(const char* debug, const char* fmt, va_list ap) {
  if (!debug) return;

  FILE *f = fopen(debug, "a");
  if (!f) return;
  /* Line buffering to prevent corruption with multiple
   * multiple instances of the code running. */
  setvbuf(f, NULL, _IOLBF, 0);

  vfprintf(f, fmt, ap);
  fclose(f);
}

__attribute__ ((format (printf, 2, 3)))
static void debug(const char* debug, const char* fmt, ...) {
  va_list ap;
  va_start(ap, fmt);
  vdebug(debug, fmt, ap);
  va_end(ap);
}

/* mlog will save all messages to a debug log file, if one is passed,
 * while also sending messages of LOG_INFO or above to syslog. */
__attribute__ ((format (printf, 3, 4)))
static void mlog(const char* path, int priority, const char* fmt, ...) {
  if (!path && priority > LOG_INFO) return;

  size_t bsize = AU_LOG_BUFFER_SIZE;
  char bdata[bsize];
  char* buffer = bdata;
  va_list ap;

  int len = snprintf(buffer, bsize, "nss-autouser for pid %d (%s) - ",
		     getpid(), process_argv ? process_argv[0] : "unknown");
  if (len < bsize) {
    buffer += len;
    bsize -= len;
  }

  va_start(ap, fmt);
  vsnprintf(buffer, bsize, fmt, ap);
  va_end(ap);

  bdata[sizeof(bdata) - 1] = '\0';

  va_start(ap, fmt);
  debug(path, "%s\n", bdata); 
  va_end(ap);

  if (priority <= LOG_INFO)
    syslog(priority, "%s", bdata);
}

static void config_free(autouser_config_t* config) {
  if (!config) return;
  free((void*)config->seed);
  for (int i = 0; i < config->matchn; ++i) {
    autouser_match_t* match = &config->match[i];
    free((void*)match->argv);
    free((void*)match->suffix);
    free((void*)match->shell);
    free((void*)match->home);
    free((void*)match->gecos);
  }
  free(config->match);
  memset(config, 0, sizeof(*config));
}

void *add_autouser_match(autouser_config_t* config) {
  config->match = (autouser_match_t*)(realloc(config->match, (config->matchn + 1) * sizeof(autouser_match_t)));
  autouser_match_t* entry = &config->match[config->matchn];
  memset(entry, 0, sizeof(*entry));
  if (config->matchn >= 1 && config->match[config->matchn - 1].argv)
    entry->argv = strdup(config->match[config->matchn - 1].argv);

  ++(config->matchn);
  return entry;
}

/* config_parse parses a libnss_autouser configuration file. */
static int config_parse(const char* path, autouser_config_t* config) {
  perror_t err = perror_init();

  statement_t suffix[] = {
	  { OPT_START, match_exact("Suffix"), expect_string(offsetof(autouser_match_t, suffix))},
    	  { OPT_NONE, match_exact("Shell"), expect_string(offsetof(autouser_match_t, shell))},
    	  { OPT_NONE, match_exact("Home"), expect_string(offsetof(autouser_match_t, home))},
    	  { OPT_NONE, match_exact("Gecos"), expect_string(offsetof(autouser_match_t, gecos))},

    	  { OPT_NONE, match_exact("MinUid"), expect_uint32(offsetof(autouser_match_t, min_uid))},
    	  { OPT_NONE, match_exact("MaxUid"), expect_uint32(offsetof(autouser_match_t, max_uid))},
    	  { OPT_NONE, match_exact("Gid"), expect_uint32(offsetof(autouser_match_t, gid))},

    	  { OPT_NONE, match_exact("PropagatePassword"), expect_bool32(offsetof(autouser_match_t, flags), MATCH_SET_PASSWORD, MATCH_USE_PASSWORD)},
    	  { OPT_NONE, match_exact("FullHomePath"), expect_bool32(offsetof(autouser_match_t, flags), MATCH_SET_FULL_HOME, MATCH_USE_FULL_HOME)},
    STATEMENTS_END,
  };

  statement_t match[] = {
	  { OPT_START, match_exact("Match"), expect_string(offsetof(autouser_match_t, argv))},
    	  { OPT_NONE, match_any(), expect_section(suffix, NULL)},
    STATEMENTS_END,
  };

  statement_t root[] = {
	  { OPT_NONE, match_exact("Seed"), expect_string(offsetof(autouser_config_t, seed))},
	  { OPT_NONE, match_exact("DebugLog"), expect_string(offsetof(autouser_config_t, debug))},
    	  { OPT_MULTI, match_any(), expect_section(match, (adder_f)(add_autouser_match))},
    STATEMENTS_END,
  };

  int status = parse_file(path, root, config, &err);
  if (status < 0) {
    mlog(NULL, LOG_ERR, "error %d parsing configuration file '%s': %s", status, path, err.message);
  }

  perror_free(&err);
  return status;
}


/* add appends strings to a buffer, for as long as the string won't
 * exceed the end of the buffer.
 *
 * Returns the address of the string within the buffer, or NULL
 * if for any reason the source str could not be copied.
 *
 * If the buffer fills, the destination pointer is set to NULL,
 * to force future addition to fail. */
static char* add(char** dest, const char* end, const char* str) {
  char* start = *dest;
  if (!start) return NULL;
  if (!str) return NULL;

  size_t len = strlen(str);
  if (start + len >= end) {
    (*dest) = NULL;
    return NULL;
  }

  memcpy(start, str, len + 1);
  (*dest) += len + 1;
  return start;
}

/* Simple FNV-1a implementation.
 *
 * A cryptographically secure hash would be preferrable here, but
 * with < 32 bits of space and the birthday paradox, finding
 * collisions would still be relatively easy, and the impact
 * of clashes is low (user still needs to authenticate, users
 * cannot pick an arbitrary number of usernames, ...). */
static uint64_t seed(const char* seed) {
  uint64_t hash = 0xcbf29ce484222325ULL;
  for (; *seed; ++seed) {
    hash ^= *(unsigned char*)(seed);
    hash *= 0x100000001b3ULL;
  }
  return hash;
}

static uint64_t hash(uint64_t seed, const char* data) {
  for (; *data; ++data) {
    seed ^= *(unsigned char*)(data);
    seed *= 0x100000001b3ULL;
  }
  return seed;
}

/* compute_uid computes a consistent UID from the hash of a username.
 *
 * The function checks if the computed UID is free before returning
 * it, and computes a different hash if it detects a collision.
 *
 * pseed is used as a seed for the hash function.
 * min uid and max uid define the range of valid uids returned.
 * attempts is the maximum number of tries to find a free uid.
 *
 * Returns 0 if no UID can be found.
 * 0 was intentionally picked to force the caller to have code rejecting
 * 0 as a valid UID - even if it was returned by mistake.
 *
 * WARNING: this function is inherently racy. Until the UID is added
 * to the system database (and this function does NOT add the UID),
 * the same UID could be assigned to a different, concurrent, user.
 *
 * Before authorizing an user for login, ALWAYS ALWAYS create the
 * corresponding record in the user database - to lock in the
 * mapping between UID <-> user. Failure to add should result in
 * rejecting the user. */
static uid_t compute_uid(const char* pseed, const char* name, uid_t min, uid_t max, int attempts) {
  static_assert(
    sizeof(uint32_t) == sizeof(uid_t),
    "uid_t is not an uint32_t, hash function assumes 32 bit uids");

  uint64_t hvalue = seed(pseed);
  for (int i = 0; i < attempts; i++) {
    hvalue = hash(hvalue, name);

    uid_t uid = min + (hvalue % (max - min + 1));
    if (!getpwuid(uid)) return uid;
  }

  return 0;
}

/* suffix_index returns the index where the specified suffix starts
 * in the input string.
 *
 * Returns -1 in case the suffix cannot be found. */
static int suffix_index(const char* input, const char* suffix) {
  size_t inputl = strlen(input);
  size_t suffixl = strlen(suffix);

  if (suffixl > inputl || strcmp(input + (inputl - suffixl), suffix))
    return -1;

  return inputl - suffixl;
}

static void config_merge(autouser_match_t* dest, autouser_match_t* source) {
  if (!source || !dest) return;

  if (source->argv != NULL && *source->argv) dest->argv = source->argv;
  if (source->suffix != NULL && *source->suffix) dest->suffix = source->suffix;
  if (source->shell != NULL && *source->shell) dest->shell = source->shell;
  if (source->home != NULL && *source->home) dest->home = source->home;
  if (source->gecos != NULL && *source->gecos) dest->gecos = source->gecos;

  if (source->min_uid > 0) dest->min_uid = source->min_uid;
  if (source->max_uid > 0) dest->max_uid = source->max_uid;
  if (source->gid > 0) dest->gid = source->gid;

  if (source->flags & MATCH_SET_PASSWORD)
    dest->flags = assign_bits(dest->flags, source->flags,  MATCH_SET_PASSWORD | MATCH_USE_PASSWORD);
  if (source->flags & MATCH_SET_FULL_HOME)
    dest->flags = assign_bits(dest->flags, source->flags,  MATCH_SET_FULL_HOME | MATCH_USE_FULL_HOME);
}

static int config_apply(const autouser_config_t* config, const char* process, const char* name, autouser_match_t *result) {
  autouser_match_t* def_process_def_user = NULL;
  autouser_match_t* def_process_set_user = NULL;
  autouser_match_t* set_process_def_user = NULL;
  autouser_match_t* set_process_set_user = NULL;

  int def_suffix_offset = -1;
  int set_suffix_offset = -1;
  for (int i = 0; i < config->matchn; ++i) {
    autouser_match_t* m = &config->match[i];
    int offset = 0;
    if (!m->argv || !*m->argv) {
      if (!m->suffix || !*m->suffix) {
        def_process_def_user = m;
      } else if ((offset = suffix_index(name, m->suffix)) >= 0) {
	def_process_set_user = m;
	def_suffix_offset = offset;
      }
    } else if (!fnmatch(m->argv, process, FNM_PATHNAME)) {
      if (!m->suffix || !*m->suffix) {
        set_process_def_user = m;
      } else if ((offset = suffix_index(name, m->suffix)) >= 0) {
	set_process_set_user = m;
	set_suffix_offset = offset;
      }
    }
  }

  config_merge(result, def_process_def_user);
  config_merge(result, def_process_set_user);
  config_merge(result, set_process_def_user);
  config_merge(result, set_process_set_user);

  return set_suffix_offset >= 0 ? set_suffix_offset : def_suffix_offset;
}

typedef enum {
  SR_FULL_DIR = 1<<0,
  SR_AUTO_GEN = 1<<1,
} store_result_flags_e;

int store_result(const char* original, const char* name, uid_t uid, autouser_match_t* match, const char* password, char* buffer, size_t buflen, struct passwd* pwd, uint32_t flags) {
  char* cursor = buffer;
  const char* end = cursor + buflen;

  pwd->pw_uid = uid;
  pwd->pw_gid = match->gid ? match->gid : uid;

  pwd->pw_name = add(&cursor, end, name);
  pwd->pw_passwd = add(&cursor, end, password ? password : "*");
  pwd->pw_gecos = add(&cursor, end, match->gecos ? match->gecos : "");
  pwd->pw_shell = add(&cursor, end, (match->shell && *match->shell) ? match->shell : AU_DEFAULT_SHELL);

  if (!cursor) return -1;

  if (match->home && *match->home && (flags & SR_FULL_DIR)) {
    pwd->pw_dir = add(&cursor, end, match->home);
    if (!cursor) return -1;
  } else {
    const char* home = (match->home && *match->home) ? match->home : "/home";
    if (!cursor || snprintf(cursor, end - cursor, "%s/%s", home, name) >= end-cursor) {
            return -1;
    }
    pwd->pw_dir = cursor;
  }

  static_assert(
    sizeof(uint32_t) == sizeof(uid_t),
    "uid_t is not an uint32_t, printf casts uid_t to uint32_t");

  setenv("AUTOUSER_ORIGINAL", original, 1);
  setenv("AUTOUSER_NAME", pwd->pw_name, 1);
  setenv("AUTOUSER_SHELL", pwd->pw_shell, 1);
  setenv("AUTOUSER_HOME", pwd->pw_dir, 1);
  setenv("AUTOUSER_GECOS", pwd->pw_gecos, 1);
  if (flags & SR_AUTO_GEN)
    setenv("AUTOUSER_AUTOGEN", "true", 1);
  else
    setenv("AUTOUSER_AUTOGEN", "false", 1);

  char ibuffer[32];
  snprintf(ibuffer, sizeof(ibuffer), "%"PRIu32, (uint32_t)(pwd->pw_uid));
  setenv("AUTOUSER_UID", ibuffer, 1);
  snprintf(ibuffer, sizeof(ibuffer), "%"PRIu32, (uint32_t)(pwd->pw_gid));
  setenv("AUTOUSER_GID", ibuffer, 1);

  return 0;
}

static void config_dump_match(autouser_config_t* config, autouser_match_t* match) {
  mlog(config->debug, LOG_INFO, "config:   argv %s", match->argv);
  mlog(config->debug, LOG_INFO, "config:   suffix %s", match->suffix);
  mlog(config->debug, LOG_INFO, "config:   shell %s", match->shell);
  mlog(config->debug, LOG_INFO, "config:   home %s", match->home);
  mlog(config->debug, LOG_INFO, "config:   gecos %s", match->gecos);
  mlog(config->debug, LOG_INFO, "config:   min_uid %" PRIu32, match->min_uid);
  mlog(config->debug, LOG_INFO, "config:   max_uid %" PRIu32, match->max_uid);
  mlog(config->debug, LOG_INFO, "config:   gid %" PRIu32, match->gid);
  mlog(config->debug, LOG_INFO, "config:   flags %08x", match->flags);
}

static void config_dump(autouser_config_t* config) {
  mlog(config->debug, LOG_INFO, "config: DebugLog %s", config->debug);
  mlog(config->debug, LOG_INFO, "config: Seed %s", config->seed ? "(set but hidden)" : "(unset)");
  for (int i = 0; i < config->matchn; ++i) {
    mlog(config->debug, LOG_INFO, "config: Entry %d:", i);
    config_dump_match(config, &config->match[i]);
  }
}

/* Return values are based on:
 *   https://www.gnu.org/software/libc/manual/html_node/NSS-Modules-Interface.html */
enum nss_status _nss_autouser_getpwnam_r(
    const char *name, struct passwd *pwd, char *buffer, size_t buflen, int *errnop) {
  static thread_local bool nesting = false;
  if (nesting) {
    if (errnop) *errnop = 0;
    return NSS_STATUS_NOTFOUND;
  }

  autouser_config_t config = {
    .seed = NULL,
  };

  if (config_parse(AU_CONFIG_PATH, &config) < 0) { 
    if (errnop) *errnop = ENOENT;
    return NSS_STATUS_UNAVAIL;
  }

  if (config.debug) config_dump(&config);

  int ierrno = 0; /* internal errno. */
  int status = NSS_STATUS_SUCCESS;
  const char *namedup = NULL, *original = name;
  if (config.matchn <= 0) {
    mlog(config.debug, LOG_ERR, "no rules specified in %s - disabled", AU_CONFIG_PATH);
    ierrno = ENOENT;
    status = NSS_STATUS_UNAVAIL;
    goto exit;
  }

  if (process_argc <= 0 || process_argv == NULL || process_argv[0] == NULL) {
    mlog(config.debug, LOG_ERR, "argv could not be detected - disabled - "
		  "this often indicates a glibc incompatibility");
    ierrno = ENOENT;
    status = NSS_STATUS_UNAVAIL;
    goto exit;
  }

  autouser_match_t match = {NULL};
  int index = config_apply(&config, process_argv[0], name, &match);

  if (config.debug) {
    mlog(config.debug, LOG_INFO, "computed configuration for user:'%s' process:'%s'", name, process_argv[0]);
    config_dump_match(&config, &match);
  }

  if (index >= 0) {
    if (!match.min_uid && !match.max_uid && !match.gid) {
      mlog(config.debug, LOG_WARNING, "user:%s has a policy that does not specify MinUid, MaxUid, nor Gid - ignoring", name);
      ierrno = EINVAL;
      status = NSS_STATUS_NOTFOUND;
      goto exit;
    }

    // Necessary as we can't shove a \0 in the middle of a user supplied buffer.
    namedup = name = strndup(name, index);

    char tmpbuffer[buflen];
    memset(tmpbuffer, 0, buflen);

    nesting = true;
    struct passwd* result = NULL;
    int gp = getpwnam_r(name, pwd, tmpbuffer, buflen, &result);
    nesting = false;

    mlog(config.debug, LOG_DEBUG,
	 "user:%s - setting config based on prefix - %p - %s", name, (void*)result, match.shell);

    if (gp == 0 && result != NULL)  {
      if (((match.min_uid || match.max_uid) && (result->pw_uid < match.min_uid || result->pw_uid > match.max_uid)) ||
	  (match.gid && result->pw_gid != match.gid)) {
        mlog(config.debug, LOG_INFO, "user:%s - refusing to apply policy - uid:%"PRIu32"or gid:%"PRIu32" not allowed", name, result->pw_uid, result->pw_gid);
        ierrno = EINVAL;
        status = NSS_STATUS_NOTFOUND;
        goto exit;
      }

      match.gid = result->pw_gid;

      if (!match.shell || !*match.shell) match.shell = pwd->pw_shell;
      if (!match.home || !*match.home) match.home = pwd->pw_dir;
      if (!match.gecos || !*match.gecos) match.gecos = pwd->pw_gecos;

      const char* passwd = match.flags & MATCH_USE_PASSWORD ? pwd->pw_passwd : NULL;
      if (store_result(original, name, result->pw_uid, &match, passwd, buffer, buflen, pwd, SR_FULL_DIR) != 0) {
	mlog(config.debug, LOG_DEBUG, "user:%s - in suffix handler - buffer too small %zu, could not store result", name, buflen);
        ierrno = ERANGE;
        status = NSS_STATUS_TRYAGAIN;
      }

      goto exit;
    }
  }

  /* Never ever allow a root UID. */
  if (match.min_uid <= 0 || match.max_uid <= 0) {
    mlog(config.debug, LOG_DEBUG, "%s - no uid set - ignoring", name);
    /* Lookup and configuration was successful, but the configuration
     * tells us not to do anything for this user. */
    ierrno = 0;
    status = NSS_STATUS_NOTFOUND;
    goto exit;
  }

  uid_t uid = compute_uid(
      config.seed ? config.seed : "default-seed", name,
      match.min_uid, match.max_uid, AU_HASH_ATTEMPTS);
  /* Never ever allow a root UID. 0 indicates failure. */
  if (!uid) {
    mlog(config.debug, LOG_ERR, "hashing '%s' generated clashing uids for %d times", name, AU_HASH_ATTEMPTS);
    ierrno = ENOENT;
    status = NSS_STATUS_NOTFOUND;
    goto exit;
  }
  
  uint32_t flags = match.flags & MATCH_USE_FULL_HOME ? SR_FULL_DIR : 0;
  if (store_result(original, name, uid, &match, NULL, buffer, buflen, pwd, SR_AUTO_GEN | flags) != 0) {
    mlog(config.debug, LOG_DEBUG, "in auto handler - could not store result for %s", name);
    ierrno = ERANGE;
    status = NSS_STATUS_TRYAGAIN;
  }

exit:
  if (status == NSS_STATUS_SUCCESS) {
    mlog(config.debug, LOG_DEBUG,
	 "user:%s - status:%d errno:%d uid:%" PRIu32 " gid:%" PRIu32 " home:%s gecos:%s shell:%s",
         name, status, ierrno, pwd->pw_uid, pwd->pw_gid, pwd->pw_dir, pwd->pw_gecos, pwd->pw_shell);
  } else {
    mlog(config.debug, LOG_DEBUG, "user:%s - status:%d errno:%d", name, status, ierrno);
  }

  free((void*)namedup);
  config_free(&config);

  if (errnop) *errnop = ierrno;
  return status;
}

static int load(int argc, char** argv, char** env) {
  if (argv) process_argv = argv;
  if (argc) process_argc = argc;
  if (env) process_env = env;
  return 0;
}

/* According to ELF specification, .init_array contains a vector of functions
 * to invoke when the .so file is loaded.
 *
 * glibc and a few other libraries provide those functions a pointer to the
 * original argc, argv, and env that was passed to the program.
 * We use this to our benefit. */
__attribute__((section(".init_array"))) __attribute__((used))
    static int (*init)(int argc, char** argv, char** env) = &load;
