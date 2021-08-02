#include <gmock/gmock.h>
#include <gtest/gtest.h>

extern "C" {
#include "confparse.h"
}

TEST(SkipLineSpaces, All) {
  perror_t error = {};

  const char* buffer = "Success is not final";
  auto ctx = context_from_buffer(buffer, &error);
  skip_line_spaces(&ctx);
  EXPECT_EQ('S', *ctx.cursor);

  buffer = "\t\t  Failure is not final";
  ctx = context_from_buffer(buffer, &error);
  skip_line_spaces(&ctx);
  EXPECT_EQ('F', *ctx.cursor);

  buffer = "   \r It is the courage to continue that counts";
  ctx = context_from_buffer(buffer, &error);
  skip_line_spaces(&ctx);
  EXPECT_EQ('\r', *ctx.cursor);

  buffer = "";
  ctx = context_from_buffer(buffer, &error);
  skip_line_spaces(&ctx);
  EXPECT_EQ('\0', *ctx.cursor);
  perror_free(&error);
}

TEST(SkipUntilEOL, All) {
  perror_t error = {};

  const char* buffer = "Success is not final";
  auto ctx = context_from_buffer(buffer, &error);
  skip_until_eol(&ctx);
  EXPECT_EQ('\0', *ctx.cursor);

  buffer = "";
  ctx = context_from_buffer(buffer, &error);
  skip_until_eol(&ctx);
  EXPECT_EQ('\0', *ctx.cursor);

  buffer = "   \r It is the courage\n to continue that counts";
  ctx = context_from_buffer(buffer, &error);
  skip_until_eol(&ctx);
  EXPECT_EQ('\n', *ctx.cursor);
  EXPECT_EQ(" to continue that counts", std::string_view(ctx.cursor + 1));

  buffer = "   \rit\nis\nthe";
  ctx = context_from_buffer(buffer, &error);
  skip_until_eol(&ctx);
  EXPECT_EQ('\n', *ctx.cursor);
  EXPECT_EQ("is\nthe", std::string_view(ctx.cursor + 1));
  perror_free(&error);
}

TEST(SkipUntilField, All) {
  perror_t error = {};
  const char* buffer = "Success is not final";
  auto ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, skip_until_field(&ctx));
  EXPECT_EQ('S', *ctx.cursor);

  buffer = " \t   Success";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, skip_until_field(&ctx));
  EXPECT_EQ('S', *ctx.cursor);

  buffer = "";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, skip_until_field(&ctx));
  EXPECT_EQ('\0', *ctx.cursor);

  buffer = "    \n   fuffa";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, skip_until_field(&ctx));
  EXPECT_EQ('\n', *ctx.cursor);
  perror_free(&error);
}

TEST(ParseBool32, All) {
  perror_t error = {};
  uint32_t result = 0;

  const char* buffer = "";
  auto ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));

  buffer = "of";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));

  buffer = "   True";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x11, result);

  buffer = "true";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x11, result);

  buffer = "on ";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x11, result);

  result = 0x1000;
  buffer = "yes blah";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x1011, result);

  buffer = " yesyes";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x1011, result);

  result = 0x1111;
  buffer = "no";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x1110, result);

  buffer = "off ";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x1110, result);

  buffer = "false ";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x1110, result);

  buffer = "False ";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_bool32(&ctx, 0x10, 0x1, &result));
  EXPECT_EQ(0x1110, result);

  perror_free(&error);
}

TEST(ParseUint32, All) {
  perror_t error = {};
  uint64_t result = 0;

  const char* buffer = "";
  auto ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_uint64(&ctx, UINT32_MAX, &result));

  buffer = "   16";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_uint64(&ctx, UINT32_MAX, &result));
  EXPECT_EQ(16, result);
  EXPECT_EQ('\0', *ctx.cursor);

  buffer = "   0x10  ";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_uint64(&ctx, UINT32_MAX, &result));
  EXPECT_EQ(16, result);
  EXPECT_EQ(' ', *ctx.cursor);

  result = 0;
  buffer = "   0x1g  ";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_uint64(&ctx, UINT32_MAX, &result));
  EXPECT_EQ(0, result);
  EXPECT_EQ('g', *ctx.cursor);

  buffer = "   0x10\n";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_uint64(&ctx, UINT32_MAX, &result));
  EXPECT_EQ(16, result);
  EXPECT_EQ('\n', *ctx.cursor);
  perror_free(&error);
}

TEST(ParseQuotedString, All) {
  perror_t error = {};
  char* result = nullptr;

  const char* buffer = "";
  auto ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_quoted_string(&ctx, &result));

  buffer = "\"";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_quoted_string(&ctx, &result));

  buffer = "\"foo \n  ";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_quoted_string(&ctx, &result));

  buffer = "   \"foo\"";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_quoted_string(&ctx, &result));
  EXPECT_EQ("foo", std::string_view(result));
  EXPECT_EQ('\0', *ctx.cursor);
  free(result);

  buffer = "   \"foo\nbar    baz buz\"U";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_quoted_string(&ctx, &result));
  EXPECT_EQ("foo\nbar    baz buz", std::string_view(result));
  EXPECT_EQ('U', *ctx.cursor);
  free(result);

  // Invalid escape, \o is not supported!
  buffer = "\"f\\oo\"";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_quoted_string(&ctx, &result));

  buffer = "\"\\"; // escape at end of buffer.
  ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_quoted_string(&ctx, &result));

  buffer = "\"\\\\\""; // valid escape, 1 byte.
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_quoted_string(&ctx, &result));
  EXPECT_EQ("\\", std::string_view(result));
  EXPECT_EQ('\0', *ctx.cursor);
  free(result);

  buffer = "  \"\\\\foo\\\"bar\\\\ goo\"uff"; // escapepalooza.
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_quoted_string(&ctx, &result));
  EXPECT_EQ("\\foo\"bar\\ goo", std::string_view(result));
  EXPECT_EQ('u', *ctx.cursor);
  free(result);

  perror_free(&error);
}

TEST(ParseString, All) {
  perror_t error = {};
  char* result = nullptr;

  const char* buffer = "";
  auto ctx = context_from_buffer(buffer, &error);
  EXPECT_GE(0, parse_string(&ctx, &result));
  EXPECT_EQ(nullptr, result);

  buffer = "a";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_string(&ctx, &result));
  EXPECT_EQ("a", std::string_view(result));
  EXPECT_EQ('\0', *ctx.cursor);
  free(result);

  buffer = "   pluto";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_string(&ctx, &result));
  EXPECT_EQ("pluto", std::string_view(result));
  EXPECT_EQ('\0', *ctx.cursor);
  free(result);

  buffer = "   pluto topolino";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_string(&ctx, &result));
  EXPECT_EQ("pluto", std::string_view(result));
  EXPECT_EQ(' ', *ctx.cursor);
  free(result);

  buffer = "   pluto\ntopolino";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_string(&ctx, &result));
  EXPECT_EQ("pluto", std::string_view(result));
  EXPECT_EQ('\n', *ctx.cursor);
  free(result);

  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_string(&ctx, nullptr));

  // quoting is allowed in plain strings.
  buffer = "   \"plu to\nto\"polino";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_string(&ctx, &result));
  EXPECT_EQ("plu to\nto", std::string_view(result));
  EXPECT_EQ('p', *ctx.cursor);
  free(result);

  perror_free(&error);
}

TEST(ParseSection, Simple) {
  perror_t error = {};

  struct test {
    char* key;
    uint32_t value;
  } result = {};

  statement_t stats[] = {
	  { OPT_NONE, match_exact("Key"), expect_string(offsetof(test, key))},
	  { OPT_NONE, match_exact("Value"), expect_uint32(offsetof(test, value))},
    STATEMENTS_END,
  };

  const char* buffer = "";
  auto ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_section(&ctx, stats, &result));
  EXPECT_EQ(nullptr, result.key);
  EXPECT_EQ(0, result.value);

  // Simple valid config.
  buffer =
"   # this is a full fledged config\n"
" Key \"test key\"\n"
" Value 0x10 # I love this value";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(0, parse_section(&ctx, stats, &result)) << error.message;
  EXPECT_EQ("test key", std::string_view(result.key));
  EXPECT_EQ(16, result.value);

  // Invalid config, Value is repeated.
  buffer =
"   # this is a full fledged config\n"
" Key \"test\n key\"\n"
" Value 0x10 # I love this value\n"
" Value";
  ctx = context_from_buffer(buffer, &error);
  EXPECT_EQ(CPE_REPEATED, parse_section(&ctx, stats, &result));

  perror_free(&error);
}

typedef struct {
    char* key;
    uint32_t value;
} kv_t;

typedef struct {
    kv_t* kv;
    int kvn;
} result_t;

void *result_add_kv(result_t* result) {
  result->kv = (kv_t*)(reallocarray(result->kv, result->kvn + 1, sizeof(kv_t)));

  kv_t* entry = &result->kv[(result->kvn)++];
  entry->key = NULL;
  entry->value = 0;
  return entry;
}

TEST(ParseSection, SimpleRecursive) {
  perror_t error = {};

  result_t result = {
    .kv = NULL,
    .kvn = 0,
  };

  statement_t kv[] = {
	  { OPT_NONE, match_exact("Mapping"), expect_nothing() },
	  { OPT_NONE, match_exact("Key"), expect_string(offsetof(kv_t, key)) },
	  { OPT_NONE, match_exact("Value"), expect_uint32(offsetof(kv_t, value)) },
    STATEMENTS_END,
  };

  statement_t object[] = {
	  { OPT_MULTI, match_exact("Mapping"), expect_section(kv, (adder_f)(result_add_kv)) },
    STATEMENTS_END,
  };

  const char* buffer = "";
  EXPECT_EQ(0, parse_buffer(buffer, object, &result, &error));
  EXPECT_EQ(0, result.kvn);
  EXPECT_EQ(NULL, result.kv);

  buffer =
" # wow, this is a complex one\n"
"Mapping\n"
"  Key \"foo bar\" # a key\n"
"  Value 0x10\n"
"\n # A second mapping\n"
"Mapping\n"
"  Key meh # a key\n"
"  Value 0x100\n"
	;
  EXPECT_EQ(0, parse_buffer(buffer, object, &result, &error)) << error.message;
  ASSERT_EQ(2, result.kvn);
  EXPECT_STREQ("foo bar", result.kv[0].key);
  EXPECT_EQ(16, result.kv[0].value);
  EXPECT_STREQ("meh", result.kv[1].key);
  EXPECT_EQ(256, result.kv[1].value);

  perror_free(&error);
}

  typedef struct {
    const char* argv;
    const char* suffix;
  
    const char* shell;
    const char* home;
    const char* gecos;
    uid_t min_uid;
    uid_t max_uid;
    gid_t gid;
  } autouser_match_t;

  typedef struct {
    const char* seed;
  
    autouser_match_t *match;
    size_t matchn;
  } autouser_config_t;


void *add_autouser_match(autouser_config_t* config) {
  config->match = (autouser_match_t*)(reallocarray(config->match, config->matchn + 1, sizeof(autouser_match_t)));
  autouser_match_t* entry = &config->match[(config->matchn)++];
  *entry = {};

  fprintf(stderr, "ADDING\n");
  return entry;
}


TEST(ParseSection, NssExample) {
  perror_t error = {};

  statement_t suffix[] = {
	  { OPT_START, match_exact("Suffix"), expect_string(offsetof(autouser_match_t, suffix))},
    	  { OPT_NONE, match_exact("Shell"), expect_string(offsetof(autouser_match_t, shell))},
    	  { OPT_NONE, match_exact("Home"), expect_string(offsetof(autouser_match_t, home))},
    	  { OPT_NONE, match_exact("Gecos"), expect_string(offsetof(autouser_match_t, gecos))},

    	  { OPT_NONE, match_exact("MinUid"), expect_uint32(offsetof(autouser_match_t, min_uid))},
    	  { OPT_NONE, match_exact("MaxUid"), expect_uint32(offsetof(autouser_match_t, max_uid))},
    	  { OPT_NONE, match_exact("Gid"), expect_uint32(offsetof(autouser_match_t, gid))},
    STATEMENTS_END,
  };

  statement_t match[] = {
	  { OPT_START, match_exact("Match"), expect_string(offsetof(autouser_match_t, argv))}, // eat token or not
    	  { OPT_NONE, match_any(), expect_section(suffix, NULL)}, // enter section or not
    STATEMENTS_END,
  };

  statement_t root[] = {
	  { OPT_NONE, match_exact("Seed"), expect_string(offsetof(autouser_config_t, seed))},
    	  { OPT_MULTI, match_any(), expect_section(match, (adder_f)(add_autouser_match))}, // enter section or not
    STATEMENTS_END,
  };

  autouser_config_t result = {};

  const char* buffer = "";
  EXPECT_EQ(0, parse_buffer(buffer, root, &result, &error));

  buffer = "Seed foobarbaz";
  EXPECT_EQ(0, parse_buffer(buffer, root, &result, &error));
  ASSERT_EQ(0, result.matchn);
  EXPECT_STREQ("foobarbaz", result.seed);

  buffer = "Seed foobarbaz\n"
    "MinUid 32";
  EXPECT_EQ(0, parse_buffer(buffer, root, &result, &error));
  EXPECT_STREQ("foobarbaz", result.seed);
  ASSERT_EQ(1, result.matchn);
  EXPECT_EQ(result.match[0].min_uid, 32);

  free(result.match);
  result.matchn = 0;
  buffer = "Seed foobarbaz\n"
    "MinUid 32\n"
    "MinUid 33";
  EXPECT_EQ(0, parse_buffer(buffer, root, &result, &error));
  EXPECT_STREQ("foobarbaz", result.seed);
  ASSERT_EQ(2, result.matchn);
  EXPECT_EQ(32, result.match[0].min_uid);
  EXPECT_EQ(33, result.match[1].min_uid);

  free(result.match);
  result.matchn = 0;
  buffer = "Seed foobarbaz\n"
    "  # this should end up a default match.\n"
    "MinUid 32\n"
    "# Here we create a match.\n"
    "Match match # well, what can we do.\n"
    "  \tMinUid 33";
  EXPECT_EQ(0, parse_buffer(buffer, root, &result, &error)) << error.message;
  EXPECT_STREQ("foobarbaz", result.seed);
  ASSERT_EQ(2, result.matchn);
  EXPECT_EQ(NULL, result.match[0].argv);
  EXPECT_EQ(32, result.match[0].min_uid);
  EXPECT_STREQ("match", result.match[1].argv);
  EXPECT_EQ(33, result.match[1].min_uid);

  free(result.match);
  result.matchn = 0;
  buffer = "Seed foobarbaz\n"
    "  # this should end up a default match.\n"
    "MinUid 32\n"
    "MaxUid 3201\n"
    "Shell foo\n"
    "# Here we create a match.\n"
    "Match match # well, what can we do.\n"
    "  \tMinUid 33\n"
    "Suffix one\n"
    "  Shell 14\n"
    "  MaxUid 5608\n"
    "Suffix two\n"
    "  Shell 15\n";
  EXPECT_EQ(0, parse_buffer(buffer, root, &result, &error)) << error.message;
  EXPECT_STREQ("foobarbaz", result.seed);
  ASSERT_EQ(4, result.matchn);

  EXPECT_EQ(NULL, result.match[0].argv);
  EXPECT_EQ(32, result.match[0].min_uid);
  EXPECT_EQ(3201, result.match[0].max_uid);
  EXPECT_STREQ("foo", result.match[0].shell);

  EXPECT_STREQ("match", result.match[1].argv);
  EXPECT_EQ(33, result.match[1].min_uid);

  EXPECT_STREQ(NULL, result.match[2].argv);
  EXPECT_EQ(5608, result.match[2].max_uid);
  EXPECT_STREQ("14", result.match[2].shell);
  EXPECT_STREQ("one", result.match[2].suffix);

  EXPECT_STREQ(NULL, result.match[3].argv);
  EXPECT_STREQ("15", result.match[3].shell);
  EXPECT_STREQ("two", result.match[3].suffix);

  perror_free(&error);
}
