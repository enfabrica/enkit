[![Go Reference](https://pkg.go.dev/badge/github.com/enfabrica/enkit/lib/github.svg)](https://pkg.go.dev/github.com/enfabrica/enkit/lib/github)

# Overview

This library provides methods and objects to easily perform
common operations on github.

Specifically, it provides a simplified github client that
always enforces timeouts, implements retries, and handles
the pagination.

It also provides a library to handle and create "stable
comments": comments appended to PRs used as mini-dashboards
that are updated as your CI/CD or automation progresses
through its work.
