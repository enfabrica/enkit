def dedupe(iterable):
    """Deduplicate a list of strs"""
    return depset(iterable).to_list()
