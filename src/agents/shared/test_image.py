"""The Python image rules match the shared vectors the Go check reads (testdata/agent-images)."""
import base64
import json
import os
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import metadata  # noqa: E402

VECTORS = HERE.parent.parent / 'testdata' / 'agent-images'


class Vectors(unittest.TestCase):
    def test_cases(self):
        for name, want in json.loads((VECTORS / 'cases.json').read_text()).items():
            data = (VECTORS / name).read_bytes()
            with self.subTest(name=name):
                if 'refused' in want:
                    with self.assertRaises(metadata.ImageError) as e:
                        metadata.image_check(data)
                    self.assertIn(want['refused'], str(e.exception))
                else:
                    _, sha, kind = metadata.image_check(data)
                    self.assertEqual((kind, metadata._image_size(data)[1:]), (want['type'], (want['size'], want['size'])))
                    self.assertEqual(len(sha), 64)

    def test_byte_cap(self):
        data = (VECTORS / 'a.png').read_bytes()
        at = data + b'\0' * (metadata.IMAGE_MAX_BYTES - len(data))
        metadata.image_check(at)
        with self.assertRaises(metadata.ImageError) as e:
            metadata.image_check(at + b'\0')
        self.assertIn('over 256 KiB', str(e.exception))


class Package(unittest.TestCase):
    def setUp(self):
        self.dir = Path(tempfile.mkdtemp())
        self.outside = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir)
        self.addCleanup(shutil.rmtree, self.outside)
        shutil.copy(VECTORS / 'a.png', self.outside / 'a.png')

    def manifest(self, image):
        (self.dir / 'agent.json').write_text(json.dumps({'description': 'Pictured agent.', 'image': image}))

    def test_help_and_ask(self):
        self.manifest('art/agent.webp')
        (self.dir / 'art').mkdir()
        shutil.copy(VECTORS / 'lossless.webp', self.dir / 'art' / 'agent.webp')
        data = (VECTORS / 'lossless.webp').read_bytes()
        value = json.loads(metadata.answer(self.dir, 'help-json', roles=[]))
        self.assertEqual(value['image'], 'art/agent.webp')
        self.assertEqual(value['image_sha256'], metadata.image_check(data)[1])
        self.assertEqual(value['asks'], ['help', 'help-config', 'help-secret', 'help-json', 'image'])
        self.assertEqual(base64.b64decode(metadata.dispatch(self.dir, 'image')), data)

    def test_no_image(self):
        (self.dir / 'agent.json').write_text(json.dumps({'description': 'Plain agent.'}))
        self.assertNotIn('image', metadata.answer(self.dir, 'help-json', roles=[]))
        self.assertEqual(metadata.dispatch(self.dir, 'image'), 'unsupported: image')

    def test_refusals(self):
        os.symlink(self.outside / 'a.png', self.dir / 'link.png')
        os.symlink(self.outside, self.dir / 'linked')
        (self.dir / 'dir').mkdir()
        for image, want in [(str(self.outside / 'a.png'), 'absolute path'), ('../a.png', '. or .. parts'),
                            ('link.png', 'symlink'), ('linked/a.png', 'symlink'), ('dir', 'not a regular file')]:
            with self.subTest(image=image):
                self.manifest(image)
                with self.assertRaises(metadata.ImageError) as e:
                    metadata.dispatch(self.dir, 'image')
                self.assertIn(want, str(e.exception))


if __name__ == '__main__':
    unittest.main()
