"""Benchmark the data_loader block."""

# standard libraries
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
        n = 6  # 15
        for i0 in range(n):
            big_yaml.append(f"foo{i0}:")
            for i1 in range(n):
                big_yaml.append(f"  bar{i1}:")
                for i2 in range(n):
                    big_yaml.append(f"    fum{i2}:")
                    for i3 in range(n):
                        big_yaml.append(f"      - foe{i3}")
        big_text = "\n".join(big_yaml)
        print(f"len(big_text)={len(big_text)}")
        print(f"yaml version = {yaml.__version__}")
        bm={}
        def benchmark(name, fn):
            bm[name] = timeit.timeit(fn, number=50)
            print(f"{name}: {bm[name]}")
        benchmark("yaml.Loader", lambda: yaml.load(big_text, Loader=yaml.Loader))
        benchmark("yaml.SafeLoader", lambda: yaml.load(big_text, Loader=yaml.SafeLoader))
        benchmark("yaml.CSafeLoader", lambda: yaml.load(big_text, Loader=yaml.CSafeLoader))
        d = data_loader.DataLoader()
        benchmark("data_loader", lambda: d.ParseYaml(big_text))



def run_everything(_):
    # unittest.main uses args differently than app.run.
    unittest.main()


if __name__ == "__main__":
    app.run(run_everything)
