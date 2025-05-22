"""Unit tests for astore_upload_files.py"""

# standard libraries
import hashlib
import json
import os
import sys
import tempfile
import textwrap
from unittest import mock

# third party libraries
from absl import flags
from absl.testing import absltest, flagsaver

# enfabrica libraries
from bazel.astore import astore_upload_files


class AstoreUploadFilesTest(absltest.TestCase):
    """Test astore_upload_files.py."""

    @classmethod
    def setUpClass(cls):
        """Set up class-level test environment."""
        pass

        # TODO: can we mark flags as required after they are already parsed?
        # if flags.FLAGS.is_parsed():
        #     flags.mark_flags_as_required(["astore_base_path", "upload_file"])
        # else:
        # flags come parsed from absltest
        #     raise Exception("Flags not parsed")

    def setUp(self):
        """Set up test environment."""

        # Create temporary files for testing
        # pylint: disable=consider-using-with
        self.temp_dir = tempfile.TemporaryDirectory()

        # Create test file
        self.test_file = os.path.join(self.temp_dir.name, "test_file.txt")
        with open(self.test_file, "w", encoding="utf-8") as f:
            f.write("Test file content")

        # Create test target
        self.test_target = os.path.join(self.temp_dir.name, "test_target.txt")
        with open(self.test_target, "w", encoding="utf-8") as f:
            f.write("Test target content")

        # Create second test target
        self.test_second_target = os.path.join(self.temp_dir.name, "test_second_target.txt")
        with open(self.test_second_target, "w", encoding="utf-8") as f:
            f.write("Test second target content")

        # Create test UID file
        self.test_uidfile = os.path.join(self.temp_dir.name, "test_uidfile.bzl")
        with open(self.test_uidfile, "w", encoding="utf-8") as f:
            f.write(
                textwrap.dedent(
                    """
            UID_TESTTARGETTXT = "old_uid"
            SHA_TESTTARGETTXT = "old_sha"
            """
                )
            )

        print("Test environment setup complete")

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_sha256sum(self):
        """Test SHA256 hash calculation."""
        # Calculate expected hash
        h = hashlib.sha256()
        h.update(b"Test file content")
        expected_hash = h.hexdigest()

        # Test the function
        result = astore_upload_files.sha256sum(self.test_file)
        self.assertEqual(result, expected_hash)

    def test_update_starlark_version_file(self):
        """Test updating UID and SHA in build file."""
        test_uid = "test_new_uid_123"
        test_sha = "test_new_sha_456"

        # Call the function
        astore_upload_files.update_starlark_version_file(self.test_uidfile, self.test_target, test_uid, test_sha)

        # Verify the file was updated correctly
        with open(self.test_uidfile, "r", encoding="utf-8") as f:
            content = f.read()

        self.assertIn(f'UID_TESTTARGETTXT = "{test_uid}"', content)
        self.assertIn(f'SHA_TESTTARGETTXT = "{test_sha}"', content)

    @mock.patch("subprocess.run")
    @mock.patch("tempfile.NamedTemporaryFile")
    @mock.patch("os.unlink")
    def test_main_success(self, mock_unlink, mock_temp_file, mock_subprocess_run):
        """Test successful execution of main function."""

        # Keep temp.json file
        mock_unlink = mock.MagicMock()
        mock_unlink.returncode = 0

        # Mock the temporary file
        mock_temp = mock.MagicMock()
        mock_temp.name = os.path.join(self.temp_dir.name, "temp.json")
        mock_temp_file.return_value.__enter__.return_value = mock_temp

        # Create the temp JSON file with test content
        with open(mock_temp.name, "w", encoding="utf-8") as f:
            f.write(
                textwrap.dedent(
                    """
                    {
                    "Artifacts": [
                        {
                        "Uid": "test_uid_123"
                        }
                    ]
                    }"""
                )
            )

        # Mock the subprocess run result
        mock_result = mock.MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = "Uid    1 2 3 4                               test_uid_123"
        mock_subprocess_run.return_value = mock_result

        # Set required flags explicitly
        test_flags = {
            "astore_base_path": self.test_file,
            "upload_file": [self.test_target],
            "uidfile": self.test_uidfile,
            "tag": ["test_tag"],
            "output_format": "table",
        }

        with flagsaver.flagsaver(**test_flags):
            # Call the main function
            astore_upload_files.main(["astore_upload_files.py"])

        # Verify subprocess was called with correct arguments
        mock_subprocess_run.assert_called_once()
        cmd_args = mock_subprocess_run.call_args[0][0]

        self.assertIn("--tag=test_tag", cmd_args)
        self.assertIn("--disable-git", cmd_args)
        self.assertIn("--file", cmd_args)
        self.assertIn(self.test_file, cmd_args)
        self.assertIn(self.test_target, cmd_args)

        self.assertTrue(os.path.exists(mock_temp.name))
        with open(mock_temp.name, "r", encoding="utf-8") as f:
            content = f.read()

        uid = json.loads(content)["Artifacts"][0]["Uid"]
        self.assertEqual(uid, "test_uid_123")
        self.assertIn(uid, mock_result.stdout)

    @mock.patch("subprocess.run")
    @mock.patch("tempfile.NamedTemporaryFile")
    @mock.patch("os.unlink")
    def test_main_success_flextape(self, mock_unlink, mock_temp_file, mock_subprocess_run):
        """Test successful execution of main function."""

        # Keep temp.json file
        mock_unlink = mock.MagicMock()
        mock_unlink.returncode = 0

        # Mock the temporary file
        mock_temp = mock.MagicMock()
        mock_temp.name = os.path.join(self.temp_dir.name, "temp.json")
        mock_temp_file.return_value.__enter__.return_value = mock_temp

        # Create the temp TOML file with test content
        with open(mock_temp.name, "w", encoding="utf-8") as f:
            f.write(
                textwrap.dedent(
                    """
                {
                "Artifacts": [
                    {
                    "Architecture": "all",
                    "Created": "1747686104596565327",
                    "Creator": "roman@enfabrica.net",
                    "MD5": [
                        129,
                        47
                    ],
                    "Note": "",
                    "Sid": "tf/mj/7rjhuz86szyg7xkd2fepqxhf576c",
                    "Size": 18067,
                    "Tag": [
                        "latest"
                    ],
                    "Uid": "kz6uksgwtwpfjofvj22jyptfsc4g3nm6"
                    }
                ]
                }"""
                )
            )

        # Mock the subprocess run result
        mock_result = mock.MagicMock()
        mock_result.returncode = 0
        # FIXME: is this correct table output?
        # pylint: disable=line-too-long
        mock_result.stdout = """| Created                 Creator                        Arch           MD5                              UID                              Size    TAGs
| 2025-05-19 18:16:43.026 roman@enfabrica.net            amd64-linux    4ca03f0e6a70a2e3fc925955ab301e5d kz6uksgwtwpfjofvj22jyptfsc4g3nm6 12 MB   [latest]
"""
        mock_subprocess_run.return_value = mock_result

        # Set required flags explicitly
        test_flags = {
            "astore_base_path": self.test_file,
            "upload_file": [self.test_target],
            "uidfile": self.test_uidfile,
            "tag": ["mytest_tag"],
            "output_format": "table",
        }

        # Set up flags for this test
        with flagsaver.flagsaver(**test_flags):
            # Call the main function
            astore_upload_files.main(["astore_upload_files.py"])

        # Verify subprocess was called with correct arguments
        mock_subprocess_run.assert_called_once()
        cmd_args = mock_subprocess_run.call_args[0][0]

        self.assertIn("--tag=mytest_tag", cmd_args)
        self.assertIn("--disable-git", cmd_args)
        self.assertIn("--file", cmd_args)
        self.assertIn(self.test_file, cmd_args)
        self.assertIn(self.test_target, cmd_args)
        self.assertNotIn("console-format", cmd_args)
        with self.assertRaises(json.decoder.JSONDecodeError):
            _ = json.loads(mock_result.stdout)

        with open(mock_temp.name, "r", encoding="utf-8") as f:
            content = f.read()

        uid = json.loads(content)["Artifacts"][0]["Uid"]
        self.assertEqual(uid, "kz6uksgwtwpfjofvj22jyptfsc4g3nm6")
        self.assertIn(uid, mock_result.stdout)

    @mock.patch("subprocess.run")
    @mock.patch("tempfile.NamedTemporaryFile")
    def test_main_success_flextape_json(self, mock_temp_file, mock_subprocess_run):
        """Test successful execution of main function."""
        # Mock the temporary file
        mock_temp = mock.MagicMock()
        mock_temp.name = os.path.join(self.temp_dir.name, "temp.json")
        mock_temp_file.return_value.__enter__.return_value = mock_temp

        # Create the temp TOML file with test content
        with open(mock_temp.name, "w", encoding="utf-8") as f:
            f.write(
                textwrap.dedent(
                    """
                {
                "Artifacts": [
                    {
                    "Architecture": "all",
                    "Created": "1747686104596565327",
                    "Creator": "roman@enfabrica.net",
                    "Note": "",
                    "Sid": "tf/mj/7rjhuz86szyg7xkd2fepqxhf576c",
                    "Size": 18067,
                    "Tag": [
                        "latest"
                    ],
                    "Uid": "ym2wvcei7j2ihhvh7sauzgipxcvdtsvr"
                    }
                ]
                }"""
                )
            )

        # Mock the subprocess run result
        mock_result = mock.MagicMock()
        mock_result.returncode = 0

        # pylint: disable=line-too-long
        data = """{"Artifacts":[{"sid":"tf/mj/7rjhuz86szyg7xkd2fepqxhf576c","uid":"kz6uksgwtwpfjofvj22jyptfsc4g3nm6","tag":["latest"],"MD5":"JC0fdAdtfj3lBMReC/qBLw==","size":18067,"creator":"roman@enfabrica.net","created":1747686104596565327,"architecture":"all"}],"Elements":null}"""
        data = json.loads(data)
        data["Artifacts"][0]["Target"] = self.test_target
        mock_result.stdout = json.dumps(data)

        mock_subprocess_run.return_value = mock_result

        # Set required flags explicitly
        test_flags = {
            "astore_base_path": self.test_file,
            "upload_file": [self.test_target],
            "uidfile": self.test_uidfile,
            "tag": ["test_tag"],
            "output_format": "json",
        }

        # Set up flags for this test
        with flagsaver.flagsaver(**test_flags):
            # Call the main function
            astore_upload_files.main(["astore_upload_files.py"])

        # Verify subprocess was called with correct arguments
        mock_subprocess_run.assert_called_once()
        cmd_args = mock_subprocess_run.call_args[0][0]

        self.assertIn("--tag=test_tag", cmd_args)
        self.assertIn("--disable-git", cmd_args)
        self.assertIn("--file", cmd_args)
        self.assertIn(self.test_file, cmd_args)
        self.assertIn(self.test_target, cmd_args)
        self.assertIn("--console-format", cmd_args)
        self.assertIn("json", cmd_args)
        self.assertEqual(cmd_args.index("--console-format"), cmd_args.index("json") - 1)
        data = json.loads(mock_result.stdout)
        artifacts = data["Artifacts"]
        self.assertEqual(artifacts[0]["MD5"], "JC0fdAdtfj3lBMReC/qBLw==")
        self.assertEqual(artifacts[0]["uid"], "kz6uksgwtwpfjofvj22jyptfsc4g3nm6")

        # must be list of dicts
        self.assertEqual(1, len(artifacts))
        self.assertEqual(artifacts[0]["Target"], self.test_target)

    @mock.patch("subprocess.run")
    @mock.patch("tempfile.NamedTemporaryFile")
    def test_main_success_flextape_two_json(self, mock_temp_file, mock_subprocess_run):
        """Test successful execution of main function."""
        # Mock the temporary file
        mock_temp = mock.MagicMock()
        mock_temp.name = os.path.join(self.temp_dir.name, "temp.json")
        mock_temp_file.return_value.__enter__.return_value = mock_temp

        # Create the temp TOML file with test content
        with open(mock_temp.name, "w", encoding="utf-8") as f:
            f.write(
                textwrap.dedent(
                    """
                    {
                    "Artifacts": [
                        {
                        "Architecture": "all",
                        "Created": "1747686104596565327",
                        "Creator": "roman@enfabrica.net",
                        "MD5": [
                            129,
                            47
                        ],
                        "Note": "",
                        "Sid": "tf/mj/7rjhuz86szyg7xkd2fepqxhf576c",
                        "Size": 18067,
                        "Tag": [
                            "latest"
                        ],
                        "Uid": "ym2wvcei7j2ihhvh7sauzgipxcvdtsvr"
                        }
                    ]
                    }"""
                )
            )

        # Mock the subprocess run result
        mock_result = mock.MagicMock()
        mock_result.returncode = 0

        # pylint: disable=line-too-long
        data = """{"Artifacts":[{"sid":"tf/mj/7rjhuz86szyg7xkd2fepqxhf576c","uid":"kz6uksgwtwpfjofvj22jyptfsc4g3nm6","tag":["latest"],"MD5":"JC0fdAdtfj3lBMReC/qBLw==","size":18067,"creator":"roman@enfabrica.net","created":1747686104596565327,"architecture":"all"}],"Elements":null}"""
        data = json.loads(data)
        data["Artifacts"][0]["Target"] = self.test_target
        data["Artifacts"].append(data["Artifacts"][0].copy())
        data["Artifacts"][1]["Target"] = self.test_second_target
        mock_result.stdout = json.dumps(data)

        mock_subprocess_run.return_value = mock_result

        # Set required flags explicitly
        test_flags = {
            "astore_base_path": self.test_file,
            "upload_file": [self.test_target, self.test_second_target],
            "tag": ["test_tag"],
            "output_format": "json",
        }

        # Set up flags for this test
        with flagsaver.flagsaver(**test_flags):
            # Call the main function
            astore_upload_files.main(["astore_upload_files.py"])

        # Verify subprocess was called with correct arguments
        self.assertEqual(mock_subprocess_run.call_count, 2)
        cmd_args = mock_subprocess_run.call_args_list[0][0][0]
        cmd_args_second = mock_subprocess_run.call_args_list[1][0][0]

        self.assertIn("--tag=test_tag", cmd_args)
        self.assertIn("--disable-git", cmd_args)
        self.assertIn("--file", cmd_args)
        self.assertIn(self.test_file, cmd_args)
        self.assertIn(self.test_target, cmd_args)
        self.assertIn(self.test_second_target, cmd_args_second)
        self.assertIn("--console-format", cmd_args)
        self.assertIn("json", cmd_args)
        self.assertEqual(cmd_args.index("--console-format"), cmd_args.index("json") - 1)
        data = json.loads(mock_result.stdout)

        artifacts = data["Artifacts"]
        self.assertEqual(len(artifacts), 2)
        self.assertEqual(artifacts[0]["MD5"], "JC0fdAdtfj3lBMReC/qBLw==")
        self.assertEqual(artifacts[0]["uid"], "kz6uksgwtwpfjofvj22jyptfsc4g3nm6")

        # must be list of dicts
        self.assertEqual(artifacts[0]["Target"], self.test_target)
        self.assertEqual(artifacts[1]["Target"], self.test_second_target)

    @mock.patch("subprocess.run")
    @mock.patch("tempfile.NamedTemporaryFile")
    def test_main_success_two_targets(self, mock_temp_file, mock_subprocess_run):
        """Test successful execution of main function."""
        # Mock the temporary file
        mock_temp = mock.MagicMock()
        mock_temp.name = os.path.join(self.temp_dir.name, "temp.json")
        mock_temp_file.return_value.__enter__.return_value = mock_temp

        # Create the temp TOML file with test content
        with open(mock_temp.name, "w", encoding="utf-8") as f:
            f.write(
                textwrap.dedent(
                    """
                    {
                    "Artifacts": [
                        {
                        "Uid": "test_uid_123"
                        }
                    ]
                    }"""
                )
            )

        # Mock the subprocess run result
        mock_result = mock.MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = "Uid    1 2 3 4                               test_uid_123"
        mock_subprocess_run.return_value = mock_result

        # Set required flags explicitly
        test_flags = {
            "astore_base_path": self.test_file,
            "upload_file": [self.test_target, self.test_second_target],
            "tag": ["test_tag"],
            "output_format": "table",
        }

        # Set up flags for this test
        with flagsaver.flagsaver(**test_flags):
            # Call the main function
            astore_upload_files.main(["astore_upload_files.py"])

        # Verify subprocess was called with correct arguments
        self.assertEqual(mock_subprocess_run.call_count, 2)
        cmd_args = mock_subprocess_run.call_args_list[0][0][0]
        cmd_args_second = mock_subprocess_run.call_args_list[1][0][0]

        self.assertIn("--tag=test_tag", cmd_args)
        self.assertIn("--disable-git", cmd_args)
        self.assertIn("--file", cmd_args)

        self.assertIn(self.test_file, cmd_args)
        self.assertIn(self.test_target, cmd_args)
        self.assertIn(self.test_second_target, cmd_args_second)

    @mock.patch("subprocess.run")
    @mock.patch("tempfile.NamedTemporaryFile")
    def test_main_json_output(self, mock_temp_file, mock_subprocess_run):
        """Test main function with JSON output format."""
        # Mock the temporary file
        mock_temp = mock.MagicMock()
        mock_temp.name = os.path.join(self.temp_dir.name, "temp.json")
        mock_temp_file.return_value.__enter__.return_value = mock_temp

        # Create the temp TOML file with test content
        with open(mock_temp.name, "w", encoding="utf-8") as f:
            f.write(
                textwrap.dedent(
                    """
                    {
                    "Artifacts": [
                        {
                        "Uid": "test_uid_123"
                        }
                    ]
                    }"""
                )
            )

        # Mock the subprocess run result
        mock_result = mock.MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = '{"Artifacts":[{"uid": "test_uid_123"}]}'
        mock_subprocess_run.return_value = mock_result

        # Set required flags explicitly
        test_flags = {"astore_base_path": self.test_file, "upload_file": [self.test_target], "output_format": "json"}

        # Set up flags for this test
        with flagsaver.flagsaver(**test_flags):
            # Call the main function
            astore_upload_files.main(["astore_upload_files.py"])

        # Verify subprocess was called with correct arguments
        mock_subprocess_run.assert_called_once()
        cmd_args = mock_subprocess_run.call_args[0][0]
        self.assertEqual(cmd_args[-3], "--console-format")
        self.assertEqual(cmd_args[-2], "json")

    @mock.patch("subprocess.run")
    @mock.patch("tempfile.NamedTemporaryFile")
    def test_main_upload_error(self, mock_temp_file, mock_subprocess_run):
        """Test main function when upload fails."""
        # Mock the temporary file
        mock_temp = mock.MagicMock()
        mock_temp.name = os.path.join(self.temp_dir.name, "temp.json")
        mock_temp_file.return_value.__enter__.return_value = mock_temp

        # Mock the subprocess run result with error
        mock_result = mock.MagicMock()
        mock_result.returncode = 1
        mock_result.stderr = "Upload failed"
        mock_subprocess_run.return_value = mock_result

        # Set required flags explicitly
        test_flags = {
            "astore_base_path": self.test_file,
            "upload_file": [self.test_target],
        }

        # Set up flags for this test
        with flagsaver.flagsaver(**test_flags):
            # Call the main function and expect sys.exit(1)
            with self.assertRaises(SystemExit) as cm:
                astore_upload_files.main(["astore_upload_files.py"])
            self.assertEqual(cm.exception.code, 1)

    @mock.patch("bazel.astore.astore_upload_files.log.fatal")
    @mock.patch("subprocess.run")
    @mock.patch("tempfile.NamedTemporaryFile")
    def test_main_required_options(self, mock_temp_file, mock_subprocess_run, fatal):
        """Test main function when upload fails."""
        fatal = mock.MagicMock()
        fatal.side_effect = None

        # Mock the temporary file
        mock_temp = mock.MagicMock()
        mock_temp.name = os.path.join(self.temp_dir.name, "temp.json")
        mock_temp_file.return_value.__enter__.return_value = mock_temp

        # Mock the subprocess run result with error
        mock_result = mock.MagicMock()
        mock_result.returncode = 1
        mock_result.stderr = "Upload failed"
        mock_subprocess_run.return_value = mock_result

        # Set required flags explicitly
        test_flags = {
            "astore_base_path": self.test_file,
        }

        # Set up flags for this test
        with flagsaver.flagsaver(**test_flags):
            # Call the main function and expect fail on iteration over None object
            with self.assertRaises(Exception) as ex:
                astore_upload_files.main(["astore_upload_files.py"])
            self.assertEqual("'NoneType' object is not iterable", str(ex.exception))

        # Set required flags explicitly
        test_flags = {
            "upload_file": [self.test_target],
        }

        # Set up flags for this test
        with flagsaver.flagsaver(**test_flags):
            # Call the main function and expect to fail on destination not set
            with self.assertRaises(Exception) as ex:
                astore_upload_files.main(["astore_upload_files.py"])
            self.assertEqual("'NoneType' object has no attribute 'endswith'", str(ex.exception))


if __name__ == "__main__":
    absltest.main()
