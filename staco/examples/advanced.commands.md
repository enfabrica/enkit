# Other files

Check out a [json example](/staco/examples/advanced.json) and the corresponding
[template](/staco/examples/advanced.tpl).

# Setting things up

Initial command:

    TEMPLATE=$(cat advanced.tpl)
    
    JSON=$(cat <<'END'
    {"Order": [], "Runs": {}}
    END
    )
    
    UPDATE=$(cat <<END
    if (.Order | index("$BUILD_ID") | not) then .Order |= ["$BUILD_ID"] + . else . end | .Runs *= {"$BUILD_ID": {"Time": "$(TZ='America/Los_Angeles' date)"}}
    END
    )
    
    staco post --template "$TEMPLATE" --json "$JSON" --jq-code "$UPDATE" --github-owner "$REPO_OWNER" --github-repo "$REPO_NAME" --pr "$PR" || true

# Updating the build

Let's say that one of the workers starts, and logs can be seen in a specific console, url based on build id:

    # Let user know the build has started.
    staco post --jq-code ".Runs[\"$BUILD_ID\"] += {\"BuildID\": \"$BUILD_ID\"}" \
      --github-owner "$REPO_OWNER" --github-repo "$REPO_NAME" --pr "$PR" || true

Later, another works start, that does testing:

    staco post --jq-code ".Runs[\"$BUILD_ID\"] += {\"TestID\": \"$UUID\"}" \
      --github-owner "$REPO_OWNER" --github-repo "$REPO_NAME" --pr "$PR" || true

Now, let's say one of the steps fails (a rebase, for example):

    UPDATE=$(cat <<END
    .Runs["$BUILD_ID"] += {
        "Error": "Could not rebase your PR on top of latest master correctly",
        "Hint": "How long ago did you rebase your PR? Some change in master conflicts with it? Rebase and push!",
        "Link": "https://link/to/helpful/doc"
    }
    END
    )
    staco post --jq-code "$UPDATE" --github-owner "$REPO_OWNER" --github-repo "$REPO_NAME" --pr "$PR"


Or let's say we have a command that shows recommend reviewers - it generates json:

    # Providing owners is best effort - ignore errors here.
    OWNERS="$(compute_best_owners --origin origin/master --json-format || true)"
    if [ -n "$OWNERS" ]; then
        staco post --jq-code ".Owners += $OWNERS" --github-owner "$REPO_OWNER" --github-repo "$REPO_NAME" --pr "$PR" || true
    fi

(the jq query adds the generated json in the .Owners field)

Or let's say we have a different failure:

    UPDATE=$(cat <<END
    .Runs["$BUILD_ID"] += {
        "Error": "Could not compute affected targets",
        "Hint": "Build graph file is broken? Lists files not in the PR? Look for 'ERROR:' in the log",
        "Link": "https://link/to/helpful/doc"
    }
    END
    )

