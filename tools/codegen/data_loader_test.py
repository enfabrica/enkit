"""Test the data_loader block."""

# standard libraries
import textwrap
import unittest
import yaml

# third party libraries
from absl import app

from tools.codegen import data_loader


class TestDataLoader(unittest.TestCase):
    """A collection of regression tests for data_loader."""

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
