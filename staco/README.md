[![Go Reference](https://pkg.go.dev/badge/github.com/enfabrica/enkit/lib/github.svg)](https://pkg.go.dev/github.com/enfabrica/enkit/lib/github)

# Overview

staco is a Command Line tool to create and manage "STAble COmments" on github.

Stable comments are comments posted on a PR that can be updated by automation
to act as a mini-dashboard.

Let's say, for example, that you have a BOT that on every PR does some analysis,
and needs to post the link to a generated report. Every time the PR is updated,
a new report is generated. But you don't want a new comment to be posted on the
PR every time! Rather, you'd like a comment to be updated with the
latest link, and maybe maintain a little bit of history with the links to
the previous analysis.

staco combines:
* text/template - to define the format of your stable comment, with support
  for the [srpig extensions](http://masterminds.github.io/sprig/).
* json - to define the data to be displayed (and updated) in the comment.
* json patches - in various formats (thanks to the [jd library](https://pkg.go.dev/github.com/josephburnett/jd@v1.6.1/lib)),
  describing how to update the data displayed in comments.

# Example

Let's say you have a CI that - while it is running, and before completion - generates
various links that are useful for the developer to verify the progress of the CI
run (for example: analysis logs, coverage, etc).

You want the generated comment to look like this (in github markdown/html):

    Build reports for the latest run (18:56, Monday 15th) available here:
    * [Static analysis](http://static-analysis)
    * [Dynamic simulation](http://dynamic-analysis)
    
    <details>
      <summary>Past runs</summary>
      <p>
        <ul>
          <li>18:56 Wednesday 10th - <a href="url1">desc1</a>, <a href="url2">desc2</a></li>
          <li>18:56 Tuesday 9th - <a href="url3">desc3</a>, <a href="url4">desc4</a></li>
        </ul>
      </p>
    </details>

(and thanks to [this gist](https://gist.github.com/seanh/13a93686bf4c2cb16e658b3cf96807f2)
for providing the list of all supported html and markdown formatting on github)

### Template

The first step is to turn your desired comment into a valid
[golang template](https://pkg.go.dev/text/template), consuming the
JSON objects. If you're not familiar with golang templates combined
with json, all you need to know is that `.Something` refers to the
top level object (`.`), field `Something`, you can iterate on
an array with `range` (or get the length with `len`), or get a specific
element with `index .Something 0` (or 1, or 2).

A template like this would work (watch out that golang templates preserve
newlines, and github html is sometimes weird around empty lines):

    Build reports for the latest run ({{(index .Run 0).Time}}) available here:
    {{range (index .Run 0).Links}}* [{{.Description}}]({{.Link}})
    {{end}}{{$length := len .Run}}{{if ge $length 1 }}
    <details>
      <summary>Past runs</summary>
      <p>
        <ul>
    {{range $i, $e := (slice .Run 1)}}<li>{{.Time}} -{{range .Links}} <a href="{{.Link}}">{{.Description}}</a>{{end}}</li>
    {{end}}</ul>
      </p>
    </details>{{end}}

### JSON

To fill the template above, we basically defined a JSON providing the data
that looks like this:

    {
      "Run": [
        {
          "Time": "18:56, Monday 15th",
          "Links": [
            {"Link": "http://static-analysis", "Description": "Static analysis"},
            {"Link": "http://dynamic-analysis", "Description": "Dynamic simulation"}
          ]
        },
        {
          "Time": "18:56 Wednesday 10th",
          "Links": [
            {"Link": "url1", "Description": "desc1"},
            {"Link": "url2", "Description": "desc2"}
          ]
        }, 
        {
          "Time": "18:56 Tuesday 9th",
          "Links": [
            {"Link": "url3", "Description": "desc3"},
            {"Link": "url4", "Description": "desc4"}
          ]
        }
      ]
    }

### Trying it out

Before moving forward, it's worth validating the template and the json:

1. Save the json in a file, `/tmp/message.json` in the example.
2. Save the text template in a file, `/tmp/message.template` in the example.
3. Pick a PR number of choice and a github repository. In the example, we'll use `8448` and `github.com/oktokit/test`.
3. Show what would be done, without actually posting it (--dry-run flag is important):

    staco post --json "$(< /tmp/message.json)" --template "$(< /tmp/message.template)" \
            --github-owner oktokit --github-repo test --pr 8448 --dry-run

Running the command above should show something like:

    On PR 8448 - would create new comment - content:
    ===========
    Build reports for the latest run (18:56, Monday 15th) available here:
    * [Static analysis](http://static-analysis)
    * [Dynamic simulation](http://dynamic-analysis)
    
    <details>
      <summary>Past runs</summary>
      <p>
        <ul>
    <li>18:56 Wednesday 10th - <a href="url1">desc1</a> <a href="url2">desc2</a></li>
    <li>18:56 Tuesday 9th - <a href="url3">desc3</a> <a href="url4">desc4</a></li>
    </ul>
      </p>
    </details>
    <!-- A wise goat once said: staco-unfortunate-id
    {"Template":"Build reports for the latest run ({{(index .Run 0).Time}}) available here:\n{{range (index .Run 0).Links}}* [{{.Description}}]({{.Link}})\n{{end}}\n\u003cdetails\u003e\n  \u003csummary\u003ePast runs\u003c/summary\u003e\n  \u003cp\u003e\n    \u003cul\u003e\n{{range $i, $e := (slice .Run 1)}}\u003cli\u003e{{.Time}} -{{range .Links}} \u003ca href=\"{{.Link}}\"\u003e{{.Description}}\u003c/a\u003e{{end}}\u003c/li\u003e\n{{end}}\u003c/ul\u003e\n  \u003c/p\u003e\n\u003c/details\u003e","Content":"{\"Run\":[{\"Links\":[{\"Description\":\"Static analysis\",\"Link\":\"http://static-analysis\"},{\"Description\":\"Dynamic simulation\",\"Link\":\"http://dynamic-analysis\"}],\"Time\":\"18:56, Monday 15th\"},{\"Links\":[{\"Description\":\"desc1\",\"Link\":\"url1\"},{\"Description\":\"desc2\",\"Link\":\"url2\"}],\"Time\":\"18:56 Wednesday 10th\"},{\"Links\":[{\"Description\":\"desc3\",\"Link\":\"url3\"},{\"Description\":\"desc4\",\"Link\":\"url4\"}],\"Time\":\"18:56 Tuesday 9th\"}]}"}
    -->
    ==========

Note that the posted comment contains a hidden json at the bottom. Thanks to
this hidden json, `staco` can **update** the content of the message easily,
by adding or removing runs, for example, without having to maintain state
locally.

Another important detail to note is the `staco-unfortunate-id` string:
in case you need to add multiple stable comments to a PR, all you have
to do is to supply a different `marker` with the `--marker` option.
`staco` - when run - will only touch comments with this marker.

### Updating a message

To update a message that was posted before, what you really need to do
is change the json with the new information to add (or remove) from the
rendered template.

`staco` uses the [jd](https://pkg.go.dev/github.com/josephburnett/jd@v1.6.1/lib) library
to parse and process `json patches` in different formats.

In short, this patch describes how to change the json that was already posted,
so that `staco` can use the modified json to re-render the template.

At time of writing, there are three formats supported by `staco` to describe
a patch:

1. **jd** fomat, native of the jd library.
2. **Merge** format, defined by [RFC 7386](https://datatracker.ietf.org/doc/html/rfc7386).
3. **JSON Patch** format, defined by [RFC 6902](https://datatracker.ietf.org/doc/html/rfc6902).

You can use the [online tool here](http://play.jd-tool.io/) to generate a patch
between two jsons, or use the `staco diff --input "$(< /tmp/before.json)" --output "$(< /tmp/after.json)"` to
generate the diff. The patch can normally be used and re-used easily from a script.

Watch out though that both the online tool and the `staco diff` command generate a patch
that is far from optimal: while valid, you will probably want to do some tweaking to make it reasonable.

For example, let's say we wanted to add a new run in the example above, and prepend it to the
list of runs (first run is always the most recent one).

In the "JSON patch format" this could look like:

    [
      {"op":"add","path":"/Run/0/Time","value":"13:22, Wednesday 17th"},
      {"op":"add","path":"/Run/0/Links/0/Description","value":"Static0 analysis"},
      {"op":"add","path":"/Run/0/Links/0/Link","value":"http://static0-analysis"},
      {"op":"add","path":"/Run/0/Links/1/Description","value":"Dynamic0 simulation"},
      {"op":"add","path":"/Run/0/Links/1/Link","value":"http://dynamic0-analysis"}
    ]

or, more simply:

    [
      {"op":"add","path":"/Run/0","value": { 
          "Time": "13:22, Wednesday 17th",
          "Links": [
            {"Link": "http://static0", "Description": "Static0"},
            {"Link": "http://static1", "Description": "Static1"}
          ]
        }
      }
    ]


or anything in between (the jd library will actually generate a long
list of test/remove/add operations, shifting the entire array, rather than
simply adding at offset 0).

Regardless, once you know how to change the json, all you have to do is
run staco with this information:

    staco post --diff-patch "$(< /tmp/message.patch.json)" \
            --github-owner oktokit --github-repo test --pr 8448

After, of course, saving your patch diff in the `/tmp/message.patch.json` file.

### Wrapping it all together

So, let's say your CI/CD pipeline is made of tools that run in parallel or
at different time. This means that none of the tool knows if it's run for
the first time, or it was run before.

What you can do is start from an empty json, and grow it through patches.

* Let's keep the template the same, in `/tmp/message.template`.
* In `/tmp/message.json`, let's make it an empty skeleton instead:

    {"Run": []}

* Now all updates and posts are patches, prepending one element to the
  `Run` list. Our `/tmp/message.patch.json` would be generated on the
  fly by our automation, to contain something like:

    [
      {
        "op":"add",
        "path":"/Run/0",
        "value": { 
          "Time": "13:22, Wednesday 17th",
          "Links": [
            {"Link": "http://static0", "Description": "Static0"},
            {"Link": "http://static1", "Description": "Static1"}
          ]
        }
      }
    ]

In bash, this could look like:

    #!/bin/bash

    TEMPLATE=$(cat <<END
    Build reports for the latest run ({{(index .Run 0).Time}}) available here:
    {{range (index .Run 0).Links}}* [{{.Description}}]({{.Link}})
    {{end}}{{$length := len .Run}}{{if ge $length 1 }}
    <details>
      <summary>Past runs</summary>
      <p>
        <ul>
    {{range $i, $e := (slice .Run 1)}}<li>{{.Time}} -{{range .Links}} <a href="{{.Link}}">{{.Description}}</a>{{end}}</li>
    {{end}}</ul>
      </p>
    </details>{{end}}
    END)

    PATCH=$(cat <<END
    [
      {
        "op":"add",
        "path":"/Run/0",
        "value": { 
          "Time": "$(date)",
          "Links": [
            {"Link": "$DYNAMIC_LINK", "Description": "Dynamic Analysis"},
            {"Link": "$STATIC_LINK", "Description": "Static Analysis"}
          ]
        }
      }
    ]
    END)

    staco post --template "$TEMPLATE" --json '{"Run":[]}' --diff-patch "$PATCH" \
            --github-owner oktokit --github-repo test --pr 8448


which would always update the stable comment by prepending new links.

## Authentication

At time of writing, `staco` only supports github tokens for authentication.
You can export a github token with the environment variable `GH_TOKEN` for
staco to pick it up, or use the flag `--github-token`.

Instructions to create a token are [here](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token).

## Using staco as a library

All the code behind staco can be used as a library. The go documentation is
available on godoc, at [https://pkg.go.dev/github.com/enfabrica/enkit/lib/github](https://pkg.go.dev/github.com/enfabrica/enkit/lib/github).
