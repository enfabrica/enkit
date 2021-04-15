#ifndef CONFPARSE_H_
#define CONFPARSE_H_

/* Any value < 0 is an error. */
typedef enum {
	CPE_SUCCESS = 0, /* No error - everything is good. */
	CPE_FAILURE = -1, /* Generic/Unspecified error. */
	/* Something happened in the library internals (snprintf, malloc, ...). */
	CPE_INTERNAL = -2,
	CPE_READ = -3, /* Could not read file (disk error, read, ...). */
	/* The wrong thing was found (was expecting a field, found \0). */
	CPE_UNEXPECTED = -4,
	CPE_COMMAND = -5, /* Command is unknown. */
	CPE_REPEATED = -6, /* Command repated, when allowed only once. */
	CPE_REQUIRED = -7, /* Required command was not found. */
	CPE_PARSE_INT = -8, /* Could not parse an integer. */
	CPE_PARSE_QUOTE = -9, /* Could not parse quotes. */
	CPE_PARSE_BOOL = -10, /* Could not parse bool. */
	/* Use values < -100 to define custom errors. */
	CPE_CUSTOM_START = -100,
} error_code_e;

/* perror_t represents an error message.
 * Initialize with perror_init() always free it with perror_free().
 * If message is != NULL, there is an error messsage. */
typedef struct {
	char *message;
} perror_t;

static inline perror_t perror_init();
static inline void perror_free(perror_t *perror);

/* assign_bits64 is an utility function able to set a few bits in the
 * middle of another integer.
 *
 * mask indicates which bits to set.
 * source contains the bits to copy.
 * dest is the destination integer.
 *
 * The modified value is returned. */
static inline uint64_t assign_bits(uint64_t dest, uint64_t source,
				   uint64_t mask);

typedef struct {
	const char *start; /* Start of the line being processed. */
	int number; /* Line number, starting from 0. */
} line_t;

typedef struct {
	const char *cursor;

	line_t line;
	perror_t *err;
} parse_context_t;

typedef union {
	size_t offset;
	void *ptr;

	void *ptrs[8];
	uint64_t ints[8];
} types_u;

typedef error_code_e (*parse_f)(parse_context_t *ctx, const char *start,
				types_u *data, void *dest);

typedef struct {
	parse_f function;
	types_u data;
} parse_t;

typedef struct {
	const char *name;
	size_t len;
} match_t;

typedef enum {
	/* No specific options.
   * This means that the statement can appear once, and is optional. */
	OPT_NONE = 0,
	/* The statement MUST be supplied in the config - not optional. */
	OPT_MUST = 1 << 0,
	/* The statement can appear multiple times - new values override old. */
	OPT_MULTI = 1 << 1,
	/* This statement always starts a new section.
   * Causes the parser to create a new section, unless the statement is first. */
	OPT_START = 1 << 2,
} options_e;

typedef struct {
	uint32_t options;
	match_t match;
	parse_t parse;
} statement_t;

#define STATEMENTS_END                                                         \
	{                                                                      \
		OPT_NONE,                                                      \
		{                                                              \
			.name = NULL                                           \
		}                                                              \
	}

static inline parse_context_t context_from_buffer(const char *buffer,
						  perror_t *err);

/* match_exact instructs the parser to look for a command by the specified name. */
static inline match_t match_exact(const char *name);
static inline match_t match_any();

extern parse_t expect_nothing();
extern parse_t expect_string(size_t offset);

/* expect_bool32 parses a "True/true/yes/on" or "False/false/no/off" string into a bit.
 *
 * Based on the string, it will set the bit indicated by the flipbit mask
 * accordingly. Additionally, if seenbit is != 0, the corresponding bits will
 * be set to 1 in the mask whenever a value is stored, indicating that the
 * value was set. This is useful to distinguish between default values,
 * and values explicitly set by the user. */
extern parse_t expect_bool32(size_t offset, uint32_t seenbit, uint32_t flipbit);
extern parse_t expect_bool64(size_t offset, uint64_t seenbit, uint64_t flipbit);

extern parse_t expect_int8(size_t offset);
extern parse_t expect_uint8(size_t offset);
extern parse_t expect_int16(size_t offset);
extern parse_t expect_uint16(size_t offset);
extern parse_t expect_int32(size_t offset);
extern parse_t expect_uint32(size_t offset);
extern parse_t expect_int64(size_t offset);
extern parse_t expect_uint64(size_t offset);

typedef void *(*adder_f)(void *);
extern parse_t expect_section(statement_t *statements, adder_f adder);

extern error_code_e parse_buffer(const char *buffer, statement_t *language,
				 void *dest, perror_t *err);
extern error_code_e parse_file(const char *path, statement_t *language,
			       void *dest, perror_t *err);

extern error_code_e adapter_bool32(parse_context_t *ctx, const char *start,
				   types_u *data, void *dest);
extern error_code_e adapter_bool64(parse_context_t *ctx, const char *start,
				   types_u *data, void *dest);
extern error_code_e adapter_uint64(parse_context_t *ctx, const char *start,
				   types_u *data, void *dest);
extern error_code_e adapter_uint32(parse_context_t *ctx, const char *start,
				   types_u *data, void *dest);
extern error_code_e adapter_uint16(parse_context_t *ctx, const char *start,
				   types_u *data, void *dest);
extern error_code_e adapter_uint8(parse_context_t *ctx, const char *start,
				  types_u *data, void *dest);
extern error_code_e adapter_int64(parse_context_t *ctx, const char *start,
				  types_u *data, void *dest);
extern error_code_e adapter_int32(parse_context_t *ctx, const char *start,
				  types_u *data, void *dest);
extern error_code_e adapter_int16(parse_context_t *ctx, const char *start,
				  types_u *data, void *dest);
extern error_code_e adapter_int8(parse_context_t *ctx, const char *start,
				 types_u *data, void *dest);

extern error_code_e adapter_string(parse_context_t *ctx, const char *start,
				   types_u *data, void *dest);
extern error_code_e adapter_section(parse_context_t *ctx, const char *start,
				    types_u *data, void *dest);

extern error_code_e parse_bool32(parse_context_t *ctx, uint32_t seenbit,
				 uint32_t flipbit, uint32_t *dest);
extern error_code_e parse_bool64(parse_context_t *ctx, uint64_t seenbit,
				 uint64_t flipbit, uint64_t *dest);
extern error_code_e parse_uint64(parse_context_t *ctx, uint64_t max,
				 uint64_t *dest);
extern error_code_e parse_int64(parse_context_t *ctx, int64_t min, int64_t max,
				int64_t *dest);
extern error_code_e parse_quoted_string(parse_context_t *ctx, char **dest);
extern error_code_e parse_string(parse_context_t *ctx, char **dest);
extern error_code_e parse_section(parse_context_t *ctx, statement_t *language,
				  void *dest);

extern void skip_line_spaces(parse_context_t *ctx);
extern void skip_until_eol(parse_context_t *ctx);
extern error_code_e skip_until_field(parse_context_t *ctx);

extern error_code_e parse_ctx_error(parse_context_t *ctx, error_code_e code,
				    const line_t *line, const char *fmt, ...);
extern void parse_ctx_newline(parse_context_t *ctx);

extern error_code_e error(perror_t *error, error_code_e code, const char *fmt,
			  ...);

/***************************************************************************/
/****************** Some inline definitions. *******************************/

static inline match_t match_exact(const char *name)
{
	match_t match = {
		.name = name,
		.len = strlen(name),
	};
	return match;
}

static inline match_t match_any()
{
	match_t match = {
		.name = "",
		.len = 0,
	};
	return match;
}

static inline parse_context_t context_from_buffer(const char *buffer,
						  perror_t *err)
{
	parse_context_t context = {
    .cursor = buffer,
    .line = {
      .start = buffer,
      .number = 0,
    },
    .err = err,
  };
	return context;
}

static inline void perror_free(perror_t *perror)
{
	if (!perror)
		return;

	free(perror->message);
	perror->message = NULL;
}

static inline perror_t perror_init()
{
	perror_t error = { .message = NULL };
	return error;
}

static inline uint64_t assign_bits(uint64_t dest, uint64_t source,
				   uint64_t mask)
{
	return dest ^ ((dest ^ source) & mask);
}

#endif
