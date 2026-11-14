import base64
import hashlib
import importlib.util
import io
from pathlib import Path
import sqlite3
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("reset_password", Path(__file__).with_name("reset-password.py"))
reset = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reset)


class FakeService:
    name = "test.service"

    def __init__(self, running=True, fail_stop=False, fail_start=False):
        self.running = running
        self.fail_stop = fail_stop
        self.fail_start = fail_start
        self.calls = []

    def is_running(self):
        return self.running

    def stop(self):
        self.calls.append("stop")
        if self.fail_stop:
            raise OSError("stop failed")

    def start(self):
        self.calls.append("start")
        if self.fail_start:
            raise OSError("start failed")


class ResetPasswordTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cnc-reset-unit-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.data = self.root / "data"
        self.data.mkdir()
        (self.data / "nc").mkdir()
        (self.data / "nc.key").write_bytes(b"k" * 32)
        (self.data / "nc" / "fixture.nc").write_bytes(b"G01 X1\r\nM30\r\n")
        self.old_hash = reset.hash_password("Original2026!")
        with sqlite3.connect(self.data / "cnc.db") as db:
            migrations = Path(__file__).resolve().parents[1] / "backend/internal/store/migrations"
            for migration in sorted(migrations.glob("*.sql")):
                db.executescript(migration.read_text())
            for uid, name in ((1, "heozmy"), (2, "other")):
                db.execute("INSERT INTO app_user VALUES (?,?,?,?,?,?,?,?)", (uid, name, "Display " + name, self.old_hash, "admin", 1, "2026-01-01", "2026-01-01"))
                db.execute("INSERT INTO user_session VALUES (?,?,?,?,?)", (uid, "token-" + name, uid, "2099-01-01", "2026-01-01"))
            db.execute("INSERT INTO drawing (id,drawing_no,created_at,updated_at) VALUES (1,'UNCHANGED','2026-01-01','2026-01-01')")
        self.account = reset.accounts(self.data)[0]
        self.output = []

    def rows(self, table, data=None):
        with sqlite3.connect((data or self.data) / "cnc.db") as db:
            return db.execute("SELECT * FROM " + table + " ORDER BY rowid").fetchall()

    def run_reset(self, service=None):
        return reset.reset_account(self.data, self.account, "NewPassword2026!", service, self.output.append)

    def test_success_backup_preservation_and_session_isolation(self):
        before = {table: self.rows(table) for table in ("app_user", "user_session", "drawing")}
        service = FakeService()
        result = self.run_reset(service)
        backup_data = Path(result["backup"]) / "data"
        for table, rows in before.items():
            self.assertEqual(self.rows(table, backup_data), rows)
        self.assertEqual(self.rows("drawing"), before["drawing"])
        self.assertEqual(self.rows("app_user")[1], before["app_user"][1])
        changed = self.rows("app_user")[0]
        self.assertEqual(changed[:3], before["app_user"][0][:3])
        self.assertEqual(changed[4:7], before["app_user"][0][4:7])
        scheme, rounds, salt, key = changed[3].split("$")
        decode = lambda value: base64.b64decode(value + "=" * (-len(value) % 4))
        self.assertEqual(scheme, "pbkdf2-sha256")
        self.assertEqual(hashlib.pbkdf2_hmac("sha256", b"NewPassword2026!", decode(salt), int(rounds), 32), decode(key))
        self.assertEqual(self.rows("user_session"), [before["user_session"][1]])
        for name in ("nc.key", "nc/fixture.nc"):
            self.assertEqual((self.data / name).read_bytes(), (backup_data / name).read_bytes())
        self.assertEqual(Path(result["backup"]).stat().st_mode & 0o777, 0o700)
        self.assertEqual(service.calls, ["stop", "start"])
        self.assertEqual(result["revokedSessions"], 1)
        self.assertEqual(self.rows("audit_log")[0][1], "reset_password")
        self.assertNotIn("NewPassword2026!", "\n".join(self.output) + str(self.rows("audit_log")))

    def test_transaction_failure_rolls_back_password_and_restores_service(self):
        with sqlite3.connect(self.data / "cnc.db") as db:
            db.execute("CREATE TRIGGER deny_delete BEFORE DELETE ON user_session BEGIN SELECT RAISE(ABORT,'fixture failure'); END")
        service = FakeService()
        with self.assertRaises(sqlite3.IntegrityError):
            self.run_reset(service)
        self.assertEqual(self.rows("app_user")[0][3], self.old_hash)
        self.assertEqual(len(self.rows("user_session")), 2)
        self.assertEqual(self.rows("audit_log"), [])
        self.assertEqual(service.calls, ["stop", "start"])

    def test_backup_failure_never_changes_password_and_restores_service(self):
        service = FakeService()
        with patch.object(reset.shutil, "copytree", side_effect=OSError("disk full")):
            with self.assertRaises(OSError):
                self.run_reset(service)
        self.assertEqual(self.rows("app_user")[0][3], self.old_hash)
        self.assertEqual(service.calls, ["stop", "start"])

    def test_interrupt_during_backup_restores_service(self):
        service = FakeService()
        with patch.object(reset.shutil, "copytree", side_effect=KeyboardInterrupt):
            with self.assertRaises(KeyboardInterrupt):
                self.run_reset(service)
        self.assertEqual(self.rows("app_user")[0][3], self.old_hash)
        self.assertEqual(service.calls, ["stop", "start"])

    def test_stop_failure_attempts_recovery_without_reset(self):
        service = FakeService(fail_stop=True)
        with self.assertRaises(OSError):
            self.run_reset(service)
        self.assertEqual(self.rows("app_user")[0][3], self.old_hash)
        self.assertEqual(service.calls, ["stop", "start"])

    def test_start_failure_reports_password_already_changed(self):
        service = FakeService(fail_start=True)
        with self.assertRaisesRegex(reset.ResetError, "密码已重置，但服务恢复失败"):
            self.run_reset(service)
        self.assertNotEqual(self.rows("app_user")[0][3], self.old_hash)

    def test_offline_directory_does_not_start_service(self):
        service = FakeService(running=False)
        result = self.run_reset(service)
        self.assertEqual(service.calls, [])
        self.assertFalse(result["serviceRestored"])

    def test_stale_disabled_account_rejected(self):
        with sqlite3.connect(self.data / "cnc.db") as db:
            db.execute("UPDATE app_user SET enabled=0 WHERE id=1")
        service = FakeService()
        with self.assertRaises(reset.ResetError):
            self.run_reset(service)
        self.assertEqual(self.rows("app_user")[0][3], self.old_hash)
        self.assertEqual(service.calls, ["stop", "start"])

    def test_lock_blocks_overlapping_resets(self):
        with reset.maintenance_lock(self.root / "backups"):
            with self.assertRaisesRegex(reset.ResetError, "另一个"):
                self.run_reset()
        self.assertEqual(self.rows("app_user")[0][3], self.old_hash)

    def test_selection_single_multiple_cancel_and_unknown(self):
        users = reset.accounts(self.data)
        self.assertEqual(reset.select_account(users[:1], output=self.output.append), users[0])
        choices = iter(["x", "9", "2"])
        self.assertEqual(reset.select_account(users, ask=lambda _: next(choices), output=self.output.append), users[1])
        self.assertIsNone(reset.select_account(users, ask=lambda _: "0", output=self.output.append))
        self.assertEqual(reset.select_account(users, "HEOZMY"), users[0])
        with self.assertRaises(reset.ResetError):
            reset.select_account(users, "missing")

    def test_secret_input_validation_retries_and_hash_bounds(self):
        answers = iter(["short", "NewPassword2026!", "mismatch2026", "ValidPassword2026!", "ValidPassword2026!"])
        self.assertEqual(reset.read_password(lambda _: next(answers), self.output.append), "ValidPassword2026!")
        for password in ("短" * 7, "长" * 129):
            with self.assertRaises(reset.ResetError):
                reset.hash_password(password)
        self.assertTrue(reset.hash_password("密" * 128).startswith("pbkdf2-sha256$"))

    def test_missing_database_not_created_and_non_terminal_refused(self):
        with self.assertRaises(sqlite3.OperationalError):
            reset.accounts(self.root / "missing")
        self.assertFalse((self.root / "missing").exists())
        with patch.object(reset.sys.stdin, "isatty", return_value=False), patch.object(reset.sys, "stderr", io.StringIO()):
            self.assertEqual(reset.main(["--data-dir", str(self.data), "--no-service"]), 1)
        self.assertEqual(self.rows("app_user")[0][3], self.old_hash)

    def test_systemd_directory_mismatch_refused_before_stop(self):
        manager = reset.SystemdService("test.service", self.data)
        with patch.object(reset.os, "geteuid", return_value=0), patch.object(manager, "command", side_effect=["loaded", "active", "CNC_DATA_DIR=/other/data"]) as command:
            with self.assertRaisesRegex(reset.ResetError, "数据目录"):
                manager.is_running()
        self.assertFalse(any(call.args == ("stop",) for call in command.call_args_list))


if __name__ == "__main__":
    unittest.main()
