import unittest
import os.path

from bazel.astore import astore_upload

class TestAstoreHandler(unittest.TestCase):
    def test_empty_uidfile(self):
        self.maxDiff = None
        h = astore_upload.AstoreHandler()
        h.update_uidfile('foobar', 'some-uid', 'some-sha')
        expected = """# Do not manually edit this file.

SHA_FOOBAR = "some-sha"
UID_FOOBAR = "some-uid"

MAP = {
  "foobar": [UID_FOOBAR, SHA_FOOBAR],
}
"""
        self.assertEqual(expected, h.text)
        h.update_uidfile('anothertool', 'another-uid', 'another-sha')
        expected = """# Do not manually edit this file.

SHA_ANOTHERTOOL = "another-sha"
SHA_FOOBAR = "some-sha"
UID_ANOTHERTOOL = "another-uid"
UID_FOOBAR = "some-uid"

MAP = {
  "anothertool": [UID_ANOTHERTOOL, SHA_ANOTHERTOOL],
  "foobar": [UID_FOOBAR, SHA_FOOBAR],
}
"""
        self.assertEqual(expected, h.text)

    def test_update_existing_uidfile(self):
        self.maxDiff = None
        h = astore_upload.AstoreHandler()
        h.text = """# foo bar

py_binary(
   name = "blahblahblah
)

FOO = "BAR"

SHA_FOOBAR = "oldfoobarsha"
UID_FOOBAR = "oldfoobaruid"
SHA_SOMETHING = "oldsha"
UID_SOMETHING = "olduid"
SHA_FEEBAR = "oldfoobarsha"
UID_FEEBAR = "oldfoobaruid"

whatever

# blah
"""
        h.update_uidfile('something', 'another-uid', 'another-sha')
        expected="""# foo bar

py_binary(
   name = "blahblahblah
)

FOO = "BAR"

SHA_FEEBAR = "oldfoobarsha"
SHA_FOOBAR = "oldfoobarsha"
SHA_SOMETHING = "another-sha"
UID_FEEBAR = "oldfoobaruid"
UID_FOOBAR = "oldfoobaruid"
UID_SOMETHING = "another-uid"

whatever

# blah
"""
        self.assertEqual(expected, h.text)

    def test_update_existing_uidfile_with_map(self):
        self.maxDiff = None
        h = astore_upload.AstoreHandler()
        h.text = """# foo bar

py_binary(
   name = "blahblahblah
)

MAP = {
  "abc": [ABC_UID, ABC_SHA],
  "def": [DEF_UID, DEF_SHA]
}

FOO = "BAR"

SHA_FOOBAR = "oldfoobarsha"
UID_FOOBAR = "oldfoobaruid"
SHA_SOMETHING = "oldsha"
UID_SOMETHING = "olduid"
SHA_FEEBAR = "oldfoobarsha"
UID_FEEBAR = "oldfoobaruid"

whatever

# blah
"""
        h.update_uidfile('something', 'another-uid', 'another-sha')
        expected="""# foo bar

py_binary(
   name = "blahblahblah
)

MAP = {
  "abc": [ABC_UID, ABC_SHA],
  "def": [DEF_UID, DEF_SHA],
  "something": [UID_SOMETHING, SHA_SOMETHING],
}

FOO = "BAR"

SHA_FEEBAR = "oldfoobarsha"
SHA_FOOBAR = "oldfoobarsha"
SHA_SOMETHING = "another-sha"
UID_FEEBAR = "oldfoobaruid"
UID_FOOBAR = "oldfoobaruid"
UID_SOMETHING = "another-uid"

whatever

# blah
"""
        self.assertEqual(expected, h.text)


if __name__ == '__main__':
    unittest.main()

