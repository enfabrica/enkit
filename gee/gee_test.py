#!/usr/bin/python3.12

import unittest
import doctest

import gee

class TestGee(unittest.TestCase):
    def test_doctests(self):
        (failure_count, test_count) = doctest.testmod(gee, verbose=False)
        self.assertGreater(test_count, 0)
        self.assertEqual(failure_count, 0)

    def test_codeowners(self):
        co = gee.Codeowners()
        co.AddRule("/*", ["@a", "@b"])  # rule 1
        co.AddRule("/.bazelrc", ["@d"])  # rule 2
        co.AddRule("/foo/bar/", ["@e"])  # rule 3
        co.AddRule("/fee/fie", ["@f"])  # rule 4
        co.AddRule("**/cache", ["@g"])  # rule 5
        co.AddRule("**/far/**/away", ["@h"])  # rule 6
        co.AddRule("BUILD", ["@i"])

        self.assertEqual(co.GetOwnersForFile("DIRECTORY"), "@a @b")
        self.assertEqual(co.GetOwnersForFile(".bazelrc"), "@d")
        self.assertEqual(co.GetOwnersForFile(".bazelrc.bak"), "@a @b")
        self.assertEqual(co.GetOwnersForFile(".bazelrc/bak"), "@d")
        self.assertEqual(co.GetOwnersForFile("foo/.bazelrc"), "@a @b")  # rule 2 doesn't match
        self.assertEqual(co.GetOwnersForFile("foo/bar"), "@a @b")  # rule 3 won't match file
        self.assertEqual(co.GetOwnersForFile("foo/bar/bum.txt"), "@e")
        self.assertEqual(co.GetOwnersForFile("fee/fie"), "@f")
        self.assertEqual(co.GetOwnersForFile("fee/fie/fum"), "@f")
        self.assertEqual(co.GetOwnersForFile("foo/bar/a/cache/bum.txt"), "@g")
        self.assertEqual(co.GetOwnersForFile("foo/bar/a/far/b/away/bum.txt"), "@h")
        self.assertEqual(co.GetOwnersForFile("BUILD"), "@i")
        self.assertEqual(co.GetOwnersForFile("a/b/c/BUILD"), "@i")
        self.assertEqual(co.GetOwnersForFile("a/b/c/BUILD/d/e/f"), "@i")
        self.assertEqual(co.GetOwnersForFile("a/b/c/BUILD2"), "@a @b")

        s = co.GetOwnersForFileSet(("a/b/c/BUILD", "fee/fie", "README.txt", "b/BUILD"))
        self.assertEqual(s, {
            "@i": ["a/b/c/BUILD", "b/BUILD"],
            "@f": ["fee/fie"],
            "@a @b": ["README.txt"],
        })



if __name__ == "__main__":
    unittest.main()
