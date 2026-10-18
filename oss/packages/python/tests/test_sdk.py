import os
import sys
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))
from msh import exec, ExecResponse


class TestMshPythonSDK(unittest.TestCase):
    def test_exec_echo(self):
        resp = exec("echo hello sdk")
        self.assertIsInstance(resp, ExecResponse)
        self.assertEqual(resp.exit_code, 0)
        self.assertEqual(resp.status, "success")
        self.assertIn("hello sdk", resp.stdout)
        self.assertFalse(resp.truncated)
        self.assertTrue(resp.ok)

    def test_exec_exit_code(self):
        resp = exec("exit 7")
        self.assertEqual(resp.exit_code, 7)
        self.assertFalse(resp.ok)


if __name__ == "__main__":
    unittest.main()
