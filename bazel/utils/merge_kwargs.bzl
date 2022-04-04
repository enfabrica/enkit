# TODO(jonathan): try to simplify this.
def merge_kwargs(d1, d2, limit = 5):
    """Combine kwargs in a useful way.

    merge_kwargs combines dictionaries by inserting keys from d2 into d1.  If
    the same key exists in both dictionaries:

    *  if the value is a scalar, d2[key] overrides d1[key].
    *  if the value is a list, the contents of d2[key] not already in d1[key]
       are appended to d1[key].
    *  if the value is a dict, the sub-dictionaries are merged similarly
       (scalars are overriden, lists are appended).

    By default, this function limits recursion to 5 levels.  The "limit"
    argument can be specified if deeper recursion is needed.
    """
    merged = {}
    to_expand = [(merged, d1, k) for k in d1] + [(merged, d2, k) for k in d2]
    for _ in range(limit):
        expand_next = []
        for m, d, k in to_expand:
            if k not in m:
                if type(d[k]) == "list":
                    m[k] = list(d[k])
                    continue

                if type(d[k]) == "dict":
                    m[k] = dict(d[k])
                    continue

                # type must be scalar:
                m[k] = d[k]
                continue

            if type(m[k]) == "dict":
                expand_next.extend([(m[k], d[k], k2) for k2 in d[k]])
                continue

            if type(m[k]) == "list":
                # uniquify as we combine lists:
                for item in d[k]:
                    if item not in m[k]:
                        m[k].append(item)
                continue

            # type must be scalar:
            m[k] = d[k]

        to_expand = expand_next
        if not to_expand:
            break

    # If <limit> layers of recursion were not enough, explicitly fail.
    if to_expand:
        fail("merge_kwargs: exceeded maximum recursion limit.")
    return merged

def add_tag(k, t):
    """Returns a kwargs dict that ensures tag `t` is present in kwargs["tags"]."""
    return merge_kwargs(k, {"tags": [t]})
