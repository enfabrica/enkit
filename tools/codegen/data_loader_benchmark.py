"""Benchmark the data_loader block.

To run:

    bazel run -c opt :data_loader_benchmark

Sample results:

    len(big_text)=35076
    yaml version = 6.0
    yaml.Loader: 15.288706555962563
    yaml.SafeLoader: 15.32324112392962
    yaml.CSafeLoader: 1.1824901800137013
    json.loads-pretty: 0.023934083059430122
    json.loads-packed: 0.01535677001811564
    data_loader: 1.380144305061549

"""

# standard libraries
import json
import textwrap
import timeit
import unittest
import yaml

# third party libraries
from absl import app

from tools.codegen import data_loader


class BenchDataLoader(unittest.TestCase):
    """A collection of regression tests for data_loader."""

    def test_time_loader(self):
        big_yaml = []
        n = 7  # 15
        for i0 in range(n):
            big_yaml.append(f"foo{i0}:")
            for i1 in range(n):
                big_yaml.append(f"  bar{i1}:")
                for i2 in range(n):
                    big_yaml.append(f"    fum{i2}:")
                    for i3 in range(n):
                        big_yaml.append(f"      - foe{i3}")
        big_text = "\n".join(big_yaml)
        x = yaml.load(big_text, Loader=yaml.CSafeLoader)
        packed_json = json.dumps(x)
        big_json = json.dumps(x, indent=4, separators=(",", ": "))  # "pretty"
        print(f"len(big_text)={len(big_text)}")
        print(f"len(big_json)={len(big_json)}")
        print(f"len(packed_json)={len(packed_json)}")
        print(f"yaml version = {yaml.__version__}")
        bm={}

        def benchmark(name, fn):
            bm[name] = timeit.timeit(fn, number=50)
            print(f"{name}: {bm[name]}")
        benchmark("yaml.Loader", lambda: yaml.load(big_text, Loader=yaml.Loader))
        benchmark("yaml.SafeLoader", lambda: yaml.load(big_text, Loader=yaml.SafeLoader))
        benchmark("yaml.CSafeLoader", lambda: yaml.load(big_text, Loader=yaml.CSafeLoader))
        benchmark("json.loads-pretty", lambda: json.loads(big_json))
        benchmark("json.loads-packed", lambda: json.loads(packed_json))
        d = data_loader.DataLoader()
        benchmark("data_loader", lambda: d.ParseYaml(big_text))



def run_everything(_):
    # unittest.main uses args differently than app.run.
    unittest.main()


if __name__ == "__main__":
    app.run(run_everything)
