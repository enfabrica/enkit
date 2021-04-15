#include <nss.h>
#include <pwd.h>
#include <shadow.h>
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <stddef.h>
#include <stdarg.h>
#include <stdint.h>
#include <sys/types.h>
#include <pwd.h>
#include <errno.h>
#include <inttypes.h>
#include <assert.h>
#include <ctype.h>
#include <stdbool.h>

#include "confparse.h"

/* read_file reads an entire file in memory in a single operation.
 *
 * The file must be seekable(), so this does not work with /proc, /sys,
 * fifos, or sockets.
 *
 * The returned buffer is '\0' terminated, and must be freed with
 * free() unless an error is returned.
 *
 * Returns -1 in case of error, or the amount of bytes read. */
static ssize_t read_file(const char* path, char** buffer) {
  FILE* f = fopen(path, "rb");
  if (!f) return -1;

  fseek(f, 0, SEEK_END);
  long size = ftell(f);
  fseek (f, 0, SEEK_SET);
  if (!buffer) {
    fclose(f);
    return -1;
  }

  *buffer = malloc(size + 1);
  size_t got = fread(*buffer, 1, size, f);
  fclose (f);

  if (got < size) {
    free(*buffer);
    *buffer = NULL;
    return -1;
  }
  (*buffer)[size] = '\0';
  return size;
}

/* verrorp is just like error() below, but takes a va_list and prefix. */
error_code_e verrorp(perror_t* error, const char* prefix, const char* fmt, va_list input) {
  if (!error) return CPE_SUCCESS;
  perror_free(error);

  va_list args;
  va_copy(args, input);
  size_t size = vsnprintf(NULL, 0, fmt, args);
  va_end(args);
  
  if (size <= 0)
    return CPE_INTERNAL;

  int plen = 0;
  if (prefix)
    plen = strlen(prefix);

  error->message = malloc(plen + size + 1);

  if (prefix)
    strcpy(error->message, prefix);

  va_copy(args, input);
  size = vsnprintf(error->message + plen, size + 1, fmt, args);
  va_end(args);

  /* In the unlikely case vsnprintf failed, we don't want to return
   * a buffer containing garbage to the user. */
  if (size < 0) {
    perror_free(error);
  }

  return CPE_SUCCESS;
}
 
int error(perror_t* error, error_code_e code, const char* fmt, ...) {
  va_list args;
  va_start(args, fmt);
  int result = verrorp(error, "", fmt, args);
  va_end(args);

  if (code < 0) return code;
  return result;
}

error_code_e adapter_string(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  char** target = (char **)((char*)(dest) + data->offset);
  if (target && *target) { free(*target); *target = NULL; }
  return parse_string(ctx, target);
}

parse_t expect_string(size_t offset) {
  parse_t callback = {
      .function = adapter_string,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

error_code_e adapter_uint32(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  uint64_t value = 0;
  int status = parse_uint64(ctx, UINT32_MAX, &value);
  *(uint32_t *)((char*)(dest) + data->offset) = value;
  return status;
}

parse_t expect_uint32(size_t offset) {
  parse_t callback = {
      .function = adapter_uint32,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

error_code_e adapter_uint64(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  uint64_t value = 0;
  int status = parse_uint64(ctx, UINT64_MAX, &value);
  *(uint64_t *)((char*)(dest) + data->offset) = value;
  return status;
}

parse_t expect_uint64(size_t offset) {
  parse_t callback = {
      .function = adapter_uint64,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

error_code_e adapter_uint16(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  uint64_t value = 0;
  int status = parse_uint64(ctx, UINT16_MAX, &value);
  *(uint16_t *)((char*)(dest) + data->offset) = value;
  return status;
}

parse_t expect_uint16(size_t offset) {
  parse_t callback = {
      .function = adapter_uint16,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

error_code_e adapter_uint8(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  uint64_t value = 0;
  int status = parse_uint64(ctx, UINT8_MAX, &value);
  *(uint16_t *)((char*)(dest) + data->offset) = value;
  return status;
}

parse_t expect_uint8(size_t offset) {
  parse_t callback = {
      .function = adapter_uint8,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

error_code_e adapter_int64(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  int64_t value = 0;
  int status = parse_int64(ctx, INT64_MIN, INT64_MAX, &value);
  *(int64_t*)((char*)(dest) + data->offset) = value;
  return status;
}

parse_t expect_int64(size_t offset) {
  parse_t callback = {
      .function = adapter_int64,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

error_code_e adapter_int32(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  int64_t value = 0;
  int status = parse_int64(ctx, INT32_MIN, INT32_MAX, &value);
  *(int32_t*)((char*)(dest) + data->offset) = value;
  return status;
}

parse_t expect_int32(size_t offset) {
  parse_t callback = {
      .function = adapter_int32,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

error_code_e adapter_int16(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  int64_t value = 0;
  int status = parse_int64(ctx, INT16_MIN, INT16_MAX, &value);
  *(int16_t*)((char*)(dest) + data->offset) = value;
  return status;
}

parse_t expect_int16(size_t offset) {
  parse_t callback = {
      .function = adapter_int16,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

error_code_e adapter_int8(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  int64_t value = 0;
  int status = parse_int64(ctx, INT8_MIN, INT8_MAX, &value);
  *(int8_t*)((char*)(dest) + data->offset) = value;
  return status;
}

parse_t expect_int8(size_t offset) {
  parse_t callback = {
      .function = adapter_int8,
      .data = {
        .offset = offset,
      }, 
  };
  return callback;
}

typedef struct {
  size_t len;
  const char* ptr;
} option_t;

/* In static context, only const expressions are allowed.
 * strlen, inline functions are not considered const.
 *
 * This macro is, however, VERY UNSAFE. name should be
 * a static string, don't even think about passing any
 * sort of expression in. */
#define OPTION_CONST(name) {sizeof(name) - 1, name}
#define OPTION_END {0, NULL}

int find_option_prefix(const option_t* options, const char* start) {
  for (int i = 0; options[i].ptr; ++i) {
    if (!strncmp(options[i].ptr, start, options[i].len)) return i;
  }
  return -1;
}

  static const option_t trues[] = {
	  OPTION_CONST("True"),
    	  OPTION_CONST("true"),
    	  OPTION_CONST("yes"),
    	  OPTION_CONST("on"),
	  OPTION_END,
  };

  static const option_t falses[] = {
	  OPTION_CONST("False"),
    	  OPTION_CONST("false"),
    	  OPTION_CONST("no"),
    	  OPTION_CONST("off"),
	  OPTION_END,
  };

error_code_e parse_bool32(parse_context_t *ctx, uint32_t seenbit, uint32_t flipbit, uint32_t *dest) {
  int result = skip_until_field(ctx);
  if (result < 0) return result;

  int option = 0;
  if ((option = find_option_prefix(trues, ctx->cursor)) >= 0) {
    *dest = assign_bits(*dest, seenbit | flipbit, seenbit | flipbit);
    ctx->cursor += trues[option].len;
  } else if ((option = find_option_prefix(falses, ctx->cursor)) >= 0) {
    *dest = assign_bits(*dest, seenbit, seenbit | flipbit);
    ctx->cursor += falses[option].len;
  } else {
    return parse_ctx_error(ctx, CPE_PARSE_BOOL, &ctx->line, "was expecting a field - found end of config");
  }

  if (!*ctx->cursor || isspace(*ctx->cursor)) return 0;
  return parse_ctx_error(ctx, CPE_PARSE_BOOL, &ctx->line, "unexpected character after bool %c", *ctx->cursor);
}

error_code_e parse_bool64(parse_context_t *ctx, uint64_t seenbit, uint64_t flipbit, uint64_t *dest) {
  int result = skip_until_field(ctx);
  if (result < 0) return result;

  int option = 0;
  if ((option = find_option_prefix(trues, ctx->cursor)) >= 0) {
    *dest = assign_bits(*dest, seenbit | flipbit, seenbit | flipbit);
    ctx->cursor += trues[option].len;
  } else if ((option = find_option_prefix(falses, ctx->cursor)) >= 0) {
    *dest = assign_bits(*dest, seenbit, seenbit | flipbit);
    ctx->cursor += falses[option].len;
  } else {
    return parse_ctx_error(ctx, CPE_PARSE_BOOL, &ctx->line, "was expecting a field - found end of config");
  }

  if (!*ctx->cursor || isspace(*ctx->cursor)) return 0;
  return parse_ctx_error(ctx, CPE_PARSE_BOOL, &ctx->line, "unexpected character after bool %c", *ctx->cursor);
}

error_code_e adapter_bool32(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  size_t offset = data->ints[0];
  uint32_t seenbit = data->ints[1];
  uint32_t flipbit = data->ints[2];
  
  return parse_bool32(ctx, seenbit, flipbit, (uint32_t *)((char*)(dest) + offset));
}

error_code_e adapter_bool64(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  size_t offset = data->ints[0];
  uint64_t seenbit = data->ints[1];
  uint64_t flipbit = data->ints[2];
  
  return parse_bool64(ctx, seenbit, flipbit, (uint64_t *)((char*)(dest) + offset));
}

parse_t expect_bool64(size_t offset, uint64_t seenbit, uint64_t flipbit) {
  parse_t callback = {
      .function = adapter_bool64,
      .data = {
	.ints = {
	  offset,
          seenbit,
	  flipbit,
	},
      }, 
  };
  return callback;
}



parse_t expect_bool32(size_t offset, uint32_t seenbit, uint32_t flipbit) {
  parse_t callback = {
      .function = adapter_bool32,
      .data = {
	.ints = {
	  offset,
          seenbit,
	  flipbit,
	},
      }, 
  };
  return callback;
}

error_code_e adapter_nothing(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  return CPE_SUCCESS;
}

parse_t expect_nothing() {
  parse_t callback = {
      .function = adapter_nothing,
  };
  return callback;
}

error_code_e adapter_section(parse_context_t* ctx, const char* start, types_u* data, void* dest) {
  statement_t* statements = (statement_t*)(data->ptrs[0]);
  /* ISO C forbids conversion of object pointer to function pointer. */
  adder_f adder;
  *(void**)(&adder) = data->ptrs[1];

  if (adder) {
    dest = adder(dest);
  }

  ctx->cursor = start;
  return parse_section(ctx, statements, dest);
}

parse_t expect_section(statement_t* statements, adder_f adder) {
  parse_t callback = {
      .function = adapter_section,
      .data = {
        .ptrs = {
	  statements,
	  *(void**)(&adder),
	},
      },
  };
  return callback;
}

error_code_e parse_ctx_error(parse_context_t* ctx, error_code_e code, const line_t* line, const char* fmt, ...) {
  char prefix[64];
  snprintf(prefix, sizeof(prefix), "line %d, char %td: ", line->number + 1, ctx->cursor - line->start);

  va_list args;
  va_start(args, fmt);
  int result = verrorp(ctx->err, prefix, fmt, args);
  va_end(args);

  if (code < 0) return code;
  return result;
}

void parse_ctx_newline(parse_context_t* ctx) {
  ctx->line.start = ctx->cursor + 1;
  (ctx->line.number)++;
}

/* skip_line_spaces moves the cursor past any 'line' space.
 *
 * A line space is any spacing chcaracter that can typically be found
 * on a single line of text, ' ' or '\t', but not '\r', '\n' or '\v'. */
void skip_line_spaces(parse_context_t* ctx) {
  for(; *ctx->cursor && (*ctx->cursor == ' ' || *ctx->cursor == '\t'); ++(ctx->cursor))
    ;
}

/* skip_until_field moves the cursor at the beginning of the first field.
 *
 * This is a convenience wrapper around skip_line_spaces that sets an
 * error in case no field can be found. */
error_code_e skip_until_field(parse_context_t* ctx) {
  skip_line_spaces(ctx);
  if (!*ctx->cursor) return parse_ctx_error(ctx, CPE_UNEXPECTED, &ctx->line, "was expecting a field - found end of config");
  if (isspace(*ctx->cursor)) return parse_ctx_error(ctx, CPE_UNEXPECTED, &ctx->line, "was expecting a field - found a new line? unexpected space");
  return 0;
}

/* skip_until_eol moves the cursor to the end of the current line. */
void skip_until_eol(parse_context_t* ctx) {
  for(; *ctx->cursor && *ctx->cursor != '\n'; ++(ctx->cursor))
    ;
}

/* parse_quoted_string parses a string enclosed in quotes (").
 *
 * The string can contain space characters, newlines, and can escape
 * quotes by using \", and escape \ itself with \\. */
error_code_e parse_quoted_string(parse_context_t* ctx, char** dest) {
  int result = skip_until_field(ctx);
  if (result < 0) return result;
  
  if (*ctx->cursor != '\"') return parse_ctx_error(ctx, CPE_PARSE_QUOTE, &ctx->line, "was expecting a quoted string, starting with '\"', found '%c'", *ctx->cursor);

  line_t line = ctx->line;
  const char* start = (ctx->cursor) + 1;
  unsigned escapes = 0;
  while(1) {
    if (*++(ctx->cursor) == '\0')
      return parse_ctx_error(ctx, CPE_UNEXPECTED, &line, "reached end of file, without finding the closing '\"'");

    if (*ctx->cursor == '"') {
      (ctx->cursor)++;
      break;
    }

    if (*ctx->cursor == '\n') {
      parse_ctx_newline(ctx);
      continue;
    }

    if (*ctx->cursor != '\\') {
      continue;
    }

    if (*(ctx->cursor + 1) == '\0')
      return parse_ctx_error(ctx, CPE_UNEXPECTED, &ctx->line, "reached end of file, while processing escape '\\'");

    if (*(ctx->cursor + 1) != '"' && *(ctx->cursor + 1) != '\\')
      return parse_ctx_error(ctx, CPE_PARSE_QUOTE, &ctx->line, "escape sequence '\\%c' is unknown - only \\\\ and \\\" supported", *(ctx->cursor + 1));

    /* we found a valid escape sequence, skip it. */
    escapes++;
    (ctx->cursor)++;
  }

  if (!dest) return CPE_SUCCESS;

  if (!escapes) {
    *dest = strndup(start, ctx->cursor - start - 1);
    return CPE_SUCCESS;
  }

  /* Code here can assume that the input is correct. */
  *dest = malloc(ctx->cursor - start - escapes);
  for (char* cursor = *dest; start < ctx->cursor - 1; ++start) {
    if (*start == '\\') {
      *cursor++ = *++start;
    } else {
      *cursor++ = *start;
    }
  }
  return CPE_SUCCESS;
}

/* parse_string parses a string.
 *
 * The string can either be in quotes, like "foo bar", or just be a naked
 * string, with no quotes. When the string has no quotes, parsing stops at
 * the first whitespace character. */
error_code_e parse_string(parse_context_t* ctx, char** dest) {
  int result = skip_until_field(ctx);
  if (result < 0) return result;

  if (*ctx->cursor == '\"')
    return parse_quoted_string(ctx, dest);

  const char* start = ctx->cursor;
  for(; *ctx->cursor && !isspace(*ctx->cursor); ++(ctx->cursor))
    ;

  if (dest) *dest = strndup(start, ctx->cursor - start);
  return CPE_SUCCESS;
}

error_code_e parse_uint64(parse_context_t* ctx, uint64_t limit, uint64_t* dest) {
  int result = skip_until_field(ctx);
  if (result < 0) return result;

  if (!isdigit(*ctx->cursor) && *ctx->cursor != '+')
    return parse_ctx_error(ctx, CPE_PARSE_INT, &ctx->line, "was expecting a digit, found '%c'", *ctx->cursor);

  static_assert(sizeof(unsigned long long int) >= sizeof(uint64_t),
    "strtoull on your system does not support 64 bit integers");

  unsigned long long int parsed = strtoull(ctx->cursor, (char**)(&ctx->cursor), 0);
  if (*ctx->cursor && !isspace(*ctx->cursor)) {
    return parse_ctx_error(ctx, CPE_PARSE_INT, &ctx->line, "was expecting a number, found invalid '%c'", *ctx->cursor);
  }

  if (parsed > limit) {
    return parse_ctx_error(ctx, CPE_PARSE_INT, &ctx->line, "specified number is too large (max: %"PRIu64")", limit);
  }

  if (dest) *dest = (uint64_t)parsed;
  return CPE_SUCCESS;
}

error_code_e parse_int64(parse_context_t* ctx, int64_t min, int64_t max, int64_t* dest) {
  int result = skip_until_field(ctx);
  if (result < 0) return result;

  if (!isdigit(*ctx->cursor) && *ctx->cursor != '+' && *ctx->cursor != '-')
    return parse_ctx_error(ctx, CPE_PARSE_INT, &ctx->line, "was expecting a digit, found '%c'", *ctx->cursor);

  static_assert(sizeof(unsigned long long int) >= sizeof(uint64_t),
    "strtoull on your system does not support 64 bit integers");

  long long int parsed = strtoll(ctx->cursor, (char**)(&ctx->cursor), 0);
  if (*ctx->cursor && !isspace(*ctx->cursor)) {
    return parse_ctx_error(ctx, CPE_PARSE_INT, &ctx->line, "was expecting a number, found invalid '%c'", *ctx->cursor);
  }

  if (parsed < min || parsed > max) {
    return parse_ctx_error(ctx, CPE_PARSE_INT, &ctx->line, "specified number is outside valid range (min:%"PRId64", max: %"PRId64"): %"PRId64, min, max, parsed);
  }

  if (dest) *dest = (int64_t)parsed;
  return CPE_SUCCESS;
}



/* parse_section parses and executes the supplied statements.
 *
 * parse_section will stop the parsing and return success when either
 * the end of the buffer is reached, or the first unknown statement is
 * encountered.
 *
 * Returns -1 whenever a recognized statement is encountered that however
 * has invalid parameters or configurations. */
error_code_e parse_section(parse_context_t* ctx, statement_t* language, void* dest) {
  bool command = true; /* indicates if we are expecting to find a command. */

  /* Initialize the per-statement parsing state. */
  int statements = 0; /* Total # of statements. */
  int required = 0; /* Total # of required statements. */
  for (; language[statements].match.name != NULL; ++statements)
    if (language[statements].options & OPT_MUST)
      required += 1;
  enum {
    STATE_SEEN = 1 << 0, /* If set, the command was seen before. */
  };
  uint8_t state[statements];
  memset(state, 0, sizeof(*state) * statements);

  int status = CPE_COMMAND; /* What to return if a command cannot be found. */
  int executed = 0; /* Number of statements successfully executed. */
  while (*ctx->cursor) {
    skip_line_spaces(ctx);

    if (*ctx->cursor == '\n') {
      parse_ctx_newline(ctx);
      (ctx->cursor)++;
      command = true;
      continue;
    }

    /* Could be a \r, or a \v, ... */
    if (isspace(*ctx->cursor)) {
      (ctx->cursor)++;
      continue;
    }

    if (*ctx->cursor == '#') {
      skip_until_eol(ctx);
      continue;
    }

    if (!command) {
      return parse_ctx_error(ctx, CPE_UNEXPECTED, &ctx->line, "'%16s...' is being parsed as command", ctx->cursor);
    }

    const char* start = ctx->cursor;
    for (; *ctx->cursor && !isspace(*ctx->cursor); ++(ctx->cursor))
      ;

    int plen = ctx->cursor - start; /* Parsed length. */
    for (int statement = 0; ; ++statement) {
      statement_t* s = &language[statement];
      uint8_t* flags = &state[statement];

      if (s->match.name == NULL) {
	ctx->cursor = start;
	if (required)
	  return parse_ctx_error(ctx, CPE_REQUIRED, &ctx->line, "%d mandatory commands were not specified", required);

	return status;
      }

      if (!*s->match.name || (plen == s->match.len && !memcmp(start, s->match.name, plen))) {
	if (executed && s->options & OPT_START) {
	  ctx->cursor = start;
          return CPE_COMMAND;
	}

	if ((*flags) & STATE_SEEN) {
  	  if (!(s->options & OPT_MULTI)) {
	    if (required)
	      return parse_ctx_error(ctx, CPE_REQUIRED, &ctx->line,"%d mandatory options were not specified", required);

	    ctx->cursor = start;
	    return CPE_REPEATED;
	  }
	} else {
	  (*flags) |= STATE_SEEN;
	  if (s->options & OPT_MUST)
	    required--;
        }

	int result = s->parse.function(ctx, start, &s->parse.data, dest);
	/* There are 4 possible outcomes from a parse function:
	 *
	 * 1) There was some real error, that the code couldn't really handle.
	 *
	 *    Result is < 0, and != CPE_COMMAND and != CPE_REPEATED.
	 *
	 * 2) It processed the command and all its arguments, and possibly more
	 *    statements. Processing is now complete.
	 *
	 *    Result is 0, cursor may or may not have moved forward (depending
	 *    on arguments), no more commands are expected until EOL (command=false).
	 *
	 * 3) It processed the command and a bunch of other statements, but it
	 *    got to a point where the next statement was unknown (or repeated).
	 *    Nothing more it can do. 
	 *
	 *    Result is CPE_COMMAND (unknown command) or CPE_REPEATED, the
	 *    cursor moved forward, as a few statements were processed, cursor
	 *    is already on the next command (command=true).
	 *
	 * 4) It turns out the command is unknown to the parser after all, cannot
	 *    really understand or parse it. Processing did not move forward.
	 *
	 *    Result is CPE_COMMAND (unknown command) or CPE_REPEATED, and the
	 *    cursor is still stuck where it was before. We need to look for
	 *    the next parsing function in the list, as none of the ones before
	 *    succeeded. */
	if (result < 0 && result != CPE_COMMAND && result != CPE_REPEATED) return result;

	if (result >= 0) {
	  status = CPE_COMMAND;
	  executed += 1;
	  command = false;
	  break;
	}

	status = result;
	if (ctx->cursor != start) {
	  executed =+ 1;
	  break;
	}
      }
    }
  }
  return CPE_SUCCESS;
}

error_code_e parse_buffer(const char* buffer, statement_t* language, void* dest, perror_t* err) {
  parse_context_t ctx = context_from_buffer(buffer, err);
  int result = parse_section(&ctx, language, dest);
  if (result < 0) {
    if (err == NULL) return result;

    if (result == CPE_COMMAND)
      return parse_ctx_error(&ctx, CPE_COMMAND, &ctx.line, "unknonw command found around '%16s...'", ctx.cursor);
    if (result == CPE_REPEATED)
      return parse_ctx_error(&ctx, CPE_REPEATED, &ctx.line, "command can only appear once");

    return result;
  }

  if (*ctx.cursor != '\0')
    return parse_ctx_error(&ctx, CPE_UNEXPECTED, &ctx.line, "unknonw parameter found around '%16s...'", ctx.cursor);

  return 0;
}

error_code_e parse_file(const char* path, statement_t* language, void* dest, perror_t* err) {
  char* buffer = NULL;
  if (read_file(path, &buffer) < 0) {
    return error(err, CPE_READ, "error reading %s: %s", path, strerror(errno));
  }

  int result = parse_buffer(buffer, language, dest, err);
  free(buffer);
  return result;
}
