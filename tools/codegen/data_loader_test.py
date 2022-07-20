"""Test the data_loader block."""

# standard libraries
import textwrap
import timeit
import unittest
import yaml

# third party libraries
from absl import app

from tools.codegen import data_loader


class TestDataLoader(unittest.TestCase):
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

    def test_parse_yaml(self):
        d = data_loader.DataLoader()
        d.ParseYaml(
            textwrap.dedent(
                """\
            foo:
              bar:
                bum: 1
              list:
                - 1
                - 2
                - 3
            foo:
              bar:
                boo: 2
              list:
                - 4
                - 5
                - 6
              new: car
            foo:
              list:
                - 7
                - 8
            arr:
              - 1
              - 2
            arr:
              - 3
              - 4
            """
            )
        )
        self.assertEqual(1, d.context["foo"]["bar"]["bum"])
        self.assertEqual(2, d.context["foo"]["bar"]["boo"])
        self.assertEqual([1, 2, 3, 4, 5, 6, 7, 8], d.context["foo"]["list"])
        self.assertEqual([1, 2, 3, 4], d.context["arr"])


def run_everything(_):
    # unittest.main uses args differently than app.run.
    unittest.main()


if __name__ == "__main__":
    app.run(run_everything)
