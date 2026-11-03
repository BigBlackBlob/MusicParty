"""Tests for prepare-isolated.py.

Run with: python -m unittest discover -s backend-go/deploy -p 'test_*.py'
Only the environment-provisioning helpers are covered; they do not shell out.
"""
import importlib.util
import json
import os
import pathlib
import stat
import sys
import unittest

SCRIPT = pathlib.Path(__file__).resolve().parent / "prepare-isolated.py"
spec = importlib.util.spec_from_file_location("prepare_isolated", SCRIPT)
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class ResolveSecretTest(unittest.TestCase):
    def setUp(self):
        self.root = pathlib.Path(self._tmpdir())
        self.runtime_env = self.root / prepare.RUNTIME_ENV_NAME

    def _tmpdir(self):
        import tempfile

        self._temp = tempfile.TemporaryDirectory()
        self.addCleanup(self._temp.cleanup)
        return self._temp.name

    def test_generates_a_secret_when_nothing_is_provided(self):
        environment = {prepare.ROOM_ACCESS_SECRET_KEY: "inherited-from-source-container"}
        secret, origin = prepare.resolve_room_access_secret(environment, self.runtime_env)
        self.assertEqual("generated", origin)
        self.assertGreaterEqual(len(secret), prepare.MIN_ROOM_ACCESS_SECRET_LENGTH)
        self.assertNotEqual("inherited-from-source-container", secret)

    def test_reuses_the_persisted_secret(self):
        self.runtime_env.write_text(prepare.ROOM_ACCESS_SECRET_KEY + "=persisted-secret-value\n")
        secret, origin = prepare.resolve_room_access_secret({}, self.runtime_env)
        self.assertEqual("persisted", origin)
        self.assertEqual("persisted-secret-value", secret)

    def test_prefers_the_caller_value(self):
        self.runtime_env.write_text(prepare.ROOM_ACCESS_SECRET_KEY + "=persisted-secret-value\n")
        os.environ[prepare.ROOM_ACCESS_SECRET_KEY] = "caller-provided-secret"
        self.addCleanup(os.environ.pop, prepare.ROOM_ACCESS_SECRET_KEY, None)
        secret, origin = prepare.resolve_room_access_secret({}, self.runtime_env)
        self.assertEqual("provided", origin)
        self.assertEqual("caller-provided-secret", secret)

    def test_ignores_an_empty_persisted_value(self):
        self.runtime_env.write_text(prepare.ROOM_ACCESS_SECRET_KEY + "=\n")
        secret, origin = prepare.resolve_room_access_secret({}, self.runtime_env)
        self.assertEqual("generated", origin)
        self.assertTrue(secret)


class RuntimeEnvironmentTest(unittest.TestCase):
    def setUp(self):
        import tempfile

        self._temp = tempfile.TemporaryDirectory()
        self.addCleanup(self._temp.cleanup)
        self.root = pathlib.Path(self._temp.name)

    @unittest.skipIf(os.name == "nt", "Windows chmod only toggles the read-only bit")
    def test_writes_owner_only_file(self):
        path = prepare.write_runtime_environment(
            self.root, {"APP_ENV": "production", prepare.ROOM_ACCESS_SECRET_KEY: "0123456789abcdef"}
        )
        self.assertEqual(0o600, stat.S_IMODE(os.stat(path).st_mode))
        self.assertIn("ROOM_ACCESS_TOKEN_SECRET=0123456789abcdef", path.read_text())

    def test_rejects_a_short_secret(self):
        with self.assertRaises(RuntimeError):
            prepare.write_runtime_environment(self.root, {prepare.ROOM_ACCESS_SECRET_KEY: "short"})

    def test_rejects_multiline_values(self):
        with self.assertRaises(RuntimeError):
            prepare.write_runtime_environment(self.root, {"APP_ENV": "production\nINJECTED=1"})

    @unittest.skipIf(os.name == "nt", "Windows chmod only toggles the read-only bit")
    def test_verify_accepts_a_written_file_and_reports_no_secret(self):
        path = prepare.write_runtime_environment(
            self.root, {"APP_ENV": "production", prepare.ROOM_ACCESS_SECRET_KEY: "0123456789abcdef"}
        )
        self.assertEqual(16, prepare.verify_runtime_environment(path))

    def test_owner_only_mode_accepts_0600(self):
        prepare.require_owner_only(0o100600)

    def test_owner_only_mode_rejects_0644(self):
        with self.assertRaises(RuntimeError):
            prepare.require_owner_only(0o100644)

    def test_owner_only_mode_rejects_group_and_other_bits_only(self):
        with self.assertRaises(RuntimeError):
            prepare.require_owner_only(0o100040)

    @unittest.skipIf(os.name == "nt", "Windows chmod only toggles the read-only bit")
    def test_verify_rejects_a_missing_secret(self):
        path = self.root / prepare.RUNTIME_ENV_NAME
        path.write_text("APP_ENV=production\n")
        os.chmod(path, 0o600)
        with self.assertRaises(RuntimeError):
            prepare.verify_runtime_environment(path)


if __name__ == "__main__":
    unittest.main()
