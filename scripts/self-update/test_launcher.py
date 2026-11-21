import hashlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import launcher as app


class UpdateTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.data = Path(tmp.name)
        self.store = self.data / ".cnc-updates"
        self.store.mkdir()
        self.image_id = self.data / "test-image-id"
        self.image_id.write_text("image-A")
        for name, value in [("STORE", self.store), ("DATA", self.data), ("IMAGE_ID", self.image_id)]:
            patcher = patch.object(app, name, value)
            patcher.start()
            self.addCleanup(patcher.stop)

    def test_image_precedence(self):
        app.activate_image()
        tag = "v2.10.0"
        (self.store / tag).touch()
        app.atomic(self.store / "active.json", tag)
        for bundled, expected in [("v2.9.9", self.store/tag), (tag, app.BUNDLED), ("v3.0.0", app.BUNDLED)]:
            with patch.object(app, "version", side_effect=lambda p: bundled if p == app.BUNDLED else tag):
                self.assertEqual(app.installed()[0], expected)

    def test_replacement_image_supersedes_higher_app_update(self):
        app.activate_image()
        tag = "v3.0.0"
        (self.store / tag).touch()
        app.atomic(self.store / "active.json", tag)
        app.activate_image()
        self.assertEqual(app.read("active.json", None), tag)
        self.image_id.write_text("image-B")
        with patch.object(app, "version", return_value="v2.5.2"):
            self.assertEqual(app.installed(), (app.BUNDLED, "v2.5.2"))
        app.activate_image()
        self.assertEqual(app.read("active.json", None), "")
        self.assertEqual(app.read("status.json", {})["phase"], "idle")
        self.assertTrue((self.store / tag).exists())

    def test_bad_download_keeps_current_binary(self):
        tag = "v2.5.4"
        manifest = hashlib.sha256(b"correct").hexdigest() + "  cnc-manager-v2.5.4-linux-amd64"
        streams = [json.dumps(dict(tag_name=tag, draft=False, prerelease=False)).encode(), manifest.encode(), b"wrong"]
        with patch.object(app.urllib.request, "urlopen", side_effect=[io.BytesIO(s) for s in streams]):
            with self.assertRaisesRegex(ValueError, "SHA-256"):
                app.download(tag)
        self.assertFalse((self.store/tag).exists())
        self.assertFalse((self.store/"active.json").exists())

    def test_resume_rollback(self):
        folder = self.store / "backup-v2.5.3-test"
        backup = folder / "data"
        backup.mkdir(parents=True)
        (backup/"cnc.db").write_bytes(b"before")
        (self.data/"cnc.db").write_bytes(b"after")
        app.atomic(self.store/"rollback.json", dict(folder=folder.name, previous="v2.5.3", phase="retain"))
        app.rollback()
        self.assertEqual((self.data/"cnc.db").read_bytes(), b"before")
        self.assertEqual((folder/"failed-data"/"cnc.db").read_bytes(), b"after")
        self.assertEqual(app.read("active.json", None), "v2.5.3")
        app.rollback()

    def test_live_start_failure_restores_snapshot_and_previous_binary(self):
        target = "v2.5.4"
        previous = "v2.5.3"
        (self.data / "cnc.db").write_bytes(b"before")
        def check(proc, addr, tag):
            if not addr.startswith("127.0.0.1:"):
                (self.data / "cnc.db").write_bytes(b"failed migration")
                raise ValueError("unhealthy")
        with patch.object(app, "download", return_value=self.store / target), \
                patch.object(app, "stop"), patch.object(app, "launch") as launch, \
                patch.object(app, "ready", side_effect=check), patch.dict(app.os.environ, {"CNC_ADDR":"0.0.0.0:8080"}):
            with self.assertRaisesRegex(ValueError, "unhealthy"):
                app.upgrade(target, app.BUNDLED, previous)
        launch.assert_called_with(app.BUNDLED)
        self.assertEqual((self.data / "cnc.db").read_bytes(), b"before")
        retained, = self.store.glob("backup-*/failed-data/cnc.db")
        self.assertEqual(retained.read_bytes(), b"failed migration")
        self.assertEqual(app.read("active.json", None), previous)
