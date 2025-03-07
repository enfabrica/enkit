#!/usr/bin/python3
"""gee, Rewriten in python.

Goals:
    * run directly anywhere:
        * use only the standard python library
        * monolithic utility (no support files)
    * respect a user's .geerc file
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
import datetime
import difflib
import io
import logging
import os
import re
import shutil
import subprocess
import pathlib
import shlex
import sys
import textwrap
import types
import inspect
import toml
import json
import logging
from typing import List, Optional

#####################################################################
## Utility functions
#####################################################################

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


#####################################################################
# Configuring gee
#####################################################################

class GeeConfig:
    """Responsible for loading and saving the user's .gee.rc file.

    TODO(jonathan): help the user create a default .gee.rc file on startup.
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

    def _inner_load(self, path):
        path = _expand_path(path)
        with open(path, "r", encoding="utf-8") as fd:
            d = toml.decoder.load(fd)
            fd.close()
        if "load" in d:
            for p in d["load"]:
                self._inner_load(p)
        self.data = self._merge(self.data, d)

    def load(self, path):
        self.path = path
        # TODO(jonathan): create reasonable defaults if missing.
        self._inner_load(path)

    def save(self, path=None):
        if path is None:
            path = self.path
        path = _expand_path(path)
        with open(path, "w", encoding="utf-8") as fd:
            toml.encoder.dump(self.data, fd)
            fd.close()
        logging.debug("Saved config: %r", path)

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
            logging.fatal("Missing %r in configuration.", key)

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
    WARNINGS = 30
    ERRORS = 40


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
    white = "\x1b[38;20m"  # TODO(jonathan): fix this code
    bold_white = "\x1b[38;1m"  # TODO(jonathan): fix this code
    yellow = "\x1b[33;20m"
    red = "\x1b[31;20m"
    bold_red = "\x1b[31;1m"
    reset = "\x1b[0m"
    format = "%(levelname)s - %(message)s"

    FORMATS = {
        logging.DEBUG: logging.Formatter(grey + "DBG: %(message)s" + reset),
        GeeLogger.LOW_STDOUT: logging.Formatter(grey + "o-> %(message)s" + reset),
        GeeLogger.LOW_STDERR: logging.Formatter(grey + "E-> %(message)s" + reset),
        GeeLogger.LOW_COMMANDS: logging.Formatter(grey + "$ " + bold_grey + "%(message)s" + reset),
        logging.INFO: logging.Formatter(grey + "%(message)s" + reset),
        GeeLogger.STDOUT: logging.Formatter(grey + "o-> %(message)s" + reset),
        GeeLogger.STDERR: logging.Formatter(grey + "E-> %(message)s" + reset),
        GeeLogger.COMMANDS: logging.Formatter(white + "$ " + bold_white + "%(message)s" + reset),
        logging.INFO: logging.Formatter(grey + "%(message)s" + reset),
        logging.WARNING: logging.Formatter(yellow + "WARNING: %(message)s" + reset),
        logging.ERROR: logging.Formatter(red + "ERROR: %(message)s" + reset),
        logging.CRITICAL: logging.Formatter(bold_red + "CRITICAL ERROR@%(filename)s:%(lineno)d: %(message)s" + reset),
    }

    def format(self, record):
        formatter = self.FORMATS.get(record.levelno)
        return formatter.format(record)


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
                formatter_class = argparse.RawDescriptionHelpFormatter,
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

    def dispatch(self, args):
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
        self.configure()

        # Save the .geerc file, creating it if it's missing.
        self.save_config()

        self.info("Initialized gee workspace: %s/%s", self.repo_dir(), self.main_branch())

class MakeBranchCommand(GeeCommand):
    """Create a new branch."""
    COMMAND = "make_branch"
    ALIASES = ["mkbr", "branch"]

    def __init__(self, gee_obj: "Gee"):
        super().__init__(gee_obj)
        self.argparser.add_argument(
                "branch",
                help="Name of branch to create.",
        )
        self.argparser.add_argument(
                "parent",
                help="Branch to use as parent for this branch.",
                nargs="?",
                default=None,
        )

    def dispatch(self, args):
        return self.gee.make_branch(args.branch, args.parent)


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
        self.argparser.add_argument(
                "option", help="Configuration option to select",
        )

    def dispatch(self, args):
        if args.option in ("default", "defaults", "vim", "vimdiff", "enable_vim", "enable_vimdiff"):
            self.gee.config.set("gee.mergetool", "vim")
        elif args.option in ("nvim", "nvimdiff", "enable_nvim", "enable_nvimdiff"):
            self.gee.config.set("gee.mergetool", "nvim")
        elif args.option in ("code", "vscode", "enable_code", "enable_vscode"):
            self.gee.config.set("gee.mergetool", "vscode")
        elif args.option in ("meld", "enable_meld"):
            self.gee.config.set("gee.mergetool", "meld")
        elif args.option in ("bcompare", "enable_bcompare", "beyondcompare", "enable_beyondcompare"):
            self.gee.config.set("gee.mergetool", "bcompare")
        else:
            self.gee.error("Unsupported configuration option: %s", args.option)
            return 1

        self.gee.configure()
        self.gee.save_config()

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
        self.repo = None  # a reference to a repo object in config.

        self.argparser = argparse.ArgumentParser(
                formatter_class = argparse.RawDescriptionHelpFormatter,
        )
        self.logger.setLevel(logging.INFO)

        # Generic flags shared by all commands:
        self.argparser.add_argument(
            "--config",
            default=os.environ.get("GEERC_PATH", "$HOME/.geerc"),
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
                action='store_true',
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

    def load_parents_map(self: "Gee"):
        # TODO(jonathan): Add backwards compatibility shim with old parents file format.
        # TODO(jonathan): find a better place to store this metadata.
        if self.parent_map_loaded:
            return

        self.gee.create_gee_dir()  # make sure .gee directory exists
        parents_file = os.path.join(self.gee.gee_dir(), "parents.json")

        if os.path.isfile(parents_file):
            with open(parents_file, "r", encoding="utf-8") as fd:
                self.parents = json.load(fd)
                fd.close()
        else:
            self.parents = {}  # Create an empty map.

        if self.repo:
            # Special case:
            self.parents[self.main_branch()] = {
                    "parent": f"upstream/{self.main_branch()}",
                    "mergebase": "",
            }

        self.parent_map_loaded = True

    def save_parents_map(self: "Gee"):
        """Write the parents dictionary back to the .gee/parents file.

        Includes safety checks to prevent writing empty data.
        """
        if not self.parent_map_loaded:
            return
        if not self.parents:
            warn("BUG: almost wrote empty parents file!")
            return

        self.gee.create_gee_dir()  # make sure .gee directory exists
        parents_file = os.path.join(self.gee.gee_dir(), "parents.json")

        with open(parents_file, "w", encoding="utf-8") as fd:
            json.dump(self.parents, fd, indent=2)
            fd.close()

    def load_config(self: "Gee"):
        self.debug("Loading config: %s", self.geerc_path)
        self.config = GeeConfig()
        self.config.load(self.geerc_path)

    def save_config(self: "Gee"):
        self.config.save(self.geerc_path)

    def select_repo(self: "Gee"):
        cwd = os.path.realpath(os.getcwd())
        gee_dir = self.gee_dir()
        rel = os.path.relpath(cwd, start=gee_dir)
        if rel.startswith(".."):
            self.debug("Could not guess repo from cwd=%r, gee_dir=%r", cwd, gee_dir)
            return
        parts = rel.split("/")
        if len(parts) < 2:
            self.debug("Could not guess repo from rel=%r", rel)
            return
        upstream = parts[0]
        repo = parts[1]
        self.repo = self.config.get(f"repo.{upstream}.{repo}", None)
        self.debug("self.repo: %r", self.repo)

    def find_binary(self: "Gee", b):
        # TODO(jonathan): search self.config.paths for binary.
        return b

    def run_interactive(self: "Gee", cmd, check=True, quiet=False):
        """Run an interactive command that communicates with the console."""

        self.log_command(cmd, quiet=quiet)
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
        )
        stdout, stderr = p.communicate()
        rc = p.wait()
        self.logger.debug("exit status = %d", rc)
        if check and rc != 0:
            self.fatal("Command failed with returncode=%d: %s", rc, cmd)
        return rc

    def run(self: "Gee", cmd, check=True, stdin=None, quiet=False, timeout=None):
        """quiet flag is for diagnostic commands."""
        self.log_command(cmd, quiet=quiet)

        p = subprocess.Popen(
            cmd,
            # Everything gee runs is run through the shell,
            # so the user can copy/paste exactly:
            shell=True if isinstance(cmd, str) else False,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            encoding="utf-8",
            errors="utf-8",
        )
        # TODO(jonathan): implement a reader thread to tee the process
        # output to the logger in realtime.
        stdout, stderr = p.communicate(input=stdin, timeout=timeout)
        self.log_command_stderr(stderr, quiet=quiet)
        self.log_command_stdout(stdout, quiet=quiet)
        rc = p.wait()
        self.logger.debug("exit status = %d", rc)
        if check and rc != 0:
            self.fatal("Command failed with returncode=%d: %s", rc, cmd)
        return rc, stdout, stderr

    def run_git(self: "Gee", cmd, check=True, stdin=None, quiet=False, timeout=None):
        git = self.find_binary(self.config.get("gee.git", "git"))
        if isinstance(cmd, str):
            cmd = git + " " + cmd
        elif isinstance(cmd, list):
            cmd = [git] + cmd
        else:
            raise TypeError("command is not a list or a string: %r", cmd)
        return self.run(cmd, check=check, stdin=stdin, quiet=quiet, timeout=timeout)

    def run_gh(self: "Gee", cmd, check=True, stdin=None, quiet=False, timeout=None):
        gh = self.find_binary(self.config.get("gee.gh", "gh"))
        if isinstance(cmd, str):
            cmd = gh + " " + cmd
        elif isinstance(cmd, list):
            cmd = [gh] + cmd
        else:
            raise TypeError("command is not a list or a string: %r", cmd)
        return self.run(cmd, check=check, stdin=stdin, quiet=quiet, timeout=timeout)

    def origin_url(self: "Gee"):
        if not self.repo:
            self.fatal("Repo was unknown.")
        return f"{self.repo['git_at_github']}:{self.config.get('gee.ghuser')}/{self.repo['repo']}"

    def repo_descriptor(self: "Gee"):
        """For example, internal/enfabrica."""
        if not self.repo:
            self.fatal("Repo was unknown.")
        return (
            f"{self.repo['upstream']}/{self.repo['repo']}"
        )

    def upstream_url(self: "Gee"):
        if not self.repo:
            self.fatal("Repo was unknown.")
        return (
                f"{self.repo['git_at_github']}:{self.repo_descriptor()}"
        )

    def gee_dir(self: "Gee"):
        """Path to the root of the gee directory."""
        return _expand_path(self.config.get("gee.gee_dir", "~/gee"))

    def repo_dir(self: "Gee"):
        if not self.repo:
            self.fatal("Repo was unknown.")
        return f"{self.gee_dir()}/{self.repo['repo']}"

    def repo_config_id(self: "Gee"):
        if not self.repo:
            self.fatal("Repo was unknown.")
        return f"repo.{self.repo['upstream']}.{self.repo['repo']}"

    def main_branch(self: "Gee"):
        # TODO(jonathan): alternately, if gh can be run reliably at this stage of initialization:
        #   result = self._gh(("repo", "view", f"{self.config.upstream}/{self.config.repo}",
        #                    "--json", "defaultBranchRef"))
        #   data = json.loads(result)
        #   return data["defaultBranchRef"]["name"]
        main = self.repo.get("main", None)
        if main is None:
            # query upstream repo to determine the default main branch:
            rc, text, _ = self.run_git(f"remote show {self.upstream_url()}")
            mo = re.search(r"  HEAD branch: (\S+)", text)
            if not mo:
                self.fatal('"git remote show" did not report a HEAD branch.')
            main = mo.group(1)
            self.info(f"Upstream branch reports {main!r} as the HEAD branch.")
            self.config.set(f"{self.repo_config_id()}.main", main)
            self.config.save()
        else:
            self.debug(f"Config file says main branch is {main!r}")
        return main

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
        self.geerc_path = os.environ.get("GEERC_PATH", "$HOME/.geerc")
        if self.args.config:
            self.geerc_path = self.args.config
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

    def fatal(self: "Gee", msg, *args, **kwargs):
        self.logger.fatal(msg, *args, **kwargs, stacklevel=2, stack_info=True)
        sys.exit(1)

    def log_command(self: "Gee", cmd, quiet=False):
        """Log a command at COMMAND priority, or LOW_COMMAND if quiet is true.

        quiet=False is for mainline commands that teach the user git.
        quiet=True is for less essential commands (error checks, diagnostics) that
          aren't as useful for the user to see.
        """
        if isinstance(cmd, list) and not isinstance(cmd, str):
            cmd = " ".join([shlex.quote(x) for x in cmd])
        if not quiet:
            self.logger.cmd(cmd)
        else:
            self.logger.low_cmd(cmd)

    def log_command_stdout(self: "Gee", text, quiet=False):
        for line in text.splitlines():
            if not quiet:
                self.logger.cmd_stdout(line)
            else:
                self.logger.low_cmd_stdout(line)

    def log_command_stderr(self: "Gee", text, quiet=False):
        for line in text.splitlines():
            if not quiet:
                self.logger.cmd_stderr(line)
            else:
                self.logger.low_cmd_stderr(line)


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
    #   * Diagnostics usually run commands with the quiet=True flag to avoid
    #     polluting the TUI.  If a diagnostic fails, any successive remedies
    #     or retries should have quiet=False.
    def check_ssh_agent(self: "Gee"):
        if not os.environ.get("SSH_AUTH_SOCK", None):
            self.warn("SSH_AUTH_SOCK is not set.")
            self.fatal("Start an ssh-agent and try again.")
        rc, _, _ = self.run("ssh-add -l", quiet=True, timeout=2, check=False)
        if rc == 2:
            self.error("SSH_AUTH_SOCK is set, but ssh-agent is unresponsive.")
            self.fatal("Start a new ssh-agent and try again.")

    def check_ssh(self: "Gee", quiet=True):
        """Returns true iff we can ssh to github."""
        self.check_ssh_agent()
        git_at_github = self.config.get("gee.git_at_github", "git@github.com")
        rc, stdout, _ = self.run(
            f"ssh -xT {git_at_github} </dev/null 2>&1", quiet=quiet, check=False
        )

        mo = re.match(r"^Hi ([a-zA-Z0-9_-]+)", stdout, flags=re.MULTILINE)
        if mo:
            return True
        self.warn("Could not authenticate to %s using ssh.", git_at_github)

        ssh_key_file = self.config.get("gee.ssh_key", None)
        if ssh_key_file and os.path.exists(ssh_key_file):
            self.run(f"ssh-add {_expand_path(ssh_key_file)!r}")
            rc, stdout, _ = self.run(f"ssh -xT {git_at_github} </dev/null 2>&1")

            mo = re.match(r"^Hi ([a-zA-Z0-9_-]+)", stdout, flags=re.MULTILINE)
            if mo:
                return True
            self.warn("Still could not authenticate to %s using ssh.", git_at_github)
        return False

    def check_gh_auth(self: "Gee"):
        rc, _, _ = self.run_gh("auth status", quiet=True, check=False)
        if rc == 0:
            return True
        self.warn("gh could not authenticate to github.")
        return self.gh_authenticate()

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
            rc, _, _ = self.run_gh(
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
        rc, _, _ = self.run_gh("auth status", quiet=False, check=False)
        if rc != 0:
            self.fatal("gh still could not authenticate to github.")
            return False
        else:
            return True

    def check_in_repo(self: "Gee"):
        self.check_basics()
        # Make sure gee init has been run:
        repo_dir = self.config.get("gee.repo_dir")
        if not os.path.isdir(repo_dir):
            self.fatal('Directory %r is missing, run "gee init".', repo_dir)

        # Make sure we're not in one of bazel's weird symlink
        # directories.
        _, pwd_p, _ = self.run("pwd -P", quiet=True)
        _, pwd_l, _ = self.run("pwd -L", quiet=True)
        while pwd_p != pwd_l:
            os.chdir("..")
            _, pwd_p, _ = self.run("pwd -P", quiet=True)
            _, pwd_l, _ = self.run("pwd -L", quiet=True)

        # check that we're in a git repo
        rc, gitdir, _ = self.run_git(
            "rev-parse --git-common-dir 2>/dev/null )", quiet=True, check=False
        )
        if rc != 0:
            self.fatal("Current directory is not in a git workspace.")
        if not gitdir.startswith(repo_dir):
            self.fatal("Current directory is not beneath %r", repo_dir)

    def ssh_enroll(self: "Gee"):
        """Ensure the user has ssh access to github, or enroll the user if not."""
        self.check_ssh_agent()  # ensure ssh agent is running.
        self.run_gh("config set git_protocol ssh")

        # Make sure the user has an ssh key:
        ssh_key_file = self.config.get("gee.ssh_key", None)
        if not ssh_key_file:
            self.config.set("gee.ssh_key", "$HOME/.ssh/id_ed25519")
            self.config.save()
            ssh_key_file = self.config.get("gee.ssh_key", None)
        ssh_key_file = _expand_path(ssh_key_file)
        if os.path.exists(ssh_key_file):
            self.logger.info(f"Re-using existing ssh key {ssh_key_file!r}")
        else:
            self.warn("%s: missing.  gee will help you generate a new key.", ssh_key_file)
            _ = self.run_interactive(
                f'ssh-keygen -f {ssh_key_file!r} -t ed25519 -C "{os.environ["USER"]}@enfabrica.net"'
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
        _ = self.run_interactive(f"ssh-add {ssh_key_file}", quiet=False)

        # If we can ssh into github, we're done:
        if self.check_ssh():
            return

        # Make sure our ssh key is enrolled at github
        rc, _, _ = self.run_gh(
            f'ssh-key add {ssh_key_file}.pub --title "gee-enrolled-key"',
            check=False,
        )
        if rc != 0:
            self.logger.warn(
                "Could not add your key to github (maybe it's already there?)."
            )
        _, _, _ = self.run_gh("ssh-key list")
        # TODO(jonathan): check that key is in the list
        # the output looks like:
        # GitHub CLI      ssh-ed25519 XXX...XXX 2024-03-19T17:37:12Z    96758991        authentication

        # fatal if we still can't connect via ssh
        if not self.check_ssh(quiet=False):
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
            repo_dict = self.config.get(f"repo.{upstream}.{repo}", None)
        else:
            mo = https_re.match(url)
            if mo:
                git_at_github = "git@github.com"
                upstream = mo.group(1)
                repo = mo.group(2)
                repo_dict = self.config.get(f"repo.{upstream}.{repo}", None)
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
            self.config.set(f"repo.{upstream}.{repo}", repo_dict)
            self.config.save()
        self.repo = repo_dict

    def make_fork(self: "Gee"):
        if not self.repo:
            self.fatal("Could not make fork: unknown repo.")
        user_fork = f"{self.config.get('gee.ghuser')}/{self.repo['repo']}"
        rc, repo_list_text, _ = self.run_gh(f"repo list | grep {user_fork}")
        assert rc == 0
        repo_set = set([x.split()[0] for x in repo_list_text.splitlines()])
        if user_fork in repo_set:
            self.info(f"{user_fork}: remote branch already exists.")
        else:
            _, _, _ = self.run_gh(
                f"repo fork --clone=false {self.repo_descriptor()!r}", check=True
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
            self.run_git(
                " ".join(
                    [
                        "clone",
                        f"--shallow-since {clone_since!r}",
                        "--no-single-branch",
                        f"{self.origin_url()!r}",
                        main_branch_dir,
                    ]
                ),
                check=True,
            )
        os.chdir(main_branch_dir)
        rc, _, stderr = self.run_git(
            f"remote add upstream {self.upstream_url()!r}", check=False
        )
        if rc != 0 and "remote upstream already exists" not in stderr:
            print(repr(stderr))
            self.fatal("Could not add upstream remote.")
            sys.exit(1)
        _, _, _ = self.run_git("fetch upstream")

    def make_branch(self: "Gee", branch: str, parent: Optional[str]=None):
        """Create a new branch and workdir, based on parent or """

    def configure(self: "Gee"):
        self.run_git("config --global rerere.enabled true")
        mergetool = self.config.get("gee.mergetool", "vim")
        if mergetool == "vim":
            self.info("Configuring git to use vimdiff as the default mergetool.")
            self.run_git("config --global merge.tool vimdiff")
            self.run_git("config --global merge.conflictstyle diff3")
            self.run_git("config --global mergetool.prompt false")
            self.run_git("config --global diff.difftool vimdiff")
            self.run_git(["config", "--global", "difftool.vimdiff.cmd", "vimdiff \"$LOCAL\" \"$REMOTE\""])
            if not self.find_binary("vimdiff"):
                self.warning("vimdiff is configured, but the tool could not be found.")
        elif mergetool == "nvim":
            self.info("Configuring git to use nvim as the default mergetool.")
            self.run_git("config --global merge.tool vimdiff")
            self.run_git("config --global merge.tool nvimdiff")
            self.run_git("config --global merge.conflictstyle diff3")
            self.run_git("config --global mergetool.prompt false")
            self.run_git("config --global diff.difftool nvimdiff")
            self.run_git(["config", "--global", "difftool.nvimdiff.cmd", "nvim -d \"$LOCAL\" \"$REMOTE\""])
            if not self.find_binary("nvim"):
                self.warning("nvim is configured, but the tool could not be found.")
        elif mergetool == "vscode":
            self.info("Setting vscode as the default GUI diff and merge tool.")
            self.run_git("config --global merge.guitool vscode")
            self.run_git(["config","--global","mergetool.vscode.cmd","code --wait \"$MERGED\""])
            if not self.find_binary("code"):
                self.warning("vscode is configured, but the tool could not be found.")
        elif mergetool == "meld":
            self.info("Setting meld as the default GUI diff and merge tool.")
            self.run_git("config --global merge.guitool meld")
            self.run_git("config","--global","mergetool.meld.cmd",
                "/usr/bin/meld \"\$LOCAL\" \"\$MERGED\" \"\$REMOTE\" --output \"\$MERGED\"",)
            self.run_git("config --global diff.guitool meld")
            self.run_git(["config","--global difftool.meld.cmd",
                "/usr/bin/meld \"\$LOCAL\" \"\$REMOTE\""])
            if not self.find_binary("meld"):
                self.warning("meld is configured, but the tool could not be found.")
        elif mergetool == "bcompare":
            self.info("Setting BeyondCompare as the default GUI diff and merge tool.")
            # Note "bc" selects a wrapper for beyondcompare that is distributed with git.
            self.run_git("config --global merge.guitool bc")
            self.run_git("config --global diff.guitool bc")
            self.run_git("config --global mergetool.bc.trustExitCode true")
            self.run_git("config --global difftool.bc.trustExitCode true")
            if not self.find_binary("bcompare"):
                self.warning("bcompare is configured, but the tool could not be found.")
        else:
            self.error("Unsupported mergetool configuration: %s", mergetool)
            self.info("Valid options are: bcompare, meld, nvim, vim, vscode")


#####################################################################
# main
#####################################################################

def main(args):
    gee = Gee()
    gee.main(args)


if __name__ == "__main__":
    main(sys.argv[1:])
