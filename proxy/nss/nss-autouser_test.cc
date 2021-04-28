#include <gmock/gmock.h>
#include <gtest/gtest.h>

#include <nss.h>
#include <pwd.h>
#include <sys/types.h>
#include <fstream>

extern "C" {
#include "nss-autouser.h"

extern uid_t compute_uid(const char* pseed, const char* name, uid_t min, uid_t max, int attempts);
extern int suffix_index(const char* input, const char* suffix);
extern int config_parse(const char* path, autouser_config_t* config);
extern void config_free(autouser_config_t* config);
extern int config_apply(const autouser_config_t* config, const char* process, const char* name, autouser_match_t *result);
extern int store_result(const char* original, const char* name, uid_t uid, autouser_match_t* match, const char* password, char* buffer, size_t buflen, struct passwd* pwd, uint32_t flags);
extern enum nss_status _nss_autouser_getpwnam_r(
    const char *name, struct passwd *pwd, char *buffer, size_t buflen, int *errnop);
}


TEST(SuffixIndex, Basic) {
  EXPECT_EQ(-1, suffix_index("foo", "bar"));
  EXPECT_EQ(0, suffix_index("foo", "foo"));
  EXPECT_EQ(1, suffix_index("foo", "oo"));
  EXPECT_EQ(2, suffix_index("foo", "o"));
  EXPECT_EQ(3, suffix_index("foo", ""));
  EXPECT_EQ(0, suffix_index("", ""));
  EXPECT_EQ(-1, suffix_index("", "baz"));
  EXPECT_EQ(-1, suffix_index("foobazz", "baz"));
  EXPECT_EQ(3, suffix_index("foobaz", "baz"));
}

TEST(ComputeUid, Basic) {
  auto uid1 = compute_uid("test-seed", "test", 1, 100000, 10);
  EXPECT_LE(1, uid1);
  EXPECT_GE(100000, uid1);

  auto uid2 = compute_uid("test-seed", "test", 1, 100000, 10);
  EXPECT_EQ(uid1, uid2) << "same user, same seed, same uid expcted";

  auto uid3 = compute_uid("tost-seed", "test", 1, 100000, 10);
  EXPECT_NE(uid1, uid3) << "same user, different seed, different uid";
}

TEST(ComputeUid, Distribution) {
  int distribution[10] = {0};
  for (int i = 0; i < 1000; ++i) {
    const auto& user = "fake-user-" + std::to_string(i);
    const auto uid = compute_uid("seed", user.c_str(), 100000, 100009, 10);
    EXPECT_LE(100000, uid);
    EXPECT_GE(100009, uid);
    ++distribution[uid - 100000];
  }

  for (const auto& dist : distribution) {
    std::cerr << "VALUE " << dist << std::endl;
  }

  auto [min, max] = std::minmax_element(distribution, distribution + 10);
  EXPECT_GE(15, *max - *min);
  EXPECT_LE(90, *min);
  EXPECT_GE(110, *max);
}

TEST(Config, ParseApplyFree) {
  autouser_config_t config = {};

  int status = config_parse("proxy/nss/testdata/empty.conf", &config);
  EXPECT_EQ(0, status);
  EXPECT_EQ(0, config.matchn);
  EXPECT_EQ(NULL, config.match);
  EXPECT_EQ(NULL, config.seed);
  config_free(&config);

  status = config_parse("proxy/nss/testdata/simple.conf", &config);
  EXPECT_EQ(0, status);
  EXPECT_STREQ("fuffa", config.seed);
  ASSERT_EQ(1, config.matchn);
  EXPECT_EQ(NULL, config.match[0].argv);
  EXPECT_EQ(NULL, config.match[0].suffix);
  EXPECT_EQ(NULL, config.match[0].shell);
  EXPECT_EQ(NULL, config.match[0].home);
  EXPECT_EQ(NULL, config.match[0].gecos);
  EXPECT_EQ(70000, config.match[0].min_uid);
  EXPECT_EQ(0xfffffff0, config.match[0].max_uid);
  EXPECT_EQ(0, config.match[0].gid);
  EXPECT_EQ(0x22, config.match[0].flags);

  autouser_match_t result = {};
  int offset = config_apply(&config, "ssh", "zarathustra", &result);
  EXPECT_GT(0, offset);

  EXPECT_EQ(NULL, result.argv);
  EXPECT_EQ(NULL, result.suffix);
  EXPECT_EQ(NULL, result.shell);
  EXPECT_EQ(NULL, result.home);
  EXPECT_EQ(NULL, result.gecos);
  EXPECT_EQ(70000, result.min_uid);
  EXPECT_EQ(0xfffffff0, result.max_uid);
  EXPECT_EQ(0, result.gid);
  EXPECT_EQ(0x22, result.flags);
  config_free(&config);

  status = config_parse("proxy/nss/testdata/advanced.conf", &config);
  EXPECT_EQ(0, status);
  EXPECT_STREQ("fuffa", config.seed);
  ASSERT_EQ(6, config.matchn);

  EXPECT_STREQ(NULL, config.match[0].argv);
  EXPECT_STREQ(NULL, config.match[0].suffix);
  EXPECT_STREQ(NULL, config.match[0].shell);
  EXPECT_STREQ(NULL, config.match[0].home);
  EXPECT_STREQ(NULL, config.match[0].gecos);
  EXPECT_EQ(70000, config.match[0].min_uid);
  EXPECT_EQ(0xfffffff0, config.match[0].max_uid);
  EXPECT_EQ(1000, config.match[0].gid);
  EXPECT_EQ(0x22, config.match[0].flags);

  EXPECT_STREQ("sshd*", config.match[1].argv);
  EXPECT_STREQ(NULL, config.match[1].suffix);
  EXPECT_STREQ("/bin/docker-login", config.match[1].shell);
  EXPECT_STREQ(NULL, config.match[1].home);
  EXPECT_STREQ(NULL, config.match[1].gecos);
  EXPECT_EQ(70000, config.match[1].min_uid);
  EXPECT_EQ(0xfffffff1, config.match[1].max_uid);
  EXPECT_EQ(0, config.match[1].gid);
  EXPECT_EQ(0, config.match[1].flags);

  EXPECT_STREQ("sshd*", config.match[2].argv);
  EXPECT_STREQ(":system", config.match[2].suffix);
  EXPECT_STREQ("/bin/bash", config.match[2].shell);
  EXPECT_STREQ(NULL, config.match[2].home);
  EXPECT_STREQ(NULL, config.match[2].gecos);
  EXPECT_EQ(0, config.match[2].min_uid);
  EXPECT_EQ(0, config.match[2].max_uid);
  EXPECT_EQ(0, config.match[2].gid);
  EXPECT_EQ(0, config.match[2].flags);

  EXPECT_STREQ("sshd*", config.match[3].argv);
  EXPECT_STREQ(":debug", config.match[3].suffix);

  EXPECT_STREQ("login", config.match[4].argv);
  EXPECT_STREQ(":system", config.match[4].suffix);

  EXPECT_STREQ("login", config.match[5].argv);
  EXPECT_STREQ(":debug", config.match[5].suffix);

  result = autouser_match_t{};
  offset = config_apply(&config, "sshdrive", "zarathustra", &result);
  EXPECT_GT(0, offset);

  EXPECT_STREQ("sshd*", result.argv);
  EXPECT_STREQ(NULL, result.suffix);
  EXPECT_STREQ("/bin/docker-login", result.shell);
  EXPECT_STREQ(NULL, result.home);
  EXPECT_STREQ(NULL, result.gecos);
  EXPECT_EQ(70000, result.min_uid);
  EXPECT_EQ(0xfffffff1, result.max_uid);
  EXPECT_EQ(1000, result.gid);
  EXPECT_EQ(0x22, result.flags);

  result = autouser_match_t{};
  offset = config_apply(&config, "sshdrive", "zara:system", &result);
  EXPECT_EQ(4, offset);

  EXPECT_STREQ("sshd*", result.argv);
  EXPECT_STREQ(":system", result.suffix);
  EXPECT_STREQ("/bin/bash", result.shell);
  EXPECT_STREQ(NULL, result.home);
  EXPECT_STREQ(NULL, result.gecos);
  EXPECT_EQ(70000, result.min_uid);
  EXPECT_EQ(0xfffffff1, result.max_uid);
  EXPECT_EQ(1000, result.gid);
  EXPECT_EQ(0x22, result.flags);

  result = autouser_match_t{};
  offset = config_apply(&config, "login", "zara:debug", &result);

  EXPECT_STREQ("login", result.argv);
  EXPECT_STREQ(":debug", result.suffix);
  EXPECT_STREQ("/bin/tcpdump", result.shell);
  EXPECT_STREQ(NULL, result.home);
  EXPECT_STREQ(NULL, result.gecos);
  EXPECT_EQ(70000, result.min_uid);
  EXPECT_EQ(0xfffffff0, result.max_uid);
  EXPECT_EQ(1000, result.gid);
  EXPECT_EQ(0x22, result.flags);

  config_free(&config);
}

TEST(Config, StoreResult) {
  autouser_match_t match = {};
  char buffer[1024];
  struct passwd pwd = {};

  // Purposedly short buffer, storing will fail.
  EXPECT_GT(0, store_result("fooz", "foo", 1200, &match, NULL, buffer, 7, &pwd, 0));
  EXPECT_STREQ(NULL, getenv("AUTOUSER_NAME"));
  EXPECT_STREQ(NULL, getenv("AUTOUSER_SHELL"));
  EXPECT_STREQ(NULL, getenv("AUTOUSER_HOME"));
  EXPECT_STREQ(NULL, getenv("AUTOUSER_UID"));
  EXPECT_STREQ(NULL, getenv("AUTOUSER_GID"));

  // An empty match structure, will result in all defaults being used.
  EXPECT_EQ(0, store_result("fooz", "foo", 1200, &match, NULL, buffer, 1024, &pwd, 0));
  EXPECT_STREQ("fooz", getenv("AUTOUSER_ORIGINAL"));
  EXPECT_STREQ("foo", getenv("AUTOUSER_NAME"));
  EXPECT_STREQ("/bin/bash", getenv("AUTOUSER_SHELL"));
  EXPECT_STREQ("/home/foo", getenv("AUTOUSER_HOME"));
  EXPECT_STREQ("1200", getenv("AUTOUSER_UID"));
  EXPECT_STREQ("1200", getenv("AUTOUSER_GID"));

  EXPECT_STREQ("foo", pwd.pw_name);
  EXPECT_STREQ("/bin/bash", pwd.pw_shell);
  EXPECT_STREQ("/home/foo", pwd.pw_dir);
  EXPECT_EQ(1200, pwd.pw_uid);
  EXPECT_EQ(1200, pwd.pw_gid);
  EXPECT_STREQ("*", pwd.pw_passwd);

  // A match with some arbitrary values.
  match = autouser_match_t{
    .shell = "/bin/unabashed",
    .home = "/tmp/goaway",
    .gecos = "foo bar",
    .gid = 42,
  };
  EXPECT_EQ(0, store_result("fooz", "foxy", 67, &match, NULL, buffer, 1024, &pwd, 0));
  EXPECT_STREQ("foxy", getenv("AUTOUSER_NAME"));
  EXPECT_STREQ("/bin/unabashed", getenv("AUTOUSER_SHELL"));
  EXPECT_STREQ("/tmp/goaway/foxy", getenv("AUTOUSER_HOME"));
  EXPECT_STREQ("67", getenv("AUTOUSER_UID"));
  EXPECT_STREQ("42", getenv("AUTOUSER_GID"));

  EXPECT_STREQ("foxy", pwd.pw_name);
  EXPECT_STREQ("/bin/unabashed", pwd.pw_shell);
  EXPECT_STREQ("/tmp/goaway/foxy", pwd.pw_dir);
  EXPECT_EQ(67, pwd.pw_uid);
  EXPECT_EQ(42, pwd.pw_gid);
  EXPECT_STREQ("*", pwd.pw_passwd);

  // Set password and full dir, annoyingly.
  EXPECT_EQ(0, store_result("fooz", "foxy", 67, &match, "goo", buffer, 1024, &pwd, 1));
  EXPECT_STREQ("goo", pwd.pw_passwd);
  EXPECT_STREQ("/tmp/goaway", getenv("AUTOUSER_HOME"));
  EXPECT_STREQ("/tmp/goaway", pwd.pw_dir);
}

TEST(Config, GetpwnamR) {
  const auto config = R"""(
Seed test

MinUid 7000
MaxUid 8000

Suffix :system
  Shell /bin/bash

Suffix :ducker
  Shell /bin/docker-login

Suffix :docker
  MinUid 1
  MaxUid 1000
  Shell /bin/docker-login
)""";
  
  struct passwd pwd = {};
  char buffer[1024] = {};
  int err = 0;
  auto status = _nss_autouser_getpwnam_r("bin", &pwd, buffer, 1024, &err);
  EXPECT_EQ(NSS_STATUS_UNAVAIL, status);
  EXPECT_EQ(ENOENT, err);

  std::ofstream file("./nss-autouser.conf");
  file << config;
  file.close();

  status = _nss_autouser_getpwnam_r("bin:ducker", &pwd, buffer, 1024, &err);
  EXPECT_EQ(NSS_STATUS_NOTFOUND, status);
  EXPECT_EQ(EINVAL, err);

  status = _nss_autouser_getpwnam_r("bin:docker", &pwd, buffer, 1024, &err);
  EXPECT_EQ(NSS_STATUS_SUCCESS, status);
  EXPECT_EQ(0, err);

  EXPECT_STREQ("bin", pwd.pw_name);
  EXPECT_STREQ("/bin/docker-login", pwd.pw_shell);
  EXPECT_STREQ("/bin", pwd.pw_dir);
  EXPECT_EQ(2, pwd.pw_uid);
  EXPECT_EQ(2, pwd.pw_gid);
  EXPECT_STREQ("*", pwd.pw_passwd);

  status = _nss_autouser_getpwnam_r("fueller", &pwd, buffer, 1024, &err);
  EXPECT_EQ(NSS_STATUS_SUCCESS, status);
  EXPECT_EQ(0, err);

  EXPECT_STREQ("fueller", pwd.pw_name);
  EXPECT_STREQ("/bin/bash", pwd.pw_shell);
  EXPECT_STREQ("/home/fueller", pwd.pw_dir);
  EXPECT_EQ(7776, pwd.pw_uid);
  EXPECT_EQ(7776, pwd.pw_gid);
  EXPECT_STREQ("*", pwd.pw_passwd);
}
