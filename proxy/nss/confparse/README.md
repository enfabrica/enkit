# confparse

`confparse` is a tiny C library to parse configuration files similar
to `ssh_config` or `sshd_config` into C structs.

Compared to other libraries:

* Self contained, depends only on basic C library functions, ~20 of them.
* < 600 lines of actual code.
* Supports sections, and recursive parsing.
* Defines a read-only static parsing tree at compile time, that can
  be reused over and over for parsing.

Additionally:
* It is trivial to use.
* Has clear error messages.
  Provides the context of the error, line and character.
* Is trivial to modify and trim down.
* Comes with some good unit level testing.

To use it in your own project, you can easily copy `confparse.h`
and `confparse.c` in your tree, and build them with your favourite
build system.

The code is clean to compile with:

    gcc -D_POSIX_C_SOURCE=200809L -pedantic -Wall -std=c17

(yes, we favour modern versions of the language and features,
at time of writing, c17 is at least 4 years old).

# Using the library

Using the library is trivial. Check the [unit test](confparse_test.c)
for some examples.

In short, something like this:

    #include "confparse.h"

    typedef struct {
      char* path;
      char* user;
      char* host;

      uint32_t port;
    } my_config_t;

    void my_function() {
      statement_t language[] = {
        { OPT_NONE, match_exact("Path"), expect_string(offsetof(my_config_t, path)) },
        { OPT_NONE, match_exact("User"), expect_string(offsetof(my_config_t, user)) },
        { OPT_NONE, match_exact("Host"), expect_string(offsetof(my_config_t, host)) },
        { OPT_NONE, match_exact("Port"), expect_uint32(offsetof(my_config_t, port)) },
      }

      perror_t error = perror_init();
      my_config_t config = {NULL};

      int status = parse_file("/etc/module.conf", language, &config, &error);

      if (status < 0) {
        fprintf(stderr, "error parsing file: %s", error.message);
      }

      config_free(&config); // You'll have to write this one.
      perror_free(error);   // Yes, safe to call even without errors.
    }

Will accept configuration files that look like:

      # Comments are fine on a line.
    User carlo # .. or at the end of the line.
    Path "/home/data/My Movies" # Quoting is fine for strings, \\ escapes.
    Host this@is-an-host
    Port 0x10 # hex, decimal, or octal are supported.

By using `OPT_` modifiers, you can tune the behavior.

With the language definition above, for example, a config like this
would also be accepted:

    User carlo

... or even an empty file, as no statement is marked as mandatory with
`OPT_MUST`. While a config like this:

    User carlo
    User carlo

would be rejected, as `User` is not marked with `OPT_MULTI`.

Parsing a buffer is also really easy, just use:

    parse_buffer(buffer, language, &config, &error);

... same as above!


# A more complex example

Let's say you need to support "configuration blocks": for example,
you have different settings for different directories, or users.

You want your configuration file to look like:

    Directory /home/test
      Scan true
      MaxFile 64000
      Owner carlo

    Directory /home/bar
      Scan false
      Owner mario

    Default
      Scan false
      Owner mario

Your language could look like this:

    enum {
      ENABLE_SCAN = 1,
    };
    typedef struct {
      uint32_t flags;

      char* directory;
      char* owner;
      uint32_t max;
    } my_directory_t;

    typedef struct {
      my_directory_t defaults; /* Defaults from Default section. */

      my_directory_t* dir; /* Each directory is added here. */
      int dirn;            /* Number of directories. */
    } my_config_t;

    statement_t options[] = {
        { OPT_NONE, match_exact("Scan"), expect_bool32(offsetof(my_config_t, flags), 0, ENABLE_SCAN) },
        { OPT_NONE, match_exact("MaxFile"), expect_uint32(offsetof(my_config_t, max)) },
        { OPT_NONE|OPT_MUST, match_exact("Owner"), expect_string(offsetof(my_config_t, owner)) },
    };

    statement_t directory[] = {
        { OPT_NONE, match_exact("Directory"), expect_string(offsetof(my_config_t, directory)) },
        { OPT_NONE|OPT_MUST, match_any(), expect_section(options, NULL) },
    };

    statement_t defaults[] = {
        { OPT_NONE, match_exact("Default"), expect_nothing() },
        { OPT_NONE|OPT_MUST, match_any(), expect_section(options, NULL) },
    };

    statement_t language[] = {
        { OPT_NONE, match_exact("Default"), expect_section(directory, (adder_f)(my_config_add_default) },
        { OPT_NONE|OPT_MULTI, match_exact("Directory"), expect_section(defaults, (adder_f)(my_config_add_dir)) },
    };

Now, you may wonder...

* Why the two levels of nesting? Well, we need to tell the parsing library how to parse the `Directory` and
  `Default` statements themselves, and that parsing is different in the two objects.

  By using `OPT_START` you can even create sections implicitly, the first time a specific statement is
  encountered.

* What are `my_config_add_default` and `my_config_add_dir`? They are two simple functions that given
  a `my_config_t` object are in charge of adding the `section` described. For example:

    void* my_config_add_default(void* config) {
      my_config_t* real = config;
      return &real->defaults;
    }

    void* my_config_add_dir(void* config) {
      my_config_t* real = config;
      real->dir = realloc(real->dir, sizeof(my_directory_t) * (real->dirn + 1));
      my_directory_t* dir = real->dir[(real->dirn)++];
      memset(dir, 0, sizeof(*dir));
      return dir;
    }

If you peek through the code and tests, you will see more examples like this.
Happy hacking!

# A couple remarks

Adding custom types or extending the set of types supported is relatively
easy: define a parsing and storing function, and store your callback and
its state in a statement. Just follow the same pattern as other parser.

All the necessary functions and structs are exported in the .h file for
convenience.

# Sharp edges

The library assumes that it owns any value in the supplied config.
If you mark a string as `OPT_MULTI`, newer values will overwrite old ones,
as is typical in configuration files.

To avoid memory leask, the library will of course `free()` any previous value.
So:
* Make sure you always initialize your config to 0.
* Don't assign string constants as defaults. 
