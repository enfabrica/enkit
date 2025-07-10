"""Unit tests for muk helper functions"""

# standard libraries
import io

# third party libraries
from absl.testing import absltest
from python.runfiles import runfiles

# enfabrica libraries
from bazel.utils.container.muk import muk


class TestMuk(absltest.TestCase):
    """Unit tests"""

    def setUp(self):
        self.runfiles = runfiles.Create()

    def test_generate_dockerfile_ubuntu(self):
        build_def = muk.parse_image_build_def(self.runfiles.Rlocation("enkit/bazel/utils/container/muk/base_dev.json"))
        with open(self.runfiles.Rlocation("enkit/bazel/utils/container/muk/testdata/base_dev.Dockerfile"), "r", encoding="utf-8") as f:
            want_dockerfile = f.read()

        got_dockerfile = io.StringIO()
        muk.generate_dockerfile(build_def, got_dockerfile)

        self.assertEqual(
            want_dockerfile,
            got_dockerfile.getvalue(),
            f"Got dockerfile:\n\n{got_dockerfile.getvalue()}",
        )


if __name__ == "__main__":
    absltest.main()
