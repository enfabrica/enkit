def dedupe(iterable):
    return depset(iterable).to_list()
