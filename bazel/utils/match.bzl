def match(data, pattern):
    """Matches a string against a simplified glob pattern

    Simplified glob patterns are glob patterns that only
    support a single '*'. For example: "*.c", or "foo*.c",
    or "foo*" are all valid simplified glob patterns.

    "foo*prefix_*.c" or "foo?bar*.c" are unsupported, the
    former because it contains multiple *, the latter because
    it contains a ?.

    The syntax for simplified glob patterns may be expanded.

    Args:
      data: string, an arbitrary string.
      pattern: string, a simplified glob pattern.

    Returns:
      True if data matches the pattern. False otherwise.
      The empty pattern matches all strings.
    """
    divided = pattern.split("*")
    if len(divided) != 1 and len(divided) != 2:
        fail("Matches use a simplified library - only one wildcard '*' is supported - '%s' is invalid - contributions to improve are welcome!" % (pattern))

    if len(divided) == 1:
        return pattern in data
    return data.startswith(divided[0]) and data.endswith(divided[1])

def to_glob(pattern):
    """Given a pattern in match format, returns a glob in shell format."""
    if "*" in pattern:
        return pattern
    return "*" + pattern + "*"

def matchall(data, patterns, default = True):
    """Returns True if the supplied string matches all the patterns.

    Args:
      data: string, an arbitrary string to match.
      patterns: an iterable of simplified glob patterns.
      default: what to return if the set of patterns is empty.
    Returns:
      If no pattern is supplied, returns the default.
      If one or more patterns are supplied, returns True if
      data matches all the supplied patterns. False otherwise.
    """
    if not patterns:
        return default

    for pattern in patterns:
        if not match(data, pattern):
            return False

    return True

def matchany(data, patterns, default = True):
    """Returns True if the supplied string matches one of the patterns.

    Args:
      data: string, an arbitrary string to match.
      patterns: an iterable of simplified glob patterns.
      default: what to return if the set of patterns is empty.
    Returns:
      If no pattern is supplied, returns the default.
      If one or more patterns are supplied, returns True if
      data matches at least one of the supplied patterns.
    """
    if not patterns:
        return default

    for pattern in patterns:
        if match(data, pattern):
            return True

    return False
