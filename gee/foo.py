# This is what Augment generated when I asked it to translate gee
# from bash to python.  It's not right, but it's a useful reference.

#!/usr/bin/env python3
"""
Git-enabled enfabrication (gee)

This script provides a user-friendly wrapper around git and GitHub CLI tools.
It implements a specific, simple, powerful workflow for git-enabled enfabrication.

Usage:
    gee <command> [options]
    gee --version
    gee --help
    gee <command> --help

Commands:
    init: Initialize a new git workspace.
    make_branch: Create a new branch.
    remove_branch: Remove a branch.
    update: Update a branch.
    rupdate: Update all branches.
    update_all: Update all branches.
    commit: Commit changes in the current branch.
    revert: Revert changes in the current branch.
    pr_checkout: Checkout a pull request.
    pr_list: List pull requests.
    pr_view: View a pull request.
    pr_submit: Submit a pull request.


TODO:

"""

import os
import sys
import subprocess
import shutil
import argparse
from pathlib import Path
from typing import List, Dict, Optional, Tuple
from dataclasses import dataclass
import json
import datetime

VERSION = "0.2.52"

@dataclass
class GeeConfig:
    version: str = VERSION
    gee_dir: str = ""
    gee_repo_dir: str = ""
    repo: str = "internal"
    upstream: str = "enfabrica"
    main_branch: str = ""
    test_mode: bool = False
    verbose: bool = True
    dry_run: bool = False
    gh_user: str = ""

class Gee:
    def __init__(self, config):
        self.config = config
        self.parents = {}
        self.mergebases = {}
        self._parents_file_is_loaded = False
        self._setup_paths()
        
    def _setup_paths(self):
        """Setup basic paths and configuration"""
        home = str(Path.home())
        self.config.gee_dir = os.getenv("GEE_DIR", os.path.join(home, "gee"))
        self.config.repo = os.getenv("GEE_REPO", "internal")
        self.config.gee_repo_dir = os.path.join(self.config.gee_dir, self.config.repo)

    def _run_cmd(self, cmd: List[str], check: bool = True) -> subprocess.CompletedProcess:
        """Run a shell command and handle errors"""
        if self.config.verbose:
            print(f"Running: {' '.join(cmd)}")
        if self.config.dry_run:
            return subprocess.CompletedProcess(cmd, 0, b"", b"")
        return subprocess.run(cmd, check=check, capture_output=True, text=True)

    def _git(self, *args: str) -> str:
        """Run a git command and return its output"""
        cmd = ["git"] + list(args)
        result = self._run_cmd(cmd)
        return result.stdout.strip()

    def _gh(self, *args: str) -> str:
        """Run a GitHub CLI command and return its output"""
        cmd = ["gh"] + list(args)
        result = self._run_cmd(cmd)
        return result.stdout.strip()
    
def _read_parents_file(self) -> None:
    """
    Read the .gee/parents file and populate the parents dictionary.
    Creates necessary directories and files if they don't exist.
    Also handles the special case for the main branch.
    """
    # Skip if already loaded
    if getattr(self, '_parents_file_is_loaded', False):
        return

    # Ensure .gee directory exists
    gee_dir = os.path.join(self.config.gee_repo_dir, ".gee")
    if not os.path.isdir(gee_dir):
        os.makedirs(gee_dir)

    # Path to parents file
    parents_file = os.path.join(gee_dir, "parents")
    
    # Create file if it doesn't exist
    if not os.path.isfile(parents_file):
        open(parents_file, 'a').close()

    # Clear existing parents
    self.parents = {}
    self.mergebases = {}

    # Read and parse the file
    try:
        with open(parents_file, 'r') as f:
            for line in f:
                # Skip empty lines
                if not line.strip():
                    continue
                
                # Parse line into components
                parts = line.strip().split()
                if len(parts) >= 2:
                    branch, parent = parts[0:2]
                    # Handle merge base if present (third column)
                    mergebase = parts[2] if len(parts) > 2 else ""
                    
                    self.parents[branch] = parent
                    self.mergebases[branch] = mergebase

    except Exception as e:
        print(f"Warning: Error reading parents file: {e}")

    # Special case: always set main branch parent to upstream/main
    self._set_main()
    self.parents[self._main] = f"upstream/{self._main}"
    self.mergebases[self._main] = ""

    # Mark as loaded
    self._parents_file_is_loaded = True

def _write_parents_file(self) -> None:
    """
    Write the parents dictionary back to the .gee/parents file.
    Includes safety checks to prevent writing empty data.
    """
    # Skip if parents file wasn't loaded
    if not getattr(self, '_parents_file_is_loaded', False):
        return

    # Safety check: don't write empty parents
    if not self.parents:
        print("Warning: Almost wrote empty parents file!")
        return

    # Ensure .gee directory exists
    gee_dir = os.path.join(self.config.gee_repo_dir, ".gee")
    if not os.path.isdir(gee_dir):
        os.makedirs(gee_dir)

    # Path to parents file
    parents_file = os.path.join(gee_dir, "parents")

    try:
        with open(parents_file, 'w') as f:
            for branch, parent in sorted(self.parents.items()):
                # Get mergebase if it exists
                mergebase = self.mergebases.get(branch, "")
                # Write all components, properly quoted
                components = [
                    shlex.quote(branch),
                    shlex.quote(parent),
                    shlex.quote(mergebase)
                ]
                f.write(f"{' '.join(components)}\n")
    except Exception as e:
        print(f"Error writing parents file: {e}")
        raise

    def init(self, repo: Optional[str] = None) -> None:
        """
        Initialize a new git workspace.
        
        Args:
            repo: Optional repository name. If not provided, uses default from config.
        """
        # Update repo configuration if provided
        if repo:
            self.config.repo = repo
            self.config.gee_repo_dir = os.path.join(self.config.gee_dir, repo)

        # Check if workspace already exists
        main_dir = os.path.join(self.config.gee_repo_dir, "main")
        if os.path.exists(main_dir):
            raise Exception(f"Initialized workspace already exists in {self.config.gee_repo_dir}")

        # Create directory structure
        os.makedirs(os.path.join(self.config.gee_repo_dir, ".gee"), exist_ok=True)

        # Install required tools and check SSH access
        self._install_tools()
        if not self._check_ssh():
            print("Cannot connect to github over ssh.")
            if self._confirm("Would you like to set up ssh access now?"):
                self._ssh_enroll()

        # Say hello (verify GitHub connectivity)
        self.hello()

        # Configure GitHub
        self._gh("config", "set", "git_protocol", "ssh")
        self._check_gh_auth()

        print(f"Initializing {self.config.gee_repo_dir} for {self.config.repo}/main")

        # Setup repository URLs
        ghuser = self._get_github_user()
        upstream = "enfabrica"  # This could be made configurable
        url = f"git@github.com:{ghuser}/{self.config.repo}.git"
        upstream_url = f"git@github.com:{upstream}/{self.config.repo}.git"

        # Create fork if needed
        try:
            repos = self._gh("repo", "list", "--json", "nameWithOwner").strip()
            if f"{ghuser}/{self.config.repo}" not in repos:
                self._gh("repo", "fork", "--clone=false", f"{upstream}/{self.config.repo}")
        except subprocess.CalledProcessError as e:
            raise Exception(f"Failed to fork repository: {e}")

        # Calculate clone depth based on months
        clone_depth_months = 3  # This could be configurable
        oldest_commit_date = (datetime.now() - datetime.timedelta(days=30 * clone_depth_months)).strftime("%Y-%m-%d")
        print(f"Fetching all commits since {oldest_commit_date}")

        # Clone repository with shallow history
        try:
            self._git("clone",
                      "--shallow-since", oldest_commit_date,
                      "--no-single-branch",
                      url,
                      main_dir)
        except subprocess.CalledProcessError as e:
            raise Exception(f"Failed to clone repository: {e}")

        # Configure remotes and fetch upstream
        os.chdir(main_dir)
        self._git("remote", "add", "upstream", upstream_url)
        self._git("fetch", "upstream")
        self._git("remote", "-v")  # Show remotes for verification

        # Fix main branch name to match upstream
        old_main = "main"
        new_main = self._set_main_by_asking_github()
        if old_main != new_main:
            os.chdir(self.config.gee_repo_dir)
            os.rename(old_main, new_main)
            os.chdir(new_main)

        print(f"Created {os.path.join(self.config.gee_repo_dir, new_main)}")

        # Set default configuration
        self.config_default()

    def _set_main_by_asking_github(self) -> str:
        """
        Determine the main branch name by asking GitHub.
        Returns the name of the main branch.
        """
        try:
            result = self._gh("repo", "view", f"{self.config.upstream}/{self.config.repo}",
                             "--json", "defaultBranchRef")
            data = json.loads(result)
            return data["defaultBranchRef"]["name"]
        except (subprocess.CalledProcessError, json.JSONDecodeError, KeyError) as e:
            print(f"Warning: Could not determine main branch name from GitHub: {e}")
            return "main"

    def config_default(self) -> None:
        """Set default configuration options."""
        # Enable git rerere (reuse recorded resolution)
        self._git("config", "--global", "rerere.enabled", "true")

        # Set default diff/merge tools based on available editors
        if self._command_exists("nvim"):
            self.config_enable_nvim()
        else:
            self.config_enable_vim()
        
        self.config_enable_meld()

    def _command_exists(self, cmd: str) -> bool:
        """Check if a command exists in the system."""
        return shutil.which(cmd) is not None

    def config_enable_vim(self) -> None:
        """Set vim as the default text-based diff and merge tool."""
        print("Setting vimdiff as the default text-based diff and merge tool.")
        self._git("config", "--global", "merge.tool", "vimdiff")
        self._git("config", "--global", "diff.tool", "vimdiff")

    def config_enable_nvim(self) -> None:
        """Set neovim as the default text-based diff and merge tool."""
        print("Setting nvimdiff as the default text-based diff and merge tool.")
        self._git("config", "--global", "merge.tool", "nvimdiff")
        self._git("config", "--global", "diff.tool", "nvimdiff")

    def config_enable_meld(self) -> None:
        """Set meld as the default GUI diff and merge tool."""
        if not self._command_exists("meld"):
            print("Meld is not installed. Skipping meld configuration.")
            return
        
        print("Setting meld as the default GUI diff and merge tool.")
        self._git("config", "--global", "merge.guitool", "meld")
        self._git("config", "--global", "diff.guitool", "meld")

    def _install_tools(self) -> None:
        """Install or verify required tools."""
        required_tools = {
            "git": "git --version",
            "gh": "gh --version",
            "ssh": "ssh -V"
        }
        
        missing_tools = []
        for tool, check_cmd in required_tools.items():
            try:
                subprocess.run(check_cmd.split(), check=True, capture_output=True)
            except (subprocess.CalledProcessError, FileNotFoundError):
                missing_tools.append(tool)
        
        if missing_tools:
            raise Exception(f"Missing required tools: {', '.join(missing_tools)}")

    def _check_ssh(self) -> bool:
        """
        Check if SSH connection to GitHub is working.
        
        Returns:
            bool: True if SSH connection is working, False otherwise
        """
        # First check if ssh-agent is running and accessible
        if "SSH_AUTH_SOCK" not in os.environ:
            print("Warning: SSH_AUTH_SOCK is not set.")
            print("Consider adding to your .bashrc: eval \"$(enkit agent print)\"")
            if self._confirm("Would you like to append this line to your .bashrc file now?"):
                with open(os.path.expanduser("~/.bashrc"), "a") as f:
                    f.write(f'\neval "$(enkit agent print)"\n')
        
        # Try to start the agent now
        try:
            result = subprocess.run(
                ["enkit", "agent", "print"],
                capture_output=True,
                text=True,
                check=True
            )
            for line in result.stdout.splitlines():
                if "=" in line:
                    key, value = line.split("=", 1)
                    value = value.rstrip(";")
                    os.environ[key] = value
        except subprocess.CalledProcessError:
            return False

        if "SSH_AUTH_SOCK" not in os.environ:
            print("Something is wrong with enkit's ssh agent.")
            return False

        # Check if ssh-agent is responsive
        try:
            subprocess.run(
                ["ssh-add", "-l"],
                timeout=2,
                capture_output=True,
                check=True
            )
        except subprocess.TimeoutExpired:
            print("Timeout while communicating with ssh-agent")
            return False
        except subprocess.CalledProcessError as e:
            if e.returncode == 2:
                print("Unable to communicate with enkit's ssh agent.")
                return False

        # Check enkit certificate
        try:
            result = subprocess.run(
                ["enkit", "agent", "print"],
                capture_output=True,
                text=True,
                check=True
            )
            if not result.stdout.strip():
                print("Warning: enkit certificate is expired.")
                print("Please authenticate:")
                subprocess.run(
                    ["enkit", "auth", f"{os.environ.get('USER', '')}@enfabrica.net"],
                    check=True
                )
                result = subprocess.run(
                    ["enkit", "agent", "print"],
                    capture_output=True,
                    text=True,
                    check=True
                )
                if not result.stdout.strip():
                    print("No enkit certificate, aborting.")
                    return False
        except subprocess.CalledProcessError:
            return False

        # Check if keys are loaded in ssh-agent
        try:
            result = subprocess.run(
                ["ssh-add", "-l"],
                timeout=2,
                capture_output=True,
                text=True
            )
            if not result.stdout.strip():
                print("enkit's certificate isn't showing up in ssh-agent.")
                return False
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
            return False

        # Finally, test GitHub SSH connection
        try:
            result = subprocess.run(
                ["ssh", "-T", "git@github.com"],
                input=b"",  # Equivalent to </dev/null in bash
                capture_output=True,
                text=True
            )
            if "Hi " in result.stderr:
                self.ghuser = result.stderr.split()[1]
                return True
        except subprocess.CalledProcessError as e:
            if e.stderr and "Hi " in e.stderr:
                self.ghuser = e.stderr.split()[1]
                return True
            print("Could not authenticate to github using ssh.")
            print(f"ssh -T git@github.com got:\n  {e.stderr}")

            # Try adding SSH key if it exists
            if os.path.exists(self.config.ssh_key_file):
                print(f"Perhaps you need to run: ssh-add {self.config.ssh_key_file}")
                try:
                    subprocess.run(["ssh-add", self.config.ssh_key_file], check=True)
                    print("Trying again...")
                    result = subprocess.run(
                        ["ssh", "-T", "git@github.com"],
                        input=b"",
                        capture_output=True,
                        text=True
                    )
                    if "Hi " in (result.stderr or ""):
                        self.ghuser = result.stderr.split()[1]
                        return True
                    print("Still couldn't authenticate to github using ssh.")
                    print(f"ssh -T git@github.com got:\n  {result.stderr}")
                except subprocess.CalledProcessError:
                    pass

        print("After repeated attempts, could not connect to github using ssh.")
        return False

    def _ssh_enroll(self) -> None:
        """Set up SSH access to GitHub."""
        # Generate SSH key if needed
        ssh_dir = os.path.expanduser("~/.ssh")
        key_file = os.path.join(ssh_dir, "id_ed25519")
        
        if not os.path.exists(key_file):
            os.makedirs(ssh_dir, mode=0o700, exist_ok=True)
            email = self._git("config", "user.email").strip()
            if not email:
                email = input("Enter your GitHub email: ").strip()
                self._git("config", "--global", "user.email", email)
            
            subprocess.run([
                "ssh-keygen",
                "-t", "ed25519",
                "-C", email,
                "-f", key_file,
                "-N", ""
            ], check=True)

        # Add key to ssh-agent
        try:
            subprocess.run(["ssh-add", key_file], check=True)
        except subprocess.CalledProcessError:
            # Start ssh-agent if needed
            eval_cmd = subprocess.run(
                ["ssh-agent", "-s"],
                capture_output=True,
                text=True,
                check=True
            )
            for line in eval_cmd.stdout.splitlines():
                if "=" in line:
                    key, value = line.split("=", 1)
                    value = value.rstrip(";")
                    os.environ[key] = value
            subprocess.run(["ssh-add", key_file], check=True)

        # Add key to GitHub
        with open(f"{key_file}.pub") as f:
            pubkey = f.read().strip()
        
        try:
            self._gh("ssh-key", "add", pubkey)
        except subprocess.CalledProcessError:
            print("Failed to automatically add SSH key to GitHub.")
            print("Please add the following public key manually:")
            print(pubkey)
            input("Press Enter after adding the key to GitHub...")

    def _check_gh_auth(self) -> None:
        """Verify GitHub CLI authentication."""
        try:
            self._gh("auth", "status")
        except subprocess.CalledProcessError:
            print("GitHub CLI not authenticated. Running authentication...")
            self._gh("auth", "login", "--web")

    def _get_github_user(self) -> str:
        """
        Get GitHub username.
        
        Returns:
            str: GitHub username
        """
        try:
            return self._gh("api", "user", "--jq", ".login").strip()
        except subprocess.CalledProcessError as e:
            raise Exception(f"Failed to get GitHub username: {e}")

    def _confirm(self, prompt: str) -> bool:
        """
        Ask for user confirmation.
        
        Args:
            prompt: The prompt to show to the user
        
        Returns:
            bool: True if user confirms, False otherwise
        """
        response = input(f"{prompt} (y/N) ").strip().lower()
        return response in ['y', 'yes']

    def make_branch(self, branch_name: str, base: Optional[str] = None) -> None:
        """Create a new branch"""
        if not base:
            base = self._get_current_branch() or f"upstream/{self.config.main_branch}"

        if base.startswith("upstream/"):
            self._git("fetch", "upstream")

        branch_path = os.path.join(self.config.gee_repo_dir, branch_name)
        self._git("worktree", "add", "-b", branch_name, "-f", branch_path, base)
        print(f"Created {branch_path}")

    def _get_current_branch(self) -> str:
        """
        Get the name of the current branch.
        
        Returns:
            str: Name of the current branch, or empty string if not in a git repository
        """
        try:
            return self._git("rev-parse", "--abbrev-ref", "HEAD").strip()
        except subprocess.CalledProcessError:
            return ""

    def _add_parent_branches_to_chain(self, branch: str, chain: List[str]) -> None:
        """
        Recursively finds all parent branches of branch and inserts them into chain
        unless they are already in chain. The branches in chain will be strictly
        ordered so that any parent is earlier than any child.
        
        Args:
            branch: The branch to process
            chain: List to store the ordered branches
        """
        if branch in chain:
            return

        # Stop if branch is upstream
        if branch.startswith("upstream/"):
            return

        parent = self._get_parent_branch(branch)
        
        # Stop if parent is the same as current branch
        if parent == branch:
            return

        # Keep recursing until we reach a branch whose parent is remote
        if "/" not in parent:
            self._add_parent_branches_to_chain(parent, chain)
        
        chain.append(branch)

    def rupdate(self) -> None:
        """
        Recursively integrate changes from parents into the current branch.
        Equivalent to the original gee rupdate command.
        """
        self._startup_checks("rupdate")
        self._check_cwd()

        current_branch = self._get_current_branch()
        if not current_branch:
            raise ValueError("Not in a git branch directory.")

        self._read_parents_file()

        # Build a chain of branches to update
        self._set_main()
        chain: List[str] = []
        self._add_parent_branches_to_chain(current_branch, chain)

        print(f"Updating branches in order: {', '.join(chain)}")

        for branch in chain:
            parent_branch = self._get_parent_branch(branch)
            print(f"\nUpdating branch '{branch}' from '{parent_branch}'")
            self._checkout_or_die(branch)

            # Check for uncommitted changes if not the current branch
            if branch != current_branch:
                uncommitted = self._git("status", "--short", "-uall").splitlines()
                if uncommitted:
                    error_msg = [
                        f"Branch {branch} contains {len(uncommitted)} uncommitted changes:",
                        *uncommitted,
                        f"Commit branch {branch} and try again."
                    ]
                    raise RuntimeError("\n".join(error_msg))

            # If parent is a remote, make sure remote is up to date
            if "/" in parent_branch:
                remote = parent_branch.split("/")[0]
                self._git("fetch", remote)

            # Rebase from parent onto branch
            if not self._safer_rebase(branch, parent_branch):
                print(f"Warning: Conflicts occurred while updating {branch}")
                return

        print("Update complete.")

    def gee__rup(self, *args: str) -> None:
        """Alias for rupdate"""
        self.rupdate(*args)

    def gee__rupdate(self, *args: str) -> None:
        """Alias for rupdate"""
        self.rupdate(*args)

    def _safer_rebase(self, child_branch: str, parent_branch: str, onto_branch: Optional[str] = None) -> bool:
        """
        Performs a safe version of "git rebase parent_branch child_branch".
        If a merge conflict occurs, invokes interactive conflict resolution.

        Args:
            child_branch: The branch to rebase
            parent_branch: The branch to rebase onto
            onto_branch: Optional branch for --onto operation

        Returns:
            bool: True if rebase was successful, False otherwise
        """
        # Enable rerere if not already enabled
        if self._git("config", "--global", "rerere.enabled") != "true":
            self._git("config", "--global", "rerere.enabled", "true")

        # Check for open PRs
        open_prs = self._list_open_pr_numbers(child_branch)
        if open_prs:
            print(f"Warning: Open PR exists for branch {child_branch}: {', '.join(open_prs)}")
            print("If a reviewer is already looking at your PR, rebasing this branch")
            print("will break the reviewer's ability to see what has changed when")
            print("you commit new changes. Are you sure you want to do this?")
            if not self._confirm_default_no():
                print(f"Skipped update of branch {child_branch}")
                return True

        # Create backup tag
        backup_tag = f"{child_branch}.REBASE_BACKUP"
        self._git("tag", "-f", backup_tag)

        # Check for uncommitted changes
        uncommitted = self._git("status", "--short", "-uall").splitlines()
        if uncommitted:
            print(f"Warning: Branch {child_branch} contains {len(uncommitted)} uncommitted changes:")
            for change in uncommitted:
                print(change)

        # Start the rebase
        rebase_cmd = ["rebase", "--autostash"]
        if onto_branch:
            rebase_cmd.extend(["--onto", onto_branch])
        rebase_cmd.extend([parent_branch, child_branch])

        try:
            self._git(*rebase_cmd)
        except subprocess.CalledProcessError:
            if self._is_rebase_in_progress():
                print("Rebase operation had merge conflicts.")
                self._interactive_conflict_resolution(parent_branch, child_branch, onto_branch)

                if self._is_rebase_in_progress():
                    status = self._git("status").splitlines()
                    print("\n".join(status))
                    print(f"Merge conflict in branch {child_branch}, must be manually resolved.")
                    return False

                # Verify rebase success
                if self._git("merge-base", "--is-ancestor", parent_branch, "HEAD").returncode == 0:
                    print("Rebase merge confirmed.")
                else:
                    print("Rebase did not succeed, aborting.")
                    return False
            else:
                print("Rebase failed for an unknown reason.")
                return False

        self._check_diff_for_merge_conflict_markers()
        print(f"To undo: git checkout {child_branch}; git reset --hard {backup_tag}")
        self._push_to_origin(f"+{child_branch}")
        return True

    def _interactive_conflict_resolution(self, parent: str, child: str, onto: Optional[str] = None) -> None:
        """Handle merge conflicts interactively during rebase."""
        while self._is_rebase_in_progress():
            # Get current commit information
            onto_commit = self._git("rev-parse", "HEAD")
            onto_desc = self._git("show", "--oneline", "-s", onto_commit)
            from_commit = self._git("rev-parse", "REBASE_HEAD")
            from_desc = self._git("show", "--oneline", "-s", from_commit)

            print(f"Attempting to apply: {from_desc}")
            print(f"               onto: {onto_desc}")

            status = self._git("status", "--porcelain").splitlines()
            if not status:
                print("Empty commit, skipping automatically.")
                self._git("rebase", "--skip")
                continue

            for status_line in status:
                file_status = status_line[:2]
                file_path = status_line[3:]
                print(f"{file_path}: {file_status}")

                while True:
                    response = input(
                        "Keep (Y)ours, keep (T)heirs, (M)erge, (G)ui-Merge, (V)iew, (A)bort, (P)ick, or (H)elp? "
                    ).strip().lower()

                    if response in ["y", "t", "m", "g", "v", "a", "p", "h"]:
                        break
                    print("Invalid choice. Please try again.")

                if response == "y":
                    print(f"{file_path}: Keeping your version from {from_desc}")
                    print(f"{file_path}: Discarding their version from {onto_desc}")
                    if file_status == "ud":
                        self._git("mergetool", "--", file_path, input="d\n")
                    else:
                        self._git("checkout", "--theirs", file_path)
                elif response == "t":
                    print(f"{file_path}: Discarding your version from {from_desc}")
                    print(f"{file_path}: Keeping their version from {onto_desc}")
                    self._git("checkout", "--ours", file_path)
                elif response == "m":
                    self._git("mergetool", file_path)
                elif response == "g":
                    gui_tool = self._git("config", "--get", "merge.guitool").strip() or "meld"
                    self._git("mergetool", "--tool", gui_tool, "--no-prompt", file_path)
                elif response == "v":
                    print(self._git("status"))
                    print(self._git("diff", file_path))
                elif response == "a":
                    print("Aborting rebase.")
                    self._git("rebase", "--abort")
                    return
                elif response == "p":
                    print("\"Pick\" will abort and restart your rebase merge from the beginning.")
                    if self._confirm_default_no("Are you sure you want to restart? (y/N) "):
                        self._git("rebase", "--abort")
                        rebase_cmd = ["rebase", "--autostash", "-i"]
                        if onto:
                            rebase_cmd.extend(["--onto", onto])
                        rebase_cmd.extend([parent, child])
                        try:
                            self._git(*rebase_cmd)
                        except subprocess.CalledProcessError:
                            if not self._is_rebase_in_progress():
                                raise Exception("Rebase command failed, but rebase is not in progress. Bug!")
                        return
                elif response == "h":
                    self._print_merge_help()

            # Check for remaining conflict markers
            if not self._git("diff", "--check"):
                self._git("add", ".")
                self._git("rebase", "--continue")
            else:
                print("Cannot proceed with rebase: conflict markers still present.")
                print("Try again...")

    def _is_rebase_in_progress(self) -> bool:
        """Check if a rebase is currently in progress."""
        git_dir = Path(self._git("rev-parse", "--git-dir").strip())
        return any((git_dir / path).exists() for path in ["rebase-apply", "rebase-merge"])

    def _check_diff_for_merge_conflict_markers(self) -> None:
        """Check if there are any merge conflict markers in the diff."""
        try:
            output = self._git("diff")
            if any(line.startswith(("<<<<<<", "======", ">>>>>>")) for line in output.splitlines()):
                print("Warning: Uncommited files contain conflict markers: please resolve.")
        except subprocess.CalledProcessError:
            pass

    def _confirm_default_no(self, prompt: str = "Continue? (y/N) ") -> bool:
        """Ask for confirmation, defaulting to No."""
        response = input(prompt).strip().lower()
        return response in ['y', 'yes']

    def _print_merge_help(self) -> None:
        """Print help text for merge conflict resolution."""
        help_text = """
        Help:
          Y = Yours: discard their changes, keep yours
          T = Theirs: discard your changes, keep theirs
          M = Merge: invoke the merge resolution tool
          G = Guimerge: invoke the GUI merge tool
          V = View: view the conflict
          A = Abort: abort the rebase
          P = Pick: restart rebase in interactive mode
          H = Help: this text
        """
        print(help_text)

    def _list_open_pr_numbers(self, branch: str) -> List[str]:
        """List open PR numbers for the given branch."""
        try:
            output = self._gh("pr", "list", "--head", branch, "--state", "open", "--json", "number")
            prs = json.loads(output)
            return [str(pr["number"]) for pr in prs]
        except (subprocess.CalledProcessError, json.JSONDecodeError):
            return []

    def _push_to_origin(self, ref: str) -> None:
        """Push a ref to origin, handling force pushes."""
        try:
            self._git("push", "origin", ref)
        except subprocess.CalledProcessError as e:
            print(f"Push failed: {e}")

    def update(self) -> None:
        """
        Integrate changes from parent into this branch.
        Equivalent to the original gee update command.
        """
        self._startup_checks("update")
        self._check_cwd()

        current_branch = self._get_current_branch()
        if not current_branch:
            raise ValueError("Not in a git branch directory.")

        # Check for upstream changes in "origin" first
        if self._remote_branch_exists("origin", current_branch):
            self._git("fetch", "origin")
            
            # Get the counts of commits ahead/behind
            counts_output = self._git("rev-list", "--left-right", "--count", 
                                    f"{current_branch}...origin/{current_branch}")
            ahead, behind = map(int, counts_output.strip().split())
            
            if behind > 0:
                print(f"Warning: origin/{current_branch} contains {behind} commit(s) " 
                      "not in your local branch.")
                print("You may need to 'git push -u origin --force' to fix your origin remote.")

        self._read_parents_file()
        parent_branch = self._get_parent_branch(current_branch)

        # If parent is main/master, update it first
        self._set_main()
        if parent_branch == self._main:
            self._update_main()

        # Checkout current branch and rebase from parent
        self._checkout_or_die(current_branch)
        if not self._safer_rebase(current_branch, parent_branch):
            print(f"Warning: Conflicts occurred while updating {current_branch}")
            return
        
        print("Update complete.")

    def _update_main(self) -> None:
        """Merge from upstream/main into main."""
        current_branch = self._get_current_branch()
        
        # Checkout main
        self._set_main()
        self._checkout_or_die(self._main)
        
        # Check for local changes
        uncommitted = self._git("diff", "--name-only").splitlines()
        if uncommitted:
            print(f"Warning: {self._main} branch contains {len(uncommitted)} uncommitted changes.")
        
        # Update from upstream
        self._update_from_upstream(self._main, self._main)
        
        # Return to original branch
        self._checkout_or_die(current_branch)

    def _update_from_upstream(self, branch: str, upstream_branch: str) -> None:
        """
        Update a branch from its upstream counterpart.
        
        Args:
            branch: The local branch to update
            upstream_branch: The upstream branch to pull from
        """
        try:
            self._git("fetch", "upstream")
            self._safer_rebase(branch, f"upstream/{upstream_branch}")
        except subprocess.CalledProcessError as e:
            print(f"Error updating from upstream: {e}")
            raise

    def _remote_branch_exists(self, remote: str, branch: str) -> bool:
        """
        Check if a branch exists in the remote repository.
        
        Args:
            remote: Remote name (e.g., 'origin', 'upstream')
            branch: Branch name to check
        
        Returns:
            bool: True if branch exists in remote, False otherwise
        """
        try:
            self._git("rev-parse", "--verify", f"{remote}/{branch}")
            return True
        except subprocess.CalledProcessError:
            return False

    def gee__up(self, *args: str) -> None:
        """Alias for update"""
        self.update(*args)

    def gee__update(self, *args: str) -> None:
        """Alias for update"""
        self.update(*args)

    def gcd(self, target: str, create_branch: bool = False, from_main: bool = False, parent: Optional[str] = None) -> str:
        """
        Change directory to another branch.
        
        Args:
            target: Target branch specification (branch or branch/path)
            create_branch: If True, create branch if it doesn't exist
            from_main: If True, create branch from main branch
            parent: Specific parent branch to create from
        
        Returns:
            str: Path to change to
        """
        if not target:
            raise ValueError("Target branch argument not specified.")

        self._set_main()

        # If not in a gee branch, start from main
        if not self._in_gee_branch():
            os.chdir(os.path.join(self.config.gee_repo_dir, self.config.main_branch))
            if not self._in_gee_branch():
                raise Exception(f"Can't find {self.config.repo}/{self.config.main_branch} branch.")

        # Parse target specification
        if "/" in target:
            # Target is <branch>/<directory>
            branch, rel_path = target.split("/", 1)
        else:
            # Target is just branch name
            branch = target
            rel_path = self._git("rev-parse", "--show-prefix").strip()

        # Check if branch exists
        if not self._local_branch_exists(branch):
            if create_branch or from_main or parent:
                # Create new branch
                if from_main:
                    base = f"upstream/{self.config.main_branch}"
                elif parent:
                    base = parent
                else:
                    base = self._get_current_branch()
                
                self.make_branch(branch, base)
            else:
                raise Exception(f"Branch '{branch}' does not exist.")

        # Construct target path
        branch_root = os.path.join(self.config.gee_repo_dir, branch)
        target_path = os.path.join(branch_root, rel_path)

        # Verify path exists
        if not os.path.exists(target_path):
            os.makedirs(target_path, exist_ok=True)

        return target_path

    def bash_setup(self) -> str:
        """
        Generate bash setup script.
        
        Returns:
            str: Bash script content for setting up gee environment
        """
        script = """# bash functions for gee
#
# This output is meant to be loaded into your shell with this command:
#
#   eval "$(gee bash_setup)"

function gee() {
    # Use locally installed gee if available
    if [[ -n "${GEE_BINARY}" ]]; then
        "${GEE_BINARY}" "$@"
    elif [[ -x ~/bin/gee ]]; then
        ~/bin/gee "$@"
    else
        # Search PATH for gee
        local path
        path="$(which gee)"
        if [[ -z "${path}" ]]; then
            echo "Cannot find gee executable!"
            return 1
        fi
        "${path}" "$@"
    fi
}

function gcd() {
    if (( "$#" == 0 )); then
        gee help gcd
        return 1
    fi

    local D="$(gee gcd "$@")"
    if [[ -n "${D}" ]]; then
        cd "${D}"
        export BROOT="$(git rev-parse --show-toplevel)"
        export BRBIN="${BROOT}/bazel-bin"
    fi
}

function grg() {
    gee grep "$@"
}

function _gee_completion_branches() {
    shift  # discard
    local REPO="${GEE_REPO:-}"
    local GD="${GEE_DIR:-"${HOME}/gee"}"
    if [[ -z "${REPO}" ]]; then
        local DIR="$(realpath --relative-base="${GD}" "${PWD}")"
        if [[ -n "${DIR}" ]] && [[ "${DIR}" != "." ]] && [[ "${DIR}" != /* ]]; then
            REPO="$(cut -d/ -f1 <<< "${DIR}")"
        fi
    fi
    local GRD="${GEE_REPO_DIR:-"${GD}/${REPO:-internal}"}"
    COMPREPLY=($(cd "${GRD}"; compgen -f -X \\.* "$@"))
}

# Command completions
function _gee_completion() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    case "${COMP_CWORD}" in
        1)
            COMPREPLY=($(compgen -W "init gcd make_branch commit update fix share rupdate" -- "${cur}"))
            ;;
        2)
            prev="${COMP_WORDS[COMP_CWORD-1]}"
            case "${prev}" in
                gcd|make_branch)
                    _gee_completion_branches "$@"
                    ;;
                *)
                    COMPREPLY=()
                    ;;
            esac
            ;;
        *)
            COMPREPLY=()
            ;;
    esac
}

# Set up command completion
complete -F _gee_completion gee

# Set up prompt customization
function gee_prompt_git_info() {
    local branch
    branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null)"
    if [[ -n "${branch}" ]]; then
        echo "(${branch})"
    fi
}

function gee_prompt_print() {
    local exit_code="$?"
    local prompt_color="${GEE_PROMPT_COLOR:-1}"
    local fg2_color="${GEE_PROMPT_FG2_COLOR:-3}"
    
    PS1="\\[\\e[3${prompt_color}m\\]\\u@\\h\\[\\e[0m\\]:"
    PS1+="\\[\\e[3${fg2_color}m\\]\\w\\[\\e[0m\\]"
    PS1+="\\[\\e[3${prompt_color}m\\]$(gee_prompt_git_info)\\[\\e[0m\\]"
    PS1+="\\$ "
    
    return ${exit_code}
}

function gee_prompt_set_ps1() {
    PROMPT_COMMAND=gee_prompt_print
}

function gee_prompt_test_colors() {
    for i in {0..7}; do
        echo -e "\\e[3${i}mColor $i\\e[0m"
    done
}

# Export GEE_BINARY
export GEE_BINARY="$(which gee)"
"""
        return script

    def _local_branch_exists(self, branch: str) -> bool:
        """
        Check if a local branch exists.
        
        Args:
            branch: Branch name to check
        
        Returns:
            bool: True if branch exists, False otherwise
        """
        try:
            self._git("rev-parse", "--verify", branch)
            return True
        except subprocess.CalledProcessError:
            return False

    def _in_gee_branch(self) -> bool:
        """
        Check if current directory is in a gee branch.
        
        Returns:
            bool: True if in gee branch, False otherwise
        """
        try:
            current_dir = os.getcwd()
            gee_dir = os.path.realpath(self.config.gee_dir)
            return current_dir.startswith(gee_dir)
        except OSError:
            return False

    def remove_branch(self, *branch_names: str) -> None:
        """
        Remove one or more branches and their associated directories.
        
        Args:
            *branch_names: Names of branches to remove
        
        Raises:
            ValueError: If no branch names are provided
        """
        self._startup_checks("remove_branch")

        if not branch_names:
            raise ValueError("Must specify at least one branch name to remove.")

        for branch in branch_names:
            self._remove_a_branch(branch)

    def _remove_a_branch(self, branch: str) -> None:
        """
        Remove a single branch and its associated directory.
        
        Args:
            branch: Name of the branch to remove
        """
        print(f"\nDeleting {branch}")
        
        # Check if branch exists locally or has a worktree directory
        branch_exists = (
            self._git("show-ref", "--quiet", f"refs/heads/{branch}", check=False).returncode == 0 or
            os.path.isdir(os.path.join(self.config.gee_repo_dir, branch))
        )

        if branch_exists:
            # Get SHA for potential undo message
            try:
                sha = self._git("rev-parse", branch).strip()
            except subprocess.CalledProcessError:
                sha = None

            # Clean up bazel cache if it exists
            worktree_path = os.path.join(self.config.gee_repo_dir, branch)
            if os.path.exists(worktree_path):
                try:
                    # Change to worktree directory to check for bazel cache
                    os.chdir(worktree_path)
                    if os.path.exists("bazel-out"):
                        bazel_cache = os.path.realpath("bazel-out")
                        bazel_cache = bazel_cache.rsplit("/execroot/", 1)[0]
                        print(f"Removing linked bazel cache directory \"{bazel_cache}\"")
                        # Ensure write permissions
                        subprocess.run(["chmod", "-R", "u+w", bazel_cache], check=True)
                        shutil.rmtree(bazel_cache)
                finally:
                    # Always return to main branch directory
                    os.chdir(self.config.gee_dir)

            # Switch to main branch before removing
            self._checkout_or_die(self.config.main_branch)
            
            # Remove worktree and branch
            try:
                self._git("worktree", "remove", "--force", branch)
            except subprocess.CalledProcessError:
                print(f"Warning: Failed to remove worktree for {branch}")
            
            try:
                self._git("branch", "-D", branch)
            except subprocess.CalledProcessError:
                print(f"Warning: Failed to delete local branch {branch}")

            if sha:
                print(f"Deleted local branch {branch}. To undo: gee make_branch {branch} {sha}")
        else:
            print(f"Local branch \"{branch}\" not found: skipped.")

        # Remove remote branch if it exists
        if self._remote_branch_exists("origin", branch):
            try:
                self._git("push", "--quiet", "origin", "--delete", branch)
                print(f"Deleted remote branch origin/{branch}")
            except subprocess.CalledProcessError:
                print(f"Warning: Failed to delete remote branch origin/{branch}")
        else:
            print(f"Remote branch origin/{branch} not found: skipped.")

        # Update parents file
        self._read_parents_file()
        prev_parent = self.parents.get(branch, self.config.main_branch)
        
        # Remove branch from parents
        if branch in self.parents:
            del self.parents[branch]
        
        # Update children's parents
        for child, parent in list(self.parents.items()):
            if parent == branch:
                self.parents[child] = prev_parent
        
        self._write_parents_file()

    def gee__rmbr(self, *args: str) -> None:
        """Alias for remove_branch"""
        self.remove_branch(*args)

    def update_all(self) -> None:
        """
        Update all local branches by rebasing them onto their respective parents.
        Branches are updated in dependency order (parents before children).
        """
        self._startup_checks("update_all")
        self._read_parents_file()

        # Get all local branches with worktrees
        branches = []
        result = self._git("worktree", "list", "--porcelain")
        for line in result.splitlines():
            if line.startswith("worktree "):
                path = line.split()[1]
                branch = os.path.basename(path)
                if not branch.startswith("."):  # Skip hidden directories
                    branches.append(branch)

        # Build dependency chain for all branches
        chain: List[str] = []
        for branch in branches:
            self._add_parent_branches_to_chain(branch, chain)

        if not chain:
            print("No branches to update.")
            return

        print(f"Updating branches in order: {', '.join(chain)}")
        original_branch = self._get_current_branch()

        try:
            for branch in chain:
                print(f"\nUpdating branch '{branch}'")
                parent = self._get_parent_branch(branch)
                
                # Skip if parent doesn't exist
                if not parent.startswith("upstream/") and not self._local_branch_exists(parent):
                    print(f"Parent branch '{parent}' not found, skipping.")
                    continue

                self._checkout_or_die(branch)
                
                # Check for uncommitted changes
                if self._git("status", "--porcelain"):
                    print(f"Branch '{branch}' has uncommitted changes, skipping.")
                    continue

                if not self._safer_rebase(branch, parent):
                    print(f"Failed to update branch '{branch}', skipping.")
                    continue

        finally:
            # Return to original branch
            if original_branch:
                self._checkout_or_die(original_branch)

    def lsbranches(self) -> None:
        """
        List all branches and their relationships.
        Shows current branch, parent branches, and child branches.
        """
        self._startup_checks("lsbranches")
        self._read_parents_file()

        # Get all branches with worktrees
        worktrees = {}
        result = self._git("worktree", "list", "--porcelain")
        current_worktree = None
        current_branch = None

        for line in result.splitlines():
            if line.startswith("worktree "):
                current_worktree = line.split()[1]
            elif line.startswith("branch "):
                branch = line.split()[1].replace("refs/heads/", "")
                if not branch.startswith("."):
                    worktrees[branch] = current_worktree
                    current_worktree = None

        # Build branch hierarchy
        branch_info = {}
        for branch in worktrees:
            parent = self._get_parent_branch(branch)
            if branch not in branch_info:
                branch_info[branch] = {"parent": parent, "children": []}
            if parent in branch_info:
                branch_info[parent]["children"].append(branch)

        # Get current branch
        current = self._get_current_branch()

        # Print branch hierarchy
        def print_branch(branch: str, level: int = 0, prefix: str = "") -> None:
            info = branch_info.get(branch, {})
            marker = "* " if branch == current else "  "
            print(f"{prefix}{marker}{branch}")
            
            children = sorted(info.get("children", []))
            for i, child in enumerate(children):
                is_last = i == len(children) - 1
                new_prefix = prefix + ("    " if is_last else "│   ")
                print_branch(child, level + 1, new_prefix + ("└── " if is_last else "├── "))

        # Find root branches (those with no local parents or upstream parents)
        roots = []
        for branch in worktrees:
            parent = branch_info[branch]["parent"]
            if parent.startswith("upstream/") or parent not in worktrees:
                roots.append(branch)

        # Print the tree
        print("\nBranch hierarchy:")
        for root in sorted(roots):
            print_branch(root)

    def whatsout(self) -> None:
        """
        Show what commits are ready to push to origin for each branch.
        Also shows if branches are ahead/behind their parents.
        """
        self._startup_checks("whatsout")
        self._read_parents_file()
        
        # Get all local branches with worktrees
        branches = []
        result = self._git("worktree", "list", "--porcelain")
        for line in result.splitlines():
            if line.startswith("worktree "):
                path = line.split()[1]
                branch = os.path.basename(path)
                if not branch.startswith("."):
                    branches.append(branch)

        current_branch = self._get_current_branch()
        
        for branch in sorted(branches):
            print(f"\nBranch: {branch}")
            
            # Check against origin
            if self._remote_branch_exists("origin", branch):
                try:
                    ahead_behind = self._git(
                        "rev-list", "--left-right", "--count",
                        f"{branch}...origin/{branch}"
                    ).strip().split()
                    ahead, behind = map(int, ahead_behind)
                    
                    if ahead > 0:
                        print(f"  {ahead} commit(s) ahead of origin/{branch}")
                    if behind > 0:
                        print(f"  {behind} commit(s) behind origin/{branch}")
                    if ahead == behind == 0:
                        print("  In sync with origin")
                except subprocess.CalledProcessError:
                    print("  Failed to compare with origin")
            else:
                print("  No corresponding branch in origin")

            # Check against parent
            parent = self._get_parent_branch(branch)
            if parent and not parent.startswith("upstream/"):
                try:
                    ahead_behind = self._git(
                        "rev-list", "--left-right", "--count",
                        f"{branch}...{parent}"
                    ).strip().split()
                    ahead, behind = map(int, ahead_behind)
                    
                    if ahead > 0:
                        print(f"  {ahead} commit(s) ahead of parent {parent}")
                    if behind > 0:
                        print(f"  {behind} commit(s) behind parent {parent}")
                except subprocess.CalledProcessError:
                    print(f"  Failed to compare with parent {parent}")

            # Show uncommitted changes
            if branch == current_branch:
                status = self._git("status", "--porcelain")
                if status:
                    changes = status.splitlines()
                    print(f"  {len(changes)} uncommitted change(s)")

    def gee__lsb(self, *args: str) -> None:
        """Alias for lsbranches"""
        self.lsbranches(*args)

    def gee__lsbr(self, *args: str) -> None:
        """Alias for lsbranches"""
        self.lsbranches(*args)

    def gee__upall(self, *args: str) -> None:
        """Alias for update_all"""
        self.update_all(*args)

    def hello(self) -> None:
        """
        Check connectivity to GitHub.
        
        Verifies that the user can communicate with GitHub using SSH and the
        GitHub CLI interface.
        """
        # First verify SSH connectivity
        if not self._check_ssh():
            raise Exception("Could not determine github username.")
        
        # Print welcome message if not in quiet mode
        if not hasattr(self, '_quiet') or not self._quiet:
            ghuser = self._get_github_user()
            print(f"Hello, {ghuser}. Connectivity to github is AOK.")
        
        # Ensure GitHub CLI is authenticated
        self._gh_authenticate()

    def gee__hello(self, *args: str) -> None:
        """Command wrapper for hello"""
        self.hello(*args)

def main():
    parser = argparse.ArgumentParser(description="Git-enabled enfabrication (gee)")
    parser.add_argument('command', help='Command to execute')
    parser.add_argument('args', nargs='*', help='Command arguments')
    parser.add_argument('--version', action='version', version=f'gee version {VERSION}')

    args = parser.parse_args()

    gee = Gee()
    
    # Map commands to methods
    commands = {
        'init': gee.init,
        'make_branch': gee.make_branch,
        'rupdate': gee.rupdate,
        'update': gee.update,
        'up': gee.gee__up,
        'geerup': gee.gee__rup,
        'geerupdate': gee.gee__rupdate,
        'gcd': gee.gcd,
        'bash_setup': gee.bash_setup,
        'rmbr': gee.gee__rmbr,
        'remove_branch': gee.remove_branch,
        'update_all': gee.update_all,
        'lsbranches': gee.lsbranches,
        'whatsout': gee.whatsout,
        'geelsb': gee.gee__lsb,
        'geelsbr': gee.gee__lsbr,
        'geupall': gee.gee__upall,
        'hello': gee.gee__hello,
        # Add more commands here
    }

    if args.command in commands:
        try:
            commands[args.command](*args.args)
        except Exception as e:
            print(f"Error: {e}", file=sys.stderr)
            sys.exit(1)
    else:
        print(f"Unknown command: {args.command}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
