load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//bazel/utils:merge_kwargs.bzl", "merge_kwargs")

def _create_test_data():
    d1 = {
        "alpha": 100,
        "beta": [1, 2, 3],
        "delta": {
            "d1": 10,
            "d2": 20,
            "d3": 30,
        },
        "gamma": {
            "g1": [55, 56, 57],
            "g2": {"g2a": 1, "g2b": 2},
        },
    }
    d2 = {
        "alpha": 200,
        "beta": [4, 5, 6],
        "delta": {
            "d1": 11,
            "d4": 40,
            "d5": 50,
        },
        "gamma": {
            "g1": [58, 59],
            "g2": {"g2a": 3, "g2c": 4},
        },
    }
    d3 = {
        "alpha": 300,
        "beta": [9],
        "gamma": {"g3": 7},
    }
    merged1 = merge_kwargs(d1, d2)
    merged2 = merge_kwargs(d1, d3)
    merged3 = merge_kwargs(d1, d2)
    return (merged1, merged2, merged3)

def _merge_kwargs_test_impl(ctx):
    env = unittest.begin(ctx)
    got = "\n".join(["%r" % merged for merged in _create_test_data()])
    merged13 = '{"alpha": 200, "beta": [1, 2, 3, 4, 5, 6], "delta": {"d1": 11, "d2": 20, "d3": 30, "d4": 40, "d5": 50}, "gamma": {"g1": [55, 56, 57, 58, 59], "g2": {"g2a": 3, "g2b": 2, "g2c": 4}}}'
    merged2 = '{"alpha": 300, "beta": [1, 2, 3, 9], "delta": {"d1": 10, "d2": 20, "d3": 30}, "gamma": {"g1": [55, 56, 57, 58, 59], "g2": {"g2a": 3, "g2b": 2, "g2c": 4}, "g3": 7}}'
    want = "\n".join((merged13, merged2, merged13))
    asserts.equals(env, want, got)
    return unittest.end(env)

_merge_kwargs_test = unittest.make(_merge_kwargs_test_impl)

def merge_kwargs_test_suite(name):
    _merge_kwargs_test(name = name + "-merge_kwargs_test")
