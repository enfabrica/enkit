#!/usr/bin/python3.12
"""gee, Rewriten in python.

Goals:
    * run directly anywhere:
        * use only the standard python library
        * monolithic utility (no support files)
    * respect a user's .gee.rc file
    * feature for feature backwards compatibility with the shell version of gee
    * show every command executed in a subshell, show all results
        * optionally?  --quiet suppresses commands
        * optionally?  --verbose shows all output of commands
    * be agnostic towards enfabrica-isms (defer to config file)

Environment variables gee cares about:
    GHUSER: used if gee.ghuser isn't set in the config file
    SSH_AUTH_SOCK: indicates a running ssh-agent
"""

# standard library
import argparse
import copy
import datetime
import difflib
import glob
import io
import json
import logging
import logging
import os
import pathlib
import pty
import re
import shlex
import shutil
import subprocess
import sys
import textwrap
import threading
import types
from typing import List, Optional

#####################################################################
## Import toml or tomllib, depending on which is available.
#####################################################################
try:
    import tomllib

    toml = None
except ImportError:
    import toml

    tomllib = None

#####################################################################
## Utility functions
#####################################################################

# Command Priorities
LOW = 1
HIGH = 2


def q(*args, **kwargs):
    """A very short alias for shlex.quote.

    We use shlex.quote everywhere, and it's convenient to have something
    terse to embed in f-strings.

    TODO: maybe "from shlex import quote as q" is better?
    """
    return shlex.quote(*args, **kwargs)


def _expand_path(path):
    """Expand a few special tokens in a path string.

    >>> os.environ["HOME"] = "/home/foo"
    >>> os.environ["USER"] = "foo"
    >>> _expand_path("~/file")
    '/home/foo/file'
    >>> _expand_path("$HOME/file.$USER.txt")
    '/home/foo/file.foo.txt'
    """

    path = path.replace("$HOME", os.environ["HOME"])
    if path.startswith("~/"):
        path = os.path.join(os.environ["HOME"], path[2:])
    path = path.replace("$USER", os.environ["USER"])
    return os.path.realpath(path)


def ask_yesno(prompt, default=True):
    """Ask the user a yes/no question."""
    yesno = " (Y/n)  " if default else " (y/N)  "
    while True:
        result = input(prompt + yesno)
        if result.lower() in ("y", "yes"):
            return True
        elif result.lower() in ("n", "no"):
            return False
        elif result == "":
            return default
        print(f"Invalid response: {result!r}")


def ask_multi(prompt, default=None, options=None):
    if not options:
        # extract options from prompt
        options = "".join(re.findall(r"\((.)\)", prompt.lower()))
    resp_char = None
    while True:
        resp = input(prompt).lower()
        if not resp:
            resp = default
        resp_char = None if resp is None else resp[0]
        if resp_char in options:
            return resp_char
            break
        print(f"Invalid response: {resp}")


#####################################################################
# Configuring gee
#####################################################################


class GeeConfig:
    """Responsible for loading and saving the user's .gee.rc file.

    TODO(jonathan): help the user create a default .gee.rc file on startup.

    TODO(jonathan): maybe move all of the repo-dict related methods here.
    """

    def __init__(self):
        self.path = None
        self.data = {}

    @staticmethod
    def _merge(d1, d2):
        for k in d2:
            if k not in d1:
                d1[k] = d2[k]
            else:
                if isinstance(d1[k], list):
                    # append:
                    d1[k].extend(d2[k])
                elif isinstance(d1[k], dict):
                    # merge:
                    d1[k] = GeeConfig._merge(d1[k], d2[k])
                else:
                    # override:
                    d1[k] = d2[k]  # override
        return d1

    def load(self, path):
        self.path = path
        if tomllib:
            with open(path, "rb") as fd:
                self.data = tomllib.load(fd)
                fd.close()
        else:
            with open(path, "r", encoding="utf-8") as fd:
                self.data = toml.load(fd)

    def save(self, path=None):
        print("Writing configuration file not yet supported.")
        return
        # TODO(jonathan): python3.11 replaced the "toml" library with
        # "tomllib", and took away the ability to write a toml file.
        # find a workaround. Maybe switch to yaml?  configparser?
        # if path is None:
        #     path = self.path
        # path = _expand_path(path)
        # with open(path, "w", encoding="utf-8") as fd:
        #     # was: toml.write()
        #     fd.close()
        # logging.debug("Saved config: %r", path)

    def validate(self):
        # TODO(jonathan)
        if not self.get("gee.ghuser", ""):
            print("Warning: gee.ghuser was not set")
            self.set("gee.ghuser", os.environ.get("GHUSER", None))
        pass

    def get(self, key, default=None):
        key_parts = key.split(".")
        data = self.data
        for key_part in key_parts:
            if key_part not in data:
                data = default
                break
            data = data[key_part]
        if data is not None:
            return data
        else:
            logging.fatal("Missing %r in configuration.", key, stacklevel=3)

    def set(self, key, value):
        key_parts = key.split(".")
        data = self.data
        for key_part in key_parts[:-1]:
            if key_part not in data:
                data[key_part] = {}
            data = data[key_part]
        data[key_parts[-1]] = value


#####################################################################
# Logging
#####################################################################


class GeeLogger(logging.Logger):
    # DEBUG=10, INFO=20, etc:
    DEBUG = 10
    LOW_STDOUT = 13  # the stdout of a non-essential executed command
    LOW_STDERR = 15  # the stderr of a non-essential executed command
    LOW_COMMANDS = 17  # the commandline for a non-essential executed command
    INFO = 20
    STDOUT = 23  # the stdout of an executed command
    STDERR = 25  # the stderr of an executed command
    COMMANDS = 27  # the commandline of an executed command
    WARNING = 30
    ERROR = 40
    CRITICAL = 50

    def cmd(self, msg, *args, **kwargs):
        self.log(GeeLogger.COMMANDS, msg, *args, **kwargs)

    def cmd_stdout(self, msg, *args, **kwargs):
        self.log(GeeLogger.STDOUT, msg, *args, **kwargs)

    def cmd_stderr(self, msg, *args, **kwargs):
        self.log(GeeLogger.STDERR, msg, *args, **kwargs)

    def low_cmd(self, msg, *args, **kwargs):
        self.log(GeeLogger.LOW_COMMANDS, msg, *args, **kwargs)

    def low_cmd_stdout(self, msg, *args, **kwargs):
        self.log(GeeLogger.LOW_STDOUT, msg, *args, **kwargs)

    def low_cmd_stderr(self, msg, *args, **kwargs):
        self.log(GeeLogger.LOW_STDERR, msg, *args, **kwargs)


class GeeLogFormatter(logging.Formatter):
    grey = "\x1b[38;20m"
    bold_grey = "\x1b[38;1m"
    white = "\x1b[97;20m"  # TODO(jonathan): fix this code
    bold_white = "\x1b[97;1m"  # TODO(jonathan): fix this code
    green = "\x1b[32;20m"
    yellow = "\x1b[33;20m"
    red = "\x1b[31;20m"
    bold_red = "\x1b[31;1m"
    black_on_white = "\x1b[30;107m\x1b[K"
    black_on_grey = "\x1b[30;47m\x1b[K"
    reset = "\x1b[0m"
    format = "%(levelname)s - %(message)s"

    FORMATS = {
        GeeLogger.DEBUG: logging.Formatter(grey + "DBG: %(message)s" + reset),
        GeeLogger.LOW_STDOUT: logging.Formatter(grey + "%(message)s" + reset),
        GeeLogger.LOW_STDERR: logging.Formatter(bold_grey + "%(message)s" + reset),
        GeeLogger.LOW_COMMANDS: logging.Formatter(
            black_on_grey + "%(message)s" + reset
        ),
        GeeLogger.INFO: logging.Formatter(green + "INFO: %(message)s" + reset),
        GeeLogger.STDOUT: logging.Formatter(grey + "%(message)s" + reset),
        GeeLogger.STDERR: logging.Formatter(bold_grey + "%(message)s" + reset),
        GeeLogger.COMMANDS: logging.Formatter(black_on_white + "%(message)s" + reset),
        GeeLogger.WARNING: logging.Formatter(yellow + "WARNING: %(message)s" + reset),
        GeeLogger.ERROR: logging.Formatter(red + "ERROR: %(message)s" + reset),
        GeeLogger.CRITICAL: logging.Formatter(
            bold_red + "CRITICAL ERROR@%(filename)s:%(lineno)d: %(message)s" + reset
        ),
    }

    def format(self, record):
        formatter = self.FORMATS.get(record.levelno)
        return formatter.format(record)


#####################################################################
# Subprocess execution
#####################################################################


class ExecutionContext:
    """An execution context for running subprocesses.

    Sometimes it is beneficial to execute a sequence of commands in
    a specific subdirectory, without necessarily changing the directory
    for the whole process.  The execution context keeps track of a cwd
    for use by all subprocesses.
    """

    def __init__(self, gee, git="git", gh="gh", cwd=None):
        self.gee = gee
        self.git = git
        self.gh = gh
        self.default_cwd = os.path.realpath(os.getcwd())
        self.cwd = cwd if cwd is not None else os.self.default_cwd

    def chdir(self, d):
        """Change the directory only for this execution context."""
        self.cwd = d

    def log_command(self, cmd, priority=HIGH, cwd=None):
        """Log a command at COMMAND priority, or LOW_COMMAND if quiet is true.

        priority=HIGH is for mainline commands that teach the user git.
        priority=LOW is for less essential commands (error checks, diagnostics) that
          aren't as useful for the user to see.
        """
        if cwd is None:
            cwd = self.cwd
        if isinstance(cmd, list) and not isinstance(cmd, str):
            cmd = " ".join([shlex.quote(x) for x in cmd])
        if cwd and cwd != self.default_cwd:
            # Inform the user which directory a command is being run from.
            cmd = f"{cwd}$ {cmd}"
        else:
            cmd = f"$ {cmd}"
        if priority == HIGH:
            self.gee.logger.cmd(cmd)
        else:
            self.gee.logger.low_cmd(cmd)

    def log_command_stdout(self, text, priority=HIGH):
        for line in text.splitlines(keepends=False):
            if priority == HIGH:
                self.gee.logger.cmd_stdout(line)
            else:
                self.gee.logger.low_cmd_stdout(line)

    def log_command_stderr(self, text, priority=HIGH):
        for line in text.splitlines(keepends=False):
            if priority == HIGH:
                self.gee.logger.cmd_stderr(line)
            else:
                self.gee.logger.low_cmd_stderr(line)

    def run(self: "Gee", *args, mode=None, **kwargs):
        """Run a subprocess.

        There are three flavors, or modes, of this run function.

        They are:

          * "interactive" mode attaches the subprocess directly
            to the user's console, so that commands that require
            user interaction (vim, etc) will function.

          * "non-interactive" mode is the default.  In this mode,
            the subprocess is run without user control, but a
            pseudo-tty is still attached so that git and gh can
            produce colorful, realtime output.

          * "no_tty" mode is for porcelain commands that require
            no user interaction, and we don't want git or gh
            inserting any control codes into the data stream.

        The following additional arguments are supported:

          * "priority" can be "HIGH" or "LOW".  A high priority
            command visible to the user at the normal (INFO) log
            level.  Low priority commands are extra checks that
            are usually invisible to the user, but can be
            revealed by changing the log level to DEBUG.

          * "cwd" can be used to override the current working
            directory for one command, otherwise the cwd of
            the ExecutionContext instance is used instead.

        All other subprocess.Popen kwargs operate as defined.
        """
        if mode == "interactive":
            return self.run_interactive(*args, **kwargs)
        elif mode == "no_tty":
            return self.run_no_tty(*args, **kwargs)
        else:
            return self.run_noninteractive(*args, **kwargs)

    def run_no_tty(
        self: "Gee", cmd, check=True, priority=HIGH, timeout=None, cwd=None, **kwargs
    ):
        """Run a subprocess in a simple way, without a pseudo-tty.

        Sometimes, we want to run a command with no pseudo-tty attached.  This is
        the easiest way to prevent git from inserting unwanted control characters
        into the byte stream.
        """
        if cwd is None:
            cwd = self.cwd

        self.log_command(cmd, priority=priority, cwd=cwd)

        process = subprocess.Popen(
            cmd,
            shell=True if isinstance(cmd, str) else False,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            encoding="utf-8",
            errors="utf-8",
            cwd=cwd,
            **kwargs,
        )

        process.wait(timeout=timeout)
        stdout = "".join(process.stdout)
        self.log_command_stdout(stdout, priority=priority)
        stderr = "".join(process.stderr)
        self.log_command_stderr(stderr, priority=priority)

        self.gee.logger.debug("exit status = %d", process.returncode)
        if check and process.returncode != 0:
            self.gee.fatal(
                "Command failed with return code = %d", process.returncode, stacklevel=3
            )

        return process.returncode, stdout, stderr

    def run_noninteractive(
        self: "Gee",
        cmd,
        check=True,
        priority=HIGH,
        timeout=None,
        capture=True,
        cwd=None,
        **kwargs,
    ):
        """Run a subprocess and capture it's output while streaming to the console.

        git and gh are tricked into interacting with a pseudo-tty, so that
        "fancy" output can be streamed to the user, without compromising the
        ability to capture the output of simple commands.
        """
        self.log_command(cmd, priority=priority, cwd=cwd)
        log_level = self.gee.logger.getEffectiveLevel()
        if priority == LOW:
            stdout_enabled = log_level <= GeeLogger.LOW_STDOUT
            stderr_enabled = log_level <= GeeLogger.LOW_STDERR
        else:
            stdout_enabled = log_level <= GeeLogger.STDOUT
            stderr_enabled = log_level <= GeeLogger.STDERR

        stdout_parent, stdout_child = pty.openpty()
        stderr_parent, stderr_child = pty.openpty()

        if cwd is None:
            cwd = self.cwd

        process = subprocess.Popen(
            cmd,
            shell=True if isinstance(cmd, str) else False,
            stdin=subprocess.DEVNULL,
            stdout=stdout_child,
            stderr=stderr_child,
            text=True,
            encoding="utf-8",
            errors="utf-8",
            cwd=cwd,
            bufsize=1,  # unbuffered output
            **kwargs,
        )
        os.close(stdout_child)
        os.close(stderr_child)

        stdout_bytes = bytearray()
        stderr_bytes = bytearray()

        def stream_and_capture(fd, is_stdout):
            nonlocal stdout_bytes, stdout_enabled, stderr_bytes, stderr_enabled
            while True:
                try:
                    output_bytes = os.read(fd, 1024)
                    if not output_bytes:
                        break
                    if is_stdout:
                        if capture:
                            stdout_bytes.extend(output_bytes)
                        if stdout_enabled:
                            os.write(
                                sys.stdout.fileno(), output_bytes
                            )  # stream to stdout (which is the tty)
                        sys.stdout.flush()  # ensure it is shown immediately.
                    else:
                        if capture:
                            stderr_bytes.extend(output_bytes)
                        os.write(
                            sys.stderr.fileno(), output_bytes
                        )  # stream to stdout (which is the tty)
                        sys.stderr.flush()  # ensure it is shown immediately.
                except OSError:
                    break

        stdout_thread = threading.Thread(
            target=stream_and_capture, args=(stdout_parent, True)
        )
        stderr_thread = threading.Thread(
            target=stream_and_capture, args=(stderr_parent, False)
        )
        stdout_thread.start()
        stderr_thread.start()
        process.wait(timeout=timeout)
        stdout_thread.join()
        stderr_thread.join()

        self.gee.logger.debug("exit status = %d", process.returncode)
        if check and process.returncode != 0:
            self.gee.fatal(
                "Command failed with return code = %d", process.returncode, stacklevel=3
            )

        os.close(stdout_parent)
        os.close(stderr_parent)

        stdout = bytes(stdout_bytes).decode("utf-8").strip()
        stderr = bytes(stderr_bytes).decode("utf-8").strip()
        return process.returncode, stdout, stderr

    def run_interactive(self: "Gee", cmd, check=True, priority=HIGH, cwd=None):
        """Run an interactive command that communicates with the console.

        For when we don't want to futz about with the multithreaded solution
        implemented in "run" above, and we're sure we don't need to capture
        the output of the command.

        Ignores log_level.
        """
        cwd = cwd if cwd else self.cwd
        if isinstance(cmd, list) and not isinstance(cmd, str):
            cmd = shlex.join(cmd)
        self.log_command(cmd, priority=priority)
        p = subprocess.Popen(
            cmd,
            # Everything gee runs is run through the shell,
            # so the user can copy/paste exactly:
            shell=True,
            stdin=sys.stdin,
            stdout=sys.stdout,
            stderr=sys.stderr,
            encoding="utf-8",
            errors="utf-8",
            cwd=cwd,
        )
        stdout, stderr = p.communicate()
        rc = p.wait()
        self.gee.logger.debug("exit status = %d", rc)
        if check and rc != 0:
            self.gee.fatal(
                "Command failed with returncode=%d: %s", rc, cmd, stacklevel=3
            )
        return rc

    def run_git(self, cmd, **kwargs):
        git = self.git
        if isinstance(cmd, str):
            cmd = git + " " + cmd
        elif isinstance(cmd, list):
            cmd = [git] + cmd
        else:
            raise TypeError("command is not a list or a string: %r", cmd)
        return self.run(cmd, **kwargs)

    def run_gh(self, cmd, **kwargs):
        gh = self.gh
        if isinstance(cmd, str):
            cmd = gh + " " + cmd
        elif isinstance(cmd, list):
            cmd = [gh] + cmd
        else:
            raise TypeError("command is not a list or a string: %r", cmd)
        a = self.run(cmd, **kwargs)
        return a


#####################################################################
# Codeowners
#####################################################################


class Codeowners:
    def __init__(self):
        self.rules = []

    def Load(self, path):
        with open(path, "r", encoding="utf-8") as fd:
            for line in fd:
                line = re.sub(r"\s*#.*", "", line).strip()
                if not line:
                    continue
                parts = line.split()
                self.AddRule(parts[0], parts[1:])
            fd.close()

    @classmethod
    def _PatternToRegexp(cls, expr):
        """Convert a .gitignore file pattern to regular expressions.

        >>> re1 = Codeowners._PatternToRegexp("*.h")
        >>> str(re1)
        "re.compile('^([^/]+/)*[^/]*\\\\\\\\.h(/.*)?$')"
        >>> bool(re1.match("foo.h"))
        True
        >>> bool(re1.match("bar/bee/bum.h"))
        True
        >>> bool(re1.match("bar/bee/.h"))
        True
        >>> bool(re1.match("bar/foo.h/bum"))
        True
        >>> re2 = Codeowners._PatternToRegexp("**/zap/**")
        >>> bool(re2.match("a/b/c/zap/foo/bum.zip"))
        True
        >>> bool(re2.match("a/b/c/zzzap/foo/bum.zip"))
        False
        >>> re3 = Codeowners._PatternToRegexp(".bazelrc")
        >>> str(re3)
        "re.compile('^([^/]+/)*\\\\\\\\.bazelrc(/.*)?$')"
        >>> bool(re3.match(".bazelrc"))
        True
        """
        # normalize the expression:
        if expr.endswith("/**"):
            # An expression ending with /** is equivalent to an expression
            # ending with /
            expr = expr[:-2]
        if "/" not in expr:
            # match any file or dir matching expr
            expr = "**/" + expr
        elif not (expr.startswith("/") or expr.startswith("**/")):
            expr = "**/" + expr
        if expr.startswith("/"):
            expr = expr[1:]
        expr_tokens = re.split(r"(\*\*\/)|(\*)|(\?)", expr)
        re_expr = "^"
        for token in expr_tokens:
            if token == "**/":
                re_expr += r"([^/]+/)*"  # zero or more paths
            elif token == "*":
                re_expr += r"[^/]*"  # any non-separator character
            elif token == "?":
                re_expr += r"."
            elif token:
                re_expr += re.escape(token)
        if re_expr.endswith("/"):
            re_expr += ".*$"  # anything can come after the final slash
        else:
            re_expr += "(/.*)?$"  # can be a file, or can be a subtree.
        r = re.compile(re_expr)
        return r

    def AddRule(self, expr, owners):
        re_expr = self._PatternToRegexp(expr)
        owners.sort()
        self.rules.append((re_expr, " ".join(owners)))

    def GetOwnersForFile(self, path):
        if path.startswith("/"):
            path = path[1:]
        owners = ""
        for r, o in self.rules:
            if r.match(path):
                logging.debug(f"matched: {r!r} {path!r}")
                owners = o
            else:
                logging.debug(f"   nope: {r!r} {path!r}")
        return owners

    def GetOwnersForFileSet(self, paths):
        owners_map = {}
        for path in paths:
            owners = self.GetOwnersForFile(path)
            if owners not in owners_map:
                owners_map[owners] = []
            owners_map[owners].append(path)
        return owners_map


#####################################################################
# Gee commands
#####################################################################


class GeeCommand:
    """Base class for adding commands to Gee.

    This class registers a subparser with the global argument
    parser, and provides a "dispatch" method for executing
    the command.  All subclasses of this class are automatically
    enrolled by the Gee constructor.

    Usually, this docstring will contain the documentation for the
    specific command.
    """

    COMMAND = None
    ALIASES = []

    def __init__(self, gee_obj: "Gee"):
        self.gee = gee_obj

        if self.COMMAND:
            shortdoc, longdoc = self.__doc__.split("\n\n", 1)
            longdoc = textwrap.dedent(longdoc)
            self.argparser = self.gee.subparsers.add_parser(
                self.COMMAND,
                aliases=self.ALIASES,
                formatter_class=argparse.RawDescriptionHelpFormatter,
                description=longdoc,
                help=shortdoc,
            )
            self.argparser.set_defaults(func=self.dispatch)

    def dispatch(self, args):
        print(f"DEBUG: {self.COMMAND}:dispatch({vars(args)!r})")
        raise NotImplementedError(self.COMMAND)


class HelpCommand(GeeCommand):
    """Show help.

    Help!  The problem!  I must have fruit!
    """

    COMMAND = "help"
    ALIASES = ["h"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument(
            "command", help="Command to get help for.", nargs="?"
        )

    def dispatch(self, args):
        command = args.command
        if command is None:
            self.gee.argparser.print_help()
            return 0
        elif command in self.gee.subcommands:
            self.gee.subcommands[command].argparser.print_help()
            return 0
        else:
            print(f"Invalid command: {command}")
            print()
            print(
                f"Supported commands: {' '.join(sorted(self.gee.subcommands.keys()))}"
            )
            return 1


class InitCommand(GeeCommand):
    """Initialize a gee environment.

    `gee init` creates a new gee-controlled workspace in the user's home
    directory.  The directory `~/gee/<repo>/main` will be created and
    populated, and all other branches will be checked out into
    `~/gee/<repo>/<branch>`.

    The init command will also attempt to ensure that the user's
    git and gh environments are configured in a correct, consistent
    manner.
    """

    COMMAND = "init"
    ALIASES = ["start", "initialize"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument(
            "url",
            help="The https/ssh-based URL for the git repository to clone.",
            default=None,
            nargs=None,
        )
        self.argparser.add_argument(
            "-m",
            "--main_branch",
            help="The name of the branch to use as the default main branch, if the default is not desired.",
            default=None,
            nargs=None,
        )

    def dispatch(self: "Gee", args):
        self.gee.install_tools()

        # Create gee directory if needed
        self.gee.create_gee_dir()

        # Check access to the github API.
        self.gee.check_gh_auth()

        # Check ssh access to github.
        if not self.gee.check_ssh():
            self.gee.ssh_enroll()

        # Clone the remote repo
        self.gee.parse_url(args.url, args.main_branch)
        self.gee.make_fork()
        self.gee.clone()

        # Configure git.
        self.gee.configure()

        # Save the .gee.rc file, creating it if it's missing.
        self.gee.save_config()

        self.gee.info(
            "Initialized gee workspace: %s/%s",
            self.gee.repo_dir(),
            self.gee.main_branch(),
        )


class MakeBranchCommand(GeeCommand):
    """Create a new branch.

    Creates a new branch and worktree directory.

    If no `parent` argument is provided, a branch based on the current branch
    will be created.
    """

    COMMAND = "make_branch"
    ALIASES = ["mkbr", "branch"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument("branch", help="Name of branch to create.")
        self.argparser.add_argument(
            "parent",
            help="Branch to use as parent for this branch.",
            nargs="?",
            default=None,
        )

    def dispatch(self, args):
        self.gee.make_branch(args.branch, args.parent)
        return 0


class LogCommand(GeeCommand):
    """A pretty version of log.

    This command creates the "logp" git alias, if it doesn't
    already exist, and then executes the rest of the command
    line as if "git logp" had been executed.
    """

    COMMAND = "log"
    ALIASES = []

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument(
            "args", nargs=argparse.REMAINDER, help="git logp arguments."
        )

    def dispatch(self, args):
        self.gee.configure_logp_alias()
        if args.args[0] == "--":
            _ = args.args.pop(0)
        self.gee.xc().run_git(["logp"] + args.args)


class ConfigCommand(GeeCommand):
    """Change configurations.

    Valid configuration options are:

    * "default": Reset to default settings.
    * "enable_vim": Set "vimdiff" as your diff/merge tool.
    * "enable_nvim": Set "nvimdiff" as your diff/merge tool.
    * "enable_emacs": Set "emacs" as your diff/merge tool.
    * "enable_vscode": Set "vscode" as your GUI diff/merge tool.
    * "enable_meld": Set "meld" as your GUI diff/merge tool.
    * "enable_bcompare": Set "BeyondCompare" as your GUI diff/merge tool.
    """

    COMMAND = "config"
    ALIASES = ["configure"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument("option", help="Configuration option to select")

    def dispatch(self, args):
        if args.option in (
            "default",
            "defaults",
            "vim",
            "vimdiff",
            "enable_vim",
            "enable_vimdiff",
        ):
            self.gee.config.set("gee.mergetool", "vim")
        elif args.option in ("nvim", "nvimdiff", "enable_nvim", "enable_nvimdiff"):
            self.gee.config.set("gee.mergetool", "nvim")
        elif args.option in ("code", "vscode", "enable_code", "enable_vscode"):
            self.gee.config.set("gee.mergetool", "vscode")
        elif args.option in ("meld", "enable_meld"):
            self.gee.config.set("gee.mergetool", "meld")
        elif args.option in (
            "bcompare",
            "enable_bcompare",
            "beyondcompare",
            "enable_beyondcompare",
        ):
            self.gee.config.set("gee.mergetool", "bcompare")
        else:
            self.gee.error("Unsupported configuration option: %s", args.option)
            return 1

        self.gee.configure()
        self.gee.save_config()


class DiffCommand(GeeCommand):
    """Compare the current branch with the parent branch.

    Shows all local change since this branch diverged from its parent branch.

    If <files...> are omitted, shows changes to all files.
    """

    COMMAND = "diff"
    ALIASES = []

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument("files", nargs="*", help="git logp arguments.")

    def dispatch(self, args):
        branch = self.gee.get_current_branch()
        parent_commit = self.gee.get_parent_commit(branch)
        self.gee.xc().run_git(["diff", f"{parent_commit}...HEAD", "--"] + args.files)


class VimdiffCommand(GeeCommand):
    """compare file(s) against the parent branch version.

    This command invokes the currently configured difftool (typically,
    "vimdiff") to show all local changes since this branch diverged from its
    parent branch.
    """

    COMMAND = "vimdiff"
    ALIASES = []

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument("files", nargs="+", help="File(s) to compare.")

    def dispatch(self, args):
        branch = self.gee.get_current_branch()
        parent = self.gee.get_parent_branch(branch)
        _, stdout, _ = self.gee.xc().run_git(
            f"config --global --get diff.difftool", priority=LOW
        )
        difftool = stdout.strip()
        files = " ".join([shlex.quote(x) for x in args.files])
        self.gee.xc().run_interactive(f"git difftool -t {difftool} {parent} -- {files}")


class FindCommand(GeeCommand):
    """Finds a file by name in the current branch.

    Searches the current branch for a file whose name matches the specified
    expression.  Will initially search for the file without traversing symlinks.
    If it fails to find any files, it will search again in the bazel-bin
    subdirectory (and follow symlinks) to see if the file is generated by a bazel
    rule.

    Roughly equivalent to running:

        find -L "$(git rev-parse --show-toplevel)" -name .git -prune -or \
             -name "${expression}" -print

    Example of use:

        gee find WORKSPACE
    """

    COMMAND = "find"
    ALIASES = []

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument("expression", help="Filename-ish to search for.")

    def dispatch(self, args):
        current_branch = self.gee.get_current_branch()
        path = self.gee.branch_dir(current_branch)
        paths = (path, f"{path}/bazel-bin/*", f"{path}/bazel-{current_branch}/*")
        for p in paths:
            _, stdout, _ = self.gee.xc().run(
                f"find {q(p)} -name .git -prune -or -name {q(args.expression)} -print",
                check=False,
            )
            if stdout.strip() != "":
                break


class CommitCommand(GeeCommand):
    """Commit and push changes.

    Commits changes to your local branch, and then pushes your commits to
    `origin`.

    Note that if you are working in a PR-associated branch created with `gee
    pr_checkout`, your commits will be pushed to your `origin` remote, and the
    remote PR branch.  To contribute your changes back to another user's PR branch,
    use the `gee pr_push` command.

    TODO(jonathan):
    Unless GEE_ENABLE_PRESUBMIT_CANCEL feature is disabled, gee
    will check to see if pushing the current commit will invalidate a presubmit job
    in the `pending` state.  If this is the case, gee will kill the previous
    presubmit before pushing the changes and thus kicking off the new presubmit.

    Example:

        gee commit -m "Added \"gee commit\" command."

    See also:

    * pr_push
    """

    COMMAND = "commit"
    ALIASES = ["c"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument(
            "-a",
            "--all",
            default=False,
            action="store_true",
            help="Automatically stage all added, modified, or deleted files.",
        )
        self.argparser.add_argument(
            "--amend",
            default=False,
            action="store_true",
            help="Amend the previous commit.",
        )
        self.argparser.add_argument(
            "-f",
            "--force",
            default=False,
            action="store_true",
            help="Force-push to origin after commit.",
        )
        self.argparser.add_argument(
            "-m", "--msg", help="Commit message to use.", default=None
        )

    def dispatch(self, args):
        branch = self.gee.get_current_branch()
        if branch == self.gee.main_branch():
            self.gee.infos(
                f"""\
                gee's workflow doesn't allow changes to the {self.gee.main_branch()} branch.
                You should move your changes to another branch.  For example:
                    git add -A; git stash; gcd -m new1; git stash apply
                """
            )
            self.gee.fatal("Preventing commit to %s branch", branch)

        git = self.gee.find_binary(self.gee.config.get("gee.git", "git"))
        cmd = [git, "commit"]
        if args.amend:
            cmd += ["--amend"]
        if args.all:
            self.gee.xc().run_git("add --all")
            cmd += ["-a"]
        if args.msg:
            cmd += ["-m", args.msg]
        rc = self.gee.xc().run_interactive(cmd, check=False)
        if rc == 0:
            # TODO(jonathan): how do we cancel presubmit jobs in a generic way?
            if not (args.amend or args.force):
                self.gee.check_origin(branch)
                self.gee.xc().run_git(f"push --quiet -u origin {branch}")
            else:
                self.gee.xc().run_git(f"push --quiet --force -u origin {branch}")
        else:
            # git commit will fail if there are no local changes.
            self.gee.debug("git commit operation returned rc=%d", rc)
            return

        # check if this is a branch of a PR:
        parent = self.gee.get_parent_branch(branch)
        if parent.startswith("upstream/refs/pull/"):
            self.gee.infos(
                """\
              NOTE: This is a branch of another user's PR.  Your changes were pushed to *your*
                    github fork.  To push your changes back into the other user's PR, you need
                    to use the \"gee pr_push\" command."
              """
            )


class UpdateCommand(GeeCommand):
    """Rebase the current branch onto the parent branch.

    "gee update" attempts to rebase the current branch onto its parent
    branch.  An interactive flow is implemented to aid in resolving
    merge conflicts.

    If the parent branch is remote (ie, "upstream/master"), gee will
    automatically perform a "git fetch" operation first.

    Before rebasing, gee always creates a `<branch-name>.REBASE_BACKUP`
    tag.  If something went wrong with the merge and you want to get back
    to where you started, you can run:

        git reset --hard your_branch.REBASE_BACKUP
    """

    COMMAND = "update"
    ALIASES = ["up"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)

    def dispatch(self, args):
        cwd = os.getcwd()
        self.gee.check_branch_directory(cwd)
        current_branch = self.gee.get_current_branch()
        parent_branch = self.gee.get_parent_branch(current_branch)
        self.gee.xc().run_git("fetch origin")  # to check for commits in origin
        return self.gee.rebase(current_branch, parent_branch)


class RecursiveUpdateCommand(GeeCommand):
    """Recursive rebase each branch onto it's parent.

    "gee rupdate" extracts the complete chain of parent branches.  Starting from
    the earliest branch, each branch is rebased with changes from it's parent
    branch, until finally the current branch is rebased.

    If the parent branch is remote (ie, "upstream/master"), gee will
    automatically perform a "git fetch" operation first.

    Before rebasing, gee always creates a `<branch-name>.REBASE_BACKUP`
    tag.  If something went wrong with the merge and you want to get back
    to where you started, you can run:

        git reset --hard your_branch.REBASE_BACKUP
    """

    COMMAND = "rupdate"
    ALIASES = ["rup"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)

    def dispatch(self, args):
        current_branch = self.gee.get_current_branch()
        branches = self.gee.get_parent_chain(current_branch)
        for branch in branches:
            # Check each branch to make sure it can be rebased.
            branch_dir = self.gee.branch_dir(branch)
            self.gee.check_branch_directory(branch_dir)
        branches.reverse()
        self.gee.xc().run_git("fetch upstream")
        self.gee.xc().run_git("fetch origin")  # to check for commits in origin
        for i in range(1, len(branches)):
            parent_branch = branches[i - 1]
            branch = branches[i]
            self.gee.banner(f"Rebasing {branch} onto {parent_branch}")
            self.gee.rebase(branch, parent_branch)


class UpdateAllCommand(GeeCommand):
    """Ordered update of all extant branches.

    "gee update_all" updates all local branches (in the correct order),
    by rebasing child branches onto parent branches.

    If the parent branch is remote (ie, "upstream/master"), gee will
    automatically perform a "git fetch" operation first.

    Before rebasing, gee always creates a `<branch-name>.REBASE_BACKUP`
    tag.  If something went wrong with the merge and you want to get back
    to where you started, you can run:

        git reset --hard your_branch.REBASE_BACKUP
    """

    COMMAND = "update_all"
    ALIASES = ["up_all"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)

    def dispatch(self, args):
        _, stdout, _ = self.gee.xc().run_git(
            'branch --format="%(refname:short)"', mode="no_tty", priority=LOW
        )
        all_branches = stdout.strip().split()
        complete_chain = []
        for branch in all_branches:
            if branch in complete_chain:
                continue
            subchain = self.gee.get_parent_chain(branch)
            subchain = [x for x in subchain if x not in complete_chain]
            subchain.reverse()  # order from eldest to youngest
            complete_chain.extend(subchain)

        self.gee.xc().run_git("fetch upstream")
        self.gee.xc().run_git("fetch origin")  # to check for commits in origin
        for branch in complete_chain:
            if "/" in branch:
                continue
            parent_branch = self.gee.get_parent_branch(branch)
            self.gee.banner(f"Rebasing {branch} onto {parent_branch}")
            branch_dir = self.gee.branch_dir(branch)
            # Check each branch to make sure it can be rebased.
            branch_dir = self.gee.branch_dir(branch)
            if not self.gee.check_branch_directory(branch_dir, fatal=False):
                self.warning(f"Skipping update of {branch}")
                continue
            self.gee.rebase(branch, parent_branch)


class WhatsoutCommand(GeeCommand):
    """List all files that differ from the parent branch.

    "gee whatsout" lists all files in the current branch that differ
    from the parent branch.
    """

    COMMAND = "whatsout"
    ALIASES = ["what", "wat", "out"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)

    def dispatch(self, args):
        current_branch = self.gee.get_current_branch()
        parent_commit = self.gee.get_parent_commit(current_branch)
        self.gee.xc().run_git(f"diff --name-only {parent_commit}...{current_branch}")


class CodeownersCommand(GeeCommand):
    """Report the required approvers for the changes in this brnach.

    Gee examines the set of modified files in this branch, and compares it against
    the rules in the CODEOWNERS file.  Gee then presents the user with detailed
    information about which approvals are necessary to submit this PR:

    *  Each line contains a list of users who are code owners of at least
       some part of the PR.

    *  A minimum of one user from each line must provide approval in order for the
       PR to be merged.

    If `--comment` option is specified, the codeowners information is appended to the
    current PR as a new comment.
    """

    COMMAND = "codeowners"
    ALIASES = ["owners", "reviewers"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument(
            "--comment",
            default=False,
            action="store_true",
            help="Add a codeowners comment to this PR.",
        )
        self.argparser.add_argument(
            "-v",
            "--verbose",
            default=False,
            action="store_true",
            help="Show which rule applies to which file.",
        )

    def dispatch(self, args):
        current_branch = self.gee.get_current_branch()
        parent_commit = self.gee.get_parent_commit(
            current_branch
        )  # TODO(jonathan): should be the base commit
        _, stdout, _ = self.gee.xc().run_git(
            f"diff --name-only {parent_commit}...{current_branch}",
            priority=LOW,
            mode="no_tty",
        )
        files = stdout.strip().splitlines()

        if len(files) == 0:
            self.gee.info(f"No changed files since {parent_commit}")
            return 0

        branch_dir = self.gee.branch_dir(current_branch)
        co = Codeowners()
        co.Load(f"{branch_dir}/CODEOWNERS")
        s = co.GetOwnersForFileSet(files)

        if len(s) == 1 and "" in s:
            self.gee.info("This PR can be submitted with approval from any repo owner.")
        else:
            self.gee.info(
                "This PR can be submitted with approval from at least one reviewer from each line:"
            )
            for o, f in s.items():
                if args.verbose:
                    if o == "":
                        o = "(anybody)"
                    self.gee.info(f"* {o}: {','.join(f)}")
                elif o != "":
                    self.gee.info(f"* {o}")


#####################################################################
# The gee base class.  All the real work is done here.  Any
# functionality that is shared by multiple commands should be here.
#
# TODO: there's a lot here -- organzie this class better so that
#  it will be easier to understand and find things.
#####################################################################


class Gee:
    def __init__(self: "Gee"):
        ch = logging.StreamHandler()
        ch.setFormatter(GeeLogFormatter())

        logging.basicConfig(force=True, handlers=[ch])
        logging.setLoggerClass(GeeLogger)
        self.logger = logging.getLogger(__name__)
        self.logger.setLevel(logging.DEBUG)

        # Current repo selection:
        self.cwd = os.path.realpath(os.getcwd())
        self.initial_cwd = self.cwd
        self.repo = None  # a reference to a repo object in config.

        self.argparser = argparse.ArgumentParser(
            formatter_class=argparse.RawDescriptionHelpFormatter
        )
        self.logger.setLevel(logging.INFO)

        # Generic flags shared by all commands:
        self.argparser.add_argument(
            "--config",
            default=os.environ.get("GEERC_PATH", "$HOME/.gee.rc"),
            help="The path to the configuration file.",
        )
        self.argparser.add_argument(
            "--log_level",
            default="INFO",
            help="Log level (DEBUG, INFO, COMMANDS, WARNINGS, ERRORS)",
            choices=["DEBUG", "INFO", "COMMANDS", "WARNINGS", "ERRORS"],
        )
        self.argparser.add_argument(
            "--dry_run",
            default=False,
            action="store_true",
            help="Don't run any commands, but show what would be run instead.",
        )

        # Construct and register all commands:
        self.subparsers = self.argparser.add_subparsers(
            title="Commands", required=True, dest="subcommands"
        )
        self.subcommands = {}
        command_classes = GeeCommand.__subclasses__()
        while command_classes:
            cmd_class = command_classes.pop(0)
            self.subcommands[cmd_class.COMMAND] = cmd_class(self)
            command_classes.extend(cmd_class.__subclasses__())

        # Mapping of branches to parents
        self.parent_map_loaded = False
        self.parent_map = {}

    ##########################################################################
    # The following methods are related to loading, saving, and fetching
    # configuration information and metadata.
    ##########################################################################

    def load_parents_map(self: "Gee"):
        # TODO(jonathan): Add backwards compatibility shim with old parents file format.
        # TODO(jonathan): find a better place to store this metadata.
        if self.parent_map_loaded:
            return

        self.create_gee_dir()  # make sure .gee directory exists
        parents_file = os.path.join(self.gee_dir(), "parents.json")

        if os.path.isfile(parents_file):
            with open(parents_file, "r", encoding="utf-8") as fd:
                self.parents = json.load(fd)
                fd.close()
        else:
            self.parents = {}  # Create an empty map.

        self.orig_parents = copy.deepcopy(self.parents)

        if self.repo and self.main_branch() not in self.parents:
            self.set_parent_branch(self.main_branch(), f"upstream/{self.main_branch()}")

        self.parent_map_loaded = True

    def save_parents_map(self: "Gee"):
        """Write the parents dictionary back to the .gee/parents file.

        Includes safety checks to prevent writing empty data.
        """
        if (not self.parent_map_loaded) or (self.parents == self.orig_parents):
            return
        if not self.parents:
            self.warn("BUG: almost wrote empty parents file!")
            return

        self.create_gee_dir()  # make sure .gee directory exists
        parents_file = os.path.join(self.gee_dir(), "parents.json")

        with open(parents_file, "w", encoding="utf-8") as fd:
            json.dump(self.parents, fd, indent=2)
            fd.close()

    def set_parent_branch(self: "Gee", branch, parent):
        """Records the parentage of a branch."""
        _, stdout, _ = self.xc().run_git(
            f"merge-base {q(branch)} {q(parent)}", priority=LOW
        )
        mergebase = stdout.strip()
        self.parents[branch] = {"parent": parent, "mergebase": mergebase}

    def get_parent_branch(self: "Gee", branch):
        """Gets the name of the parent of the specified branch."""
        if not branch in self.parents:
            logging.warning(f"Branch {branch} does not have a parent branch.")
            logging.info(f"Using {self.repo['repo']}/{self.repo['main']}")
            self.select_repo()
            self.set_parent_branch(branch, f"{self.repo['repo']}/{self.repo['main']}")
        return self.parents[branch]["parent"]

    def get_parent_chain(self: "Gee", branch):
        """Return an ordered list containing the specified branch and all of its parents.

        The first element in the list is the specified branch.  Each subsequent branch
        listed is the previous branch's parent.
        """
        branches = [branch]
        while "/" not in branch:
            parent_branch = self.get_parent_branch(branch)
            branches.append(parent_branch)
            branch = parent_branch
        return branches

    def get_parent_mergebase(self: "Gee", branch):
        """Gets the name of the parent of the specified branch."""
        if not branch in self.parents:
            logging.warning(f"Branch {branch} does not have a parent branch.")
            logging.info(f"Using {self.repo.repo}/{self.repo.main}")
            self.select_repo()
            self.set_parent_branch(branch, f"{self.repo.repo}/{self.repo.main}")
        return self.parents[branch]["mergebase"]

    def load_config(self: "Gee"):
        path = _expand_path(self.gee_rc_path)
        self.debug("Loading config: %s", path)
        self.config = GeeConfig()
        self.config.load(path)

    def save_config(self: "Gee"):
        self.config.save(self.gee_rc_path)

    def select_repo(self: "Gee"):
        gee_dir = self.gee_dir()
        rel = os.path.relpath(self.cwd, start=gee_dir)
        if rel.startswith(".."):
            self.debug(
                "Could not guess repo from cwd=%r, gee_dir=%r", self.cwd, gee_dir
            )
            return None
        parts = rel.split("/")
        repo = parts[0]
        self.repo = self.config.get(f"repo.{repo}", None)
        self.debug("self.repo: %r", self.repo)
        return self.repo

    def get_repo(self: "Gee"):
        """Get the repo data structure for the current directory.

        The "repo" is a dictionary containing the contents of the
        "repo.<foo>" sub-section of gee's configuration file, as
        selected by the current working directory.

        It contains the following keys:

        * "repo": the name of the cloned repository (in upstream and origin),
          ie. "enkit".

        * "upstream": the github account that owns the upstream repository, ie.
          "enfabrica".

        * "dir": the name of the subdirectory containing the local clone of
          this repository.  ie, "internal"

        * "gcloud_project": The gcloud project wherein presubmit jobs
          associated with this repository's PRs are launched.

        * "git_at_github": the username and hostname to use when connecting
          the the associated git server via ssh, ie. "git@github.com".

        * "clone_depth_months": when creating a shallow clone, the depth of
          the clone in months before the present.

        * "main": the name of the main branch of the repo, if not "main",
          ie. "master".
        """
        if not self.repo:
            self.select_repo()
        return self.repo

    def gee_dir(self: "Gee"):
        return _expand_path(self.config.get("gee.gee_dir", "~/gee"))

    def repo_config_id(self: "Gee"):
        repo = self.get_repo()
        return f"repo.{repo['repo']}"

    def origin_url(self: "Gee"):
        repo = self.get_repo()
        return f"{repo['git_at_github']}:{self.config.get('gee.ghuser')}/{repo['repo']}"

    def repo_descriptor(self: "Gee"):
        """For example, internal/enfabrica."""
        repo = self.get_repo()
        return f"{repo['upstream']}/{repo['repo']}"

    def upstream_url(self: "Gee"):
        repo = self.get_repo()
        return f"{repo['git_at_github']}:{self.repo_descriptor()}"

    def repo_dir(self: "Gee"):
        repo = self.get_repo()
        return f"{self.gee_dir()}/{repo['repo']}"

    def branch_dir(self: "Gee", branch):
        return f"{self.repo_dir()}/{branch}"

    def main_branch(self: "Gee"):
        # TODO(jonathan): alternately, if gh can be run reliably at this stage of initialization:
        #   result = self._gh(("repo", "view", f"{self.config.upstream}/{self.config.repo}",
        #                    "--json", "defaultBranchRef"))
        #   data = json.loads(result)
        #   return data["defaultBranchRef"]["name"]
        main = self.repo.get("main", None)
        if main is None:
            # query upstream repo to determine the default main branch:
            rc, text, _ = self.xc().run_git(f"remote show {self.upstream_url()}")
            mo = re.search(r"  HEAD branch: (\S+)", text)
            if not mo:
                self.fatal('"git remote show" did not report a HEAD branch.')
            main = mo.group(1)
            self.info(f"Upstream branch reports {q(main)} as the HEAD branch.")
            self.config.set(f"{self.repo_config_id()}.main", main)
            self.config.save()
        else:
            self.debug(f"Config file says main branch is {q(main)}")
        return main

    def get_commit(self: "Gee", branch):
        parts = branch.split("/", 2)
        if len(parts) == 2:
            self.xc().run_git(f"fetch {parts[0]} {parts[1]}")
            branch = "FETCH_HEAD"
        return branch

    def get_parent_commit(self: "Gee", branch):
        parent = self.get_parent_branch(branch)
        return self.get_commit(parent)

    ##########################################################################
    # The following methods assist with running subprocesses.
    ##########################################################################

    def find_binary(self: "Gee", b):
        # TODO(jonathan): search self.config.paths for binary.
        return b

    def xc(self: "Gee"):
        """Create a new ExecutionContext."""
        return ExecutionContext(
            gee=self,
            git=self.find_binary(self.config.get("gee.git", "git")),
            gh=self.find_binary(self.config.get("gee.gh", "gh")),
            cwd=self.cwd,
        )

    ##########################################################################
    # The following methods represent basic git operations used in a variety
    # of commands.
    ##########################################################################

    def check_origin(self, branch):
        """This checks to see if origin contains extra commits.

        Some operations require a force-push to origin.  This command checks
        origin first to make sure origin isn't ahead of the local branch. If
        extra commits exist, the user is asked whether or not to integrate
        them.
        """
        if self.remote_branch_exists("origin", branch):
            _, stdout, _ = self.xc().run_git(
                f'rev-list --left-right --count "{branch}...origin/{branch}"',
                priority=LOW,
            )
            counts = [int(x) for x in stdout.strip().split()]
            if counts[1] > 0:
                self.warn(
                    f"Remote branch origin/{branch} contains {counts[1]} commits that are not in local {branch}"
                )
                self.infos(
                    """\
                    There are two likely causes:

                    * Another user may have pushed a commit into your remote
                      branch. You probably will want to integrate the origin
                      commits before proceeding.

                    * You manually rebased your local branch, which rewrites
                      the commit identifiers, but forgot to do a subsequent
                      force-push. In this case, you probably do not want to
                      integrate the origin commits.

                    The commits in question are listed below:
                    """
                )
                self.configure_logp_alias()
                self.xc().run_git(
                    f"logp {q(branch)}..origin/{q(branch)}"
                )  # only two dots
                resp = ask_multi(
                    "Do you want to (P)ull in those commits, (d)iscard those commits, or (a)bort? ",
                    default="p",
                )
                if resp == "p":
                    self.info(f"Pulling in commits from origin/{branch}.")
                    self._inner_rebase(branch, f"origin/{branch}")
                elif resp == "d":
                    self.warn("The extra commits in origin will be discarded.")
                else:
                    self.infos(
                        f"""\
                        gee will now abort, so you can resolve this issue on your own.  If you are
                        certain that you want to blow away and overwrite the commits in origin, you can
                        run:
                            gcd {q(branch)}; git push -u origin {q(branch)} --force
                        """
                    )
                    self.fatal("Aborting.")
                    sys.exit(1)

    def rebase(self, branch, parent, onto=None):
        """Safely rebase branch onto parent.

        It is assumed that "git fetch origin" was run before this operation
        begins.
        """
        self.check_origin(branch)
        self._inner_rebase(branch, parent, onto)

    def rebase_in_progress(self, branch_dir):
        _, git_dir, _ = self.xc().run_git(
            "rev-parse --git-dir", cwd=branch_dir, check=True, priority=LOW
        )
        git_dir = git_dir.strip()
        rebase_files = glob.glob(f"{git_dir}/rebase*")
        if rebase_files:
            return True  # rebase is in progress
        else:
            return False  # rebase is not in progress

    def cherrypick_in_progress(self, branch_dir):
        _, git_dir, _ = self.xc().run_git(
            "rev-parse --git-dir", cwd=branch_dir, check=True, priority=LOW
        )
        git_dir = git_dir.strip()
        cherry_pick_files = glob.glob(f"{git_dir}/CHERRY_PICK_HEAD*")
        if cherry_pick_files:
            return True  # cherry_pick is in progress
        else:
            return False  # cherry_pick is not in progress

    def _inner_rebase(self, branch, parent, onto=None):
        if "/" in parent:
            parts = parent.split("/", 2)
            self.xc().run_git(f"fetch {parts[0]}")

        # The original gee would check for an existing PR here and ask
        # the user to confirm.  This was a work-around because users were
        # worried about a github issue where PR comments would be lost if
        # the PR owner did a force-rebase.  In practice, the users always
        # answer yes to this question.  This version of gee doesn't even
        # check or ask.

        # The original gee would check for uncommitted changes here and
        # issue a warning.  That information is unnecessary noise, dropped
        # here.

        branch_dir = self.branch_dir(branch)
        self.xc().run_git(
            f"tag -f {branch}.REBASE_BACKUP", cwd=branch_dir, priority=LOW
        )
        parent_commit = self.get_commit(parent)
        cmd = ["rebase", "--autostash"]
        if onto:
            cmd += ["--onto", onto]
        cmd += [parent_commit, branch]
        rc, _, _ = self.xc().run_git(cmd, cwd=branch_dir, check=False)
        if rc != 0:
            if not self.rebase_in_progress(branch_dir):
                self.fatal("Rebase command failed for an unknown reason.")
                sys.exit(1)
            self.warn("Rebase operation had merge conflicts.")
            success = self._interactive_conflict_resolution(branch, parent, onto)

            if not success:
                return

            if self.rebase_in_progress(branch_dir):
                self.warn(
                    "Interactive conflict resolution failed, must be manually resolved."
                )
                self.xc().run_git("status", cwd=branch_dir)
                self.fatal(f"Exited without resolving rebase conflict in {branch_dir}.")
                sys.exit(1)

            rc, _, _ = self.xc().run_git(
                f"merge-base --is-ancestor {q(parent_commit)} HEAD", priority=LOW
            )
            if rc != 0:
                self.fatal("Rebase did not succeed, aborting.")
                sys.exit(1)
            self.check_for_merge_conflict_markers(branch, parent_commit)
            self.info("Rebase completed.")
            self.info("To undo: gcd {branch}; git reset --hard {branch}.REBASE_BACKUP")
            self.xc().run_git(f"push --set-upstream --quiet --force origin {q(branch)}")

    def check_for_merge_conflict_markers(self, branch, parent_commit):
        branch_dir = self.branch_dir(branch)
        _ = parent_commit
        rc, _, _ = self.xc().run_git(
            (f'diff | grep -q -E "^((<{6,})|(={6,})|(>{6,}))"'),
            cwd=branch_dir,
            check=False,
            priority=LOW,
        )
        if rc == 0:
            log.warn("Changes still contain merge conflict markers: please resolve.")
            return True
        else:
            return False

    STATUS_DECODE_MAP = {
        "DD": "Both deleted",
        "AU": "Added by them",
        "UD": "Deleted by us",
        "UA": "Added by us",
        "DU": "Deleted by them",
        "AA": "Both added",
        "UU": "Both modified",
    }

    def _resolve_conflict_yours(self, st, file, branch_dir, from_desc, onto_desc):
        self.info(f"{file}: Keeping your version from {from_desc}")
        self.info(f"{file}: Discarding their version from {onto_desc}")
        if st == "UD":
            self.xc().run(f"yes d | {git} mergetool -- {q(file)}", cwd=branch_dir)
            print()
            self.xc().run_interactive(f"{git} rebase --continue", cwd=branch_dir)
        else:
            self.info('Unlike merge, during a rebase, "--theirs" means yours:')
            self.xc().run_git(f"checkout --theirs {q(file)}", cwd=branch_dir)
            self.xc().run_git(f"add {q(file)}", cwd=branch_dir)

    def _resolve_conflict_theirs(self, st, file, branch_dir, from_desc, onto_desc):
        self.info(f"{file}: Discarding your version from {from_desc}")
        self.info(f"{file}: Keeping their version from {onto_desc}")
        if st == "DU":
            self.xc().run(f"yes d | {git} mergetool -- {q(file)}", cwd=branch_dir)
            print()
            self.xc().run_interactive(f"{git} rebase --continue", cwd=branch_dir)
        else:
            self.info('Unlike merge, during a rebase, "--ours" means theirs:')
            self.xc().run_git(f"checkout --ours {q(file)}", cwd=branch_dir)
            self.xc().run_git(f"add {q(file)}", cwd=branch_dir)

    def _interactive_conflict_resolution(self, branch, parent, onto):
        """Interactively help the user resolve merge conflicts.

        Interactive conflict resolution options:
           (Y)ours: discard their changes to a file, keeps yours.
           (T)heirs: discard your changes to a file, keeps theirs.
           (M)erge: invokes the merge resolution tool.
           (G)uimerge: invokes the GUI merge tool.
           (V)iew: view the conflict.
           (A)bort: abort this rebase operation.
           (H)elp: this text.

        Psst!  Secret menu for advanced users:
           (P)ick: abort this merge, and instead attempt "git rebase -i".
           (S)hell: drop into interactive shell.
           s(K)ip: discard your entire conflicting commit.

        The fundamental flow here is:

            while rebase_in_progress:
               * make a list of all files with merge conflicts
               * for each file with merge conflicts:
                   * ask what to do with each file.
                * commit merged files, if any.
                * git rebase --continue

        At each step along the way, we check to see if the rebase is still
        in progress, or if the git rebase FSM "saved us time" by prematurely
        marking the rebase operation complete.
        """
        help_text = textwrap.dedent(
            Gee._interactive_conflict_resolution.__doc__.split("\n\n", 1)[1]
        )
        branch_dir = self.branch_dir(branch)
        git = self.find_binary(self.config.get("gee.git", "git"))
        abort = False
        xc = self.xc()
        xc.chdir(branch_dir)
        while self.rebase_in_progress(branch_dir):
            _, onto_commit, _ = xc.run_git("rev-parse HEAD", priority=LOW)
            _, onto_desc, _ = xc.run_git(
                # "| cat" is necessary to convince git not to add superfluous vt100 codes.
                f"show --oneline --no-color -s {onto_commit} | cat",
                priority=LOW,
            )
            _, from_commit, _ = xc.run_git("rev-parse REBASE_HEAD", priority=LOW)
            _, from_desc, _ = xc.run_git(
                f"show --oneline --no-color -s {from_commit} | cat", priority=LOW
            )
            self.banner(f"Yours:  {from_desc.strip()}", f"Theirs: {onto_desc.strip()}")
            self.xc().run_git("status -s")
            state = "unresolved"
            while state == "unresolved":
                _, status, _ = xc.run_git("status --porcelain", priority=LOW)
                if status == "":
                    self.info("Empty commit, skipping.")
                    xc.run_interactive(f"{git} rebase --skip")
                    state = "resolved"
                    continue
                state = "resolved"
                status_lines = status.splitlines(keepends=False)
                while status_lines:
                    status_line = status_lines.pop(0)
                    st, file = status_line.split(maxsplit=1)
                    if len(st) == 1:
                        self.debug(f"{st}  {file}: no action needed.")
                        continue
                    decoded_st = self.STATUS_DECODE_MAP.get(st, "Bizarre!")
                    self.info(f"{file}: {decoded_st}")
                    resp = "h"
                    while resp == "h":
                        resp = ask_multi(
                            "keep (Y)ours, keep (T)heirs, (M)erge, (G)ui-merge, (V)iew, (A)bort, or (H)elp?  ",
                            options="ytmgvahk",
                        )
                        if resp == "h":
                            log.infos(help_text.splitlines())
                    if resp == "y":
                        state = "unresolved"
                        self._resolve_conflict_yours(
                            st, file, branch_dir, from_desc, onto_desc
                        )
                    elif resp == "t":
                        state = "unresolved"
                        self._resolve_conflict_theirs(
                            st, file, branch_dir, from_desc, onto_desc
                        )
                    elif resp == "m" or resp == "g":
                        state = "unresolved"
                        cmd = [git, "mergetool", "--no-prompt"]
                        if resp == "g":
                            cmd += ["--gui"]
                        cmd += file
                        rc = xc.run_interactive(cmd, check=False)
                        rc, stdout, _ = xc.run_git(
                            "diff --check -- {q(file)}", check=False
                        )
                        if "conflict marker" in stdout:
                            self.warn(
                                "Conflict markers are still present, please resolve."
                            )
                            status_lines.insert(0, status_line)  # redo
                        else:
                            xc.run_git("add .", check=False)
                    # elif resp == "p":
                    #     state = False
                    #     xc.run_git("rebase --abort")
                    #     cmd = [git, "rebase", "-i", "--autostash"]
                    #     if onto:
                    #         cmd += ["--onto", onto]
                    #     cmd += [parent, branch]
                    #     rc = xc.run_interactive(cmd, check=False)
                    #     if rc == 0:
                    #         self.info("Rebase succeeded.")
                    #     else:
                    #         if self.rebase_in_progress(branch_dir):
                    #             self.warn("Rebase operation had merge conflicts.")
                    #         else:
                    #             self.fatal(
                    #                 "Rebase command failed, but rebase is not in progress.  Bug?"
                    #             )
                    #             sys.exit(1)
                    #         status_lines = []  # restart.
                    elif resp == "k":
                        state = "skip"
                        status_lines = []
                    elif resp == "a":
                        state = "abort"
                        xc.run_git("rebase --abort")
                        status_lines = []
                        return False
                    else:
                        raise Exception("wat")
            if not self.rebase_in_progress(branch_dir):
                # Sometimes, such as when picking "theirs", git will spontaneously
                # decide there is no more work to be done and exit the rebase state.
                break
            if state == "resolved":
                xc.run_git(
                    "commit --allow-empty --reuse-message=HEAD@{1}"
                )  # undocumented flag!
                xc.run_interactive(f"{git} rebase --continue")
            elif state == "skip":
                xc.run_git("rebase --skip")
            elif state == "abort":
                xc.run_git("rebase --abort")
            else:
                logging.error("Entered unexpected state: {state}")
        return True

    def check_branch_directory(self, branch_dir, fatal=True):
        """Checks if a branch is intact.

        Fails if a branch is in a rebase-in-progress, cherry-pick-in-progress, or detached
        HEAD state.

        If fatal=True, exits the process on error.  Otherwise, returns True if
        no errors were detected.
        """
        if self.rebase_in_progress(branch_dir):
            self.errors(
                (
                    f"{branch_dir}: a rebase operation is already in progress.",
                    'Try "gee repair" or "git rebase --abort" and try again.',
                )
            )
            if fatal:
                sys.exit(1)
            return False
        if self.cherrypick_in_progress(branch_dir):
            self.errors(
                (
                    f"{branch_dir}: a cherry-pick operation is already in progress.",
                    'Try "gee repair" or "git cherry-pick --abort" and try again.',
                )
            )
            if fatal:
                sys.exit(1)
            return False
        current_branch = self.get_current_branch(branch_dir)
        if current_branch == "HEAD":
            inferred_branch = self.get_inferred_branch(cwd=branch_dir)
            self.errors(
                (
                    f"{branch_dir}: branch is in a detached HEAD state.",
                    f'Try "gee repair" or "git checkout {inferred_branch}" and try again.',
                )
            )
            if fatal:
                sys.exit(1)
            return False
        return True

    ##########################################################################
    # The main method for this application.
    ##########################################################################

    def main(self: "Gee", args):
        self.args = self.argparser.parse_args(args)
        if self.args.log_level == "DEBUG":
            self.logger.setLevel(GeeLogger.DEBUG)
        elif self.args.log_level == "INFO":
            self.logger.setLevel(GeeLogger.INFO)
        elif self.args.log_level == "COMMANDS":
            self.logger.setLevel(GeeLogger.COMMANDS)
        elif self.args.log_level == "WARNINGS":
            self.logger.setLevel(GeeLogger.WARNINGS)
        elif self.args.log_level == "ERRORS":
            self.logger.setLevel(GeeLogger.ERRORS)
        else:
            self.fatal("Unknown log_level: %s", self.args.log_level)

        self.repo = None
        self.gee_rc_path = os.environ.get("GEERC_PATH", "$HOME/.gee.rc")
        if self.args.config:
            self.gee_rc_path = self.args.config
        self.load_config()
        self.config.validate()
        self.select_repo()
        self.load_parents_map()
        rc = self.args.func(self.args)
        self.save_parents_map()
        return rc

    def debug(self: "Gee", msg, *args, **kwargs):
        self.logger.debug(msg, *args, **kwargs)

    def info(self: "Gee", msg, *args, **kwargs):
        self.logger.info(msg, *args, **kwargs)

    def infos(self: "Gee", multiline_text):
        multiline_text = textwrap.dedent(multiline_text)
        for msg in multiline_text.splitlines(keepends=False):
            self.logger.info(msg)

    def warn(self: "Gee", msg, *args, **kwargs):
        self.logger.warning(msg, *args, **kwargs)

    def warning(self: "Gee", msg, *args, **kwargs):
        self.logger.warning(msg, *args, **kwargs, stacklevel=2)

    def error(self: "Gee", msg, *args, **kwargs):
        self.logger.error(msg, *args, **kwargs, stacklevel=2)

    def errors(self: "Gee", msgs, stacklevel=2, **kwargs):
        for msg in msgs:
            self.logger.error(msg, **kwargs, stacklevel=stacklevel, stack_info=False)

    def fatal(self: "Gee", msg, *args, stacklevel=2, **kwargs):
        self.logger.error(msg, *args, **kwargs, stacklevel=stacklevel, stack_info=False)
        sys.exit(1)

    def fatals(self: "Gee", msgs, stacklevel=2, **kwargs):
        for msg in msgs:
            self.logger.error(msg, **kwargs, stacklevel=stacklevel, stack_info=False)
        sys.exit(1)

    def exception(self: "Gee", msg, *args, stacklevel=2, **kwargs):
        self.logger.critical(
            msg, *args, **kwargs, stacklevel=stacklevel, stack_info=True
        )
        sys.exit(1)

    def banner(self: "Gee", *msgs):
        self.logger.info("")
        self.logger.info(50 * "#")
        for msg in msgs:
            self.logger.info(f"# {msg}")
        self.logger.info(50 * "#")

    ########################################

    def install_tools(self: "Gee"):
        # TODO(jonathan): implement
        pass

    def create_gee_dir(self: "Gee"):
        gee_dir = pathlib.Path(self.gee_dir())
        if not gee_dir.exists():
            gee_dir.mkdir(parents=True)
        if not gee_dir.is_dir():
            self.fatal("%s exists but is not a directory.", gee_dir)

    # Diagnostics:
    #   * Diagnostics usually run commands with the priority=LOW flag to avoid
    #     polluting the TUI.  If a diagnostic fails, any successive remedies
    #     or retries should have priority=HIGH.
    def check_ssh_agent(self: "Gee"):
        if not os.environ.get("SSH_AUTH_SOCK", None):
            self.warn("SSH_AUTH_SOCK is not set.")
            self.fatal("Start an ssh-agent and try again.")
        rc, _, _ = self.xc().run("ssh-add -l", priority=LOW, timeout=2, check=False)
        if rc == 2:
            self.error("SSH_AUTH_SOCK is set, but ssh-agent is unresponsive.")
            self.fatal("Start a new ssh-agent and try again.")

    def check_ssh(self: "Gee", priority=LOW):
        """Returns true iff we can ssh to github."""
        self.check_ssh_agent()
        git_at_github = self.config.get("gee.git_at_github", "git@github.com")
        rc, stdout, _ = self.xc().run(
            f"ssh -xT {git_at_github} </dev/null 2>&1", priority=priority, check=False
        )

        mo = re.match(r"^Hi ([a-zA-Z0-9_-]+)", stdout, flags=re.MULTILINE)
        if mo:
            return True
        self.warn("Could not authenticate to %s using ssh.", git_at_github)

        ssh_key_file = self.config.get("gee.ssh_key", None)
        if ssh_key_file and os.path.exists(ssh_key_file):
            self.xc().run(f"ssh-add {q(_expand_path(ssh_key_file))}")
            rc, stdout, _ = self.xc().run(f"ssh -xT {git_at_github} </dev/null 2>&1")

            mo = re.match(r"^Hi ([a-zA-Z0-9_-]+)", stdout, flags=re.MULTILINE)
            if mo:
                return True
            self.warn("Still could not authenticate to %s using ssh.", git_at_github)
        return False

    def check_gh_auth(self: "Gee"):
        xc = self.xc()
        rc, _, _ = xc.run_gh("auth status", priority=LOW, check=False)
        if rc == 0:
            return True
        self.warn("gh could not authenticate to github.")
        return self.gh_authenticate()

    def check_executable(self: "Gee", path):
        return (
            path
            and os.path.exists(path)
            and os.path.isfile(path)
            and os.access(path, os.X_OK)
        )

    def check_basics(self: "Gee"):
        self.check_executable(self.find_binary(self.config.get("gee.git", "git")))
        self.check_executable(self.find_binary(self.config.get("gee.gh", "gh")))
        # TODO check ghuser
        # TODO check git config email
        # TODO check git config username

    def gh_authenticate(self: "Gee"):
        self.infos(
            """\
          You are not currently authenticated to github.  We need to create a github
          authentication token for you before we can proceed.

          To create a github authentication token:

          1. Open in a web browser: https://github.com/settings/tokens/new
          2. Log in to github, if you aren't already.
          3. Fill in the \"Note\" field with something descriptive.
          4. Select a reasonable Expiration duration (90 days is recommended).
          5. Click to enable the following permissions:
             * \"repo\" (enables the whole sub-tree of permissions)
             * \"read:org\" (below the \"admin:org\" permission tree)
             * \"admin:public_key\" (enables the whole sub-tree of permissions)
          6. Finally, click \"Generate token\" at the bottom of the form.

          """
        )
        tries = 3
        while tries > 0:
            token = input("Paste your github authentication token here: ")
            self.info("Trying to authenticate using your token")
            rc, _, _ = self.xc().run_gh(
                f"auth login -p ssh -h github.com --with-token",
                stdin=token + "\n",
                check=False,
            )
            if rc == 0:
                self.info("Authentication successful.")
                break
            else:
                self.warning("Authentication unsuccessful.  Try again.")
            tries -= 1
        rc, _, _ = self.xc().run_gh("auth status", priority=HIGH, check=False)
        if rc != 0:
            self.fatal("gh still could not authenticate to github.")
            return False
        else:
            return True

    def check_in_repo(self: "Gee"):
        self.check_basics()
        # Make sure gee init has been run:
        if not self.select_repo():
            self.fatal("Current directory is not inside %r", self.gee_dir())
        repo_dir = self.repo_dir()
        if not os.path.isdir(repo_dir):
            self.fatal('Directory %r is missing, run "gee init".', repo_dir)

        # Make sure we're not in one of bazel's weird symlink
        # directories.
        _, pwd_p, _ = self.xc().run("pwd -P", priority=LOW)
        _, pwd_l, _ = self.xc().run("pwd -L", priority=LOW)
        while pwd_p != pwd_l:
            self.cwd = os.path.dirname(self.cwd)
            _, pwd_p, _ = self.xc().run("pwd -P", priority=LOW)
            _, pwd_l, _ = self.xc().run("pwd -L", priority=LOW)

        # check that we're in a git repo
        rc, gitdir, _ = self.xc().run_git(
            "rev-parse --show-toplevel 2>/dev/null", priority=LOW, check=False
        )
        if rc != 0:
            self.fatal("Current directory is not in a git workspace.")
        if not gitdir.startswith(repo_dir):
            self.fatal("Current directory is not beneath %r", repo_dir)
        return True

    def get_current_branch(self: "Gee", cwd=None):
        if not self.check_in_repo():
            # default to the main branch:
            return self.main_branch()
        else:
            rc, branch, _ = self.xc().run_git(
                "rev-parse --abbrev-ref HEAD", priority=LOW, check=False, cwd=cwd
            )
            branch = branch.strip()
            if rc != 0:
                logging.warning(
                    "Could not identify current branch: git rev-parse command failed (rc=%d)",
                    rc,
                )
                return self.main_branch()
            elif branch == "":
                logging.warning(
                    "Could not identify current branch: git rev-parse said nothing."
                )
                return self.main_branch()
            else:
                if branch == "HEAD":
                    logging.warning("Current branch is in a detached HEAD state.")
                return branch

    def get_inferred_branch(self: "Gee", cwd=None):
        """Infers the intended branch name from the workdir path.

        There are a couple of things we could do here: one is, we can infer
        the branch from the gee-created workdir path name.  Or, we can use
        the metadata provided by "git worktree list" to identify the branch
        associated with each worktree.

        Simple solutions such as `git show -s --pretty=%d HEAD` don't work
        in all cases (ie, in a rebase-in-progress state).
        """
        _, stdout, _ = self.xc().run_git(
            "rev-parse --show-toplevel", cwd=cwd, priority=LOW, check=True
        )
        return os.path.basename(stdout.strip())

    def ssh_enroll(self: "Gee"):
        """Ensure the user has ssh access to github, or enroll the user if not."""
        self.check_ssh_agent()  # ensure ssh agent is running.
        self.xc().run_gh("config set git_protocol ssh")

        # Make sure the user has an ssh key:
        ssh_key_file = self.config.get("gee.ssh_key", None)
        if not ssh_key_file:
            self.config.set("gee.ssh_key", "$HOME/.ssh/id_ed25519")
            self.config.save()
            ssh_key_file = self.config.get("gee.ssh_key", None)
        ssh_key_file = _expand_path(ssh_key_file)
        if os.path.exists(ssh_key_file):
            self.logger.info(f"Re-using existing ssh key {q(ssh_key_file)}")
        else:
            self.warn(
                "%s: missing.  gee will help you generate a new key.", ssh_key_file
            )
            _ = self.xc().run_interactive(
                f'ssh-keygen -f {q(ssh_key_file)} -t ed25519 -C "{os.environ["USER"]}@enfabrica.net"'
            )
            if not os.path.exists(ssh_key_file):
                self.logger.fatal(f"ssh-keygen failed.")

        # Make sure the user's ssh key is in .ssh/config:
        text = ""
        if os.path.exists(_expand_path("$HOME/.ssh/config")):
            with open(_expand_path("$HOME/.ssh/config"), "r", encoding="utf-8") as fd:
                text = fd.read()
                fd.close()
        mo = re.search(r"Host .*github.com", text)
        if not mo:
            text += "\n".join(
                [
                    "",
                    "# gee: block start",
                    "Host *.github.com github.com",
                    f"  IdentityFile {ssh_key_file}",
                    "# gee: block stop",
                    "",
                ]
            )
            with open(_expand_path("$HOME/.ssh.config"), "w", encoding="utf-8") as fd:
                fd.write(text)
                fd.close()

        # Make sure the user's ssh-key is enrolled:
        _ = self.xc().run_interactive(f"ssh-add {ssh_key_file}", priority=HIGH)

        # If we can ssh into github, we're done:
        if self.check_ssh():
            return

        # Make sure our ssh key is enrolled at github
        rc, _, _ = self.xc().run_gh(
            f'ssh-key add {ssh_key_file}.pub --title "gee-enrolled-key"', check=False
        )
        if rc != 0:
            self.logger.warn(
                "Could not add your key to github (maybe it's already there?)."
            )
        _, _, _ = self.xc().run_gh("ssh-key list")
        # TODO(jonathan): check that key is in the list
        # the output looks like:
        # GitHub CLI      ssh-ed25519 XXX...XXX 2024-03-19T17:37:12Z    96758991        authentication

        # fatal if we still can't connect via ssh
        if not self.check_ssh(priority=HIGH):
            self.logger.fatal(
                "Something still wrong: can't authenticate to github via ssh."
            )

    def parse_url(self: "Gee", url, main=None):
        ssh_re = re.compile(r"^([a-zA-Z0-9_-]+)@github.com:(\S+)\/(\S+).git$")
        https_re = re.compile(r"^https://github.com/(\S+)/(\S+).git$")
        print(url)
        mo = ssh_re.match(url)
        repo_dict = None
        if mo:
            git_at_github = f"{mo.group(1)}@github.com"
            upstream = mo.group(2)
            repo = mo.group(3)
            repo_dict = self.config.get(f"repo.{repo}", None)
        else:
            mo = https_re.match(url)
            if mo:
                git_at_github = "git@github.com"
                upstream = mo.group(1)
                repo = mo.group(2)
                repo_dict = self.config.get(f"repo.{repo}", None)
            else:
                self.fatal("Could not parse repo URL: %r", url)
        if repo_dict is None:
            repo_dict = {
                "upstream": upstream,
                "repo": repo,
                "git_at_github": git_at_github,
                "dir": upstream,
                "clone_depth_months": 3,
                "main": main,
            }
            self.config.set(f"repo.{repo}", repo_dict)
            self.config.save()
        self.repo = repo_dict

    def make_fork(self: "Gee"):
        if not self.repo:
            self.fatal("Could not make fork: unknown repo.")
        user_fork = f"{self.config.get('gee.ghuser')}/{self.repo['repo']}"
        rc, repo_list_text, _ = self.xc().run_gh(f"repo list | grep {user_fork}")
        assert rc == 0
        repo_set = set([x.split()[0] for x in repo_list_text.splitlines()])
        if user_fork in repo_set:
            self.info(f"{user_fork}: remote branch already exists.")
        else:
            _, _, _ = self.xc().run_gh(
                f"repo fork --clone=false {q(self.repo_descriptor())}", check=True
            )

    def clone(self: "Gee"):
        if not self.repo:
            self.fatal("Could not clone: unknown repo.")
        depth_months = self.repo.get(
            "clone_depth_months", self.config.get("clone_depth_months", 3)
        )
        clone_since = datetime.date.today() - datetime.timedelta(weeks=4 * depth_months)
        clone_since = clone_since.strftime("%Y-%m-%d")
        main_branch_dir = f"{self.repo_dir()}/{self.main_branch()}"
        if os.path.isdir(main_branch_dir):
            self.warning(f"{main_branch_dir}: already exists, skipping clone step.")
        else:
            self.xc().run_git(
                [
                    "clone",
                    "--shallow-since",
                    clone_since,
                    "--no-single-branch",
                    self.origin_url(),
                    main_branch_dir,
                ],
                mode="interactive",  # this is a slow command.
                check=True,
            )
        self.cwd = main_branch_dir
        rc, _, stderr = self.xc().run_git(
            f"remote add upstream {q(self.upstream_url())}", check=False
        )
        if rc != 0 and "remote upstream already exists" not in stderr:
            print(repr(stderr))
            self.fatal("Could not add upstream remote.")
            sys.exit(1)
        _, _, _ = self.xc().run_git("fetch --quiet upstream")

    def remote_branch_exists(self, repo, branch) -> bool:
        rc, stdout, _ = self.xc().run_git(
            f"ls-remote {q(repo)} {q(branch)}", priority=LOW
        )
        return not (stdout.strip() == "")

    def make_branch(self: "Gee", branch: str, parent: Optional[str] = None):
        """Create a new branch and workdir, based on parent or the current branch."""
        if not parent:
            parent = self.get_current_branch()
        path = self.branch_dir(branch)
        self.xc().run_git(f"worktree add -f -b {q(branch)} {q(path)} {q(parent)}")
        self.set_parent_branch(branch, parent)
        self.cwd = path  # all further commands run from this new branch.

        self.xc().run_git("fetch origin", priority=LOW, check=True)
        if self.remote_branch_exists("origin", branch):
            _, text, _ = self.xc().run_git(
                f'rev-list --left-right --count "HEAD...origin/{branch}"', priority=LOW
            )
            counts = text.strip().split()
            if int(counts[1]) > 0:
                self.warn(
                    f"Remote branch origin/{branch} is {counts[1]} commits ahead of {branch}."
                )
                self.warn(
                    f"Do you want to reset {branch} to be the same as origin/{branch}?"
                )
                if ask_yesno(f"Reset {branch} to match origin/{branch}?", default=True):
                    self.xc().run_git(f'reset --hard "origin/{branch}"')
                else:
                    self.warn("Commits from origin were not integrated.")
                    self.warn(
                        f'You probably want to run "gee update" in branch {branch}.'
                    )
            else:
                self.debug(
                    f"Remote branch origin/{branch} exists, but is not ahead of {branch}."
                )

    def configure_logp_alias(self: "Gee"):
        if self.xc().run_git("config --get alias.logp", check=False, priority=LOW):
            self.debug("alias.logp is already defined.")
            return
        logp_command = shlex.quote(
            shlex.join(
                (
                    "log",
                    "--color",
                    "--graph",
                    "--pretty=format:%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset",
                    "--abbrev-commit",
                )
            )
        )
        self.xc().run_git(f"config --global alias.logp {logp_command}")

    def configure(self: "Gee"):
        self.xc().run_git("config --global rerere.enabled true")
        self.configure_logp_alias()
        mergetool = self.config.get("gee.mergetool", "vim")
        if mergetool == "vim":
            self.info("Configuring git to use vimdiff as the default mergetool.")
            self.xc().run_git("config --global merge.tool vimdiff")
            self.xc().run_git("config --global merge.conflictstyle diff3")
            self.xc().run_git("config --global mergetool.prompt false")
            self.xc().run_git("config --global diff.difftool vimdiff")
            self.xc().run_git(
                [
                    "config",
                    "--global",
                    "difftool.vimdiff.cmd",
                    'vimdiff "$LOCAL" "$REMOTE"',
                ]
            )
            if not self.find_binary("vimdiff"):
                self.warning("vimdiff is configured, but the tool could not be found.")
        elif mergetool == "nvim":
            self.info("Configuring git to use nvim as the default mergetool.")
            self.xc().run_git("config --global merge.tool vimdiff")
            self.xc().run_git("config --global merge.tool nvimdiff")
            self.xc().run_git("config --global merge.conflictstyle diff3")
            self.xc().run_git("config --global mergetool.prompt false")
            self.xc().run_git("config --global diff.difftool nvimdiff")
            self.xc().run_git(
                [
                    "config",
                    "--global",
                    "difftool.nvimdiff.cmd",
                    'nvim -d "$LOCAL" "$REMOTE"',
                ]
            )
            if not self.find_binary("nvim"):
                self.warning("nvim is configured, but the tool could not be found.")
        elif mergetool == "vscode":
            self.info("Setting vscode as the default GUI diff and merge tool.")
            self.xc().run_git("config --global merge.guitool vscode")
            self.xc().run_git(
                ["config", "--global", "mergetool.vscode.cmd", 'code --wait "$MERGED"']
            )
            if not self.find_binary("code"):
                self.warning("vscode is configured, but the tool could not be found.")
        elif mergetool == "meld":
            self.info("Setting meld as the default GUI diff and merge tool.")
            self.xc().run_git("config --global merge.guitool meld")
            self.xc().run_git(
                "config",
                "--global",
                "mergetool.meld.cmd",
                '/usr/bin/meld "$LOCAL" "$MERGED" "$REMOTE" --output "$MERGED"',
            )
            self.xc().run_git("config --global diff.guitool meld")
            self.xc().run_git(
                [
                    "config",
                    "--global difftool.meld.cmd",
                    '/usr/bin/meld "$LOCAL" "$REMOTE"',
                ]
            )
            if not self.find_binary("meld"):
                self.warning("meld is configured, but the tool could not be found.")
        elif mergetool == "bcompare":
            self.info("Setting BeyondCompare as the default GUI diff and merge tool.")
            # Note "bc" selects a wrapper for beyondcompare that is distributed with git.
            self.xc().run_git("config --global merge.guitool bc")
            self.xc().run_git("config --global diff.guitool bc")
            self.xc().run_git("config --global mergetool.bc.trustExitCode true")
            self.xc().run_git("config --global difftool.bc.trustExitCode true")
            if not self.find_binary("bcompare"):
                self.warning("bcompare is configured, but the tool could not be found.")
        else:
            self.error("Unsupported mergetool configuration: %s", mergetool)
            self.info("Valid options are: bcompare, meld, nvim, vim, vscode")


#####################################################################
# main
#####################################################################


def main(args):
    os.chdir(".")  # git likes to recreate directories, this is a workaround.
    gee = Gee()
    gee.main(args)


if __name__ == "__main__":
    main(sys.argv[1:])
