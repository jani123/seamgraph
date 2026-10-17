import importlib.util
from pathlib import Path
import unittest
import tempfile

spec = importlib.util.spec_from_file_location('seamgraph_python', Path(__file__).with_name('analyzer.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class AnalyzerTests(unittest.TestCase):
    def test_relative_reexports_all_and_src_namespace(self):
        root = Path(__file__).resolve().parents[2]/'testdata/projects/python'
        config = {'Modules': {'world': {'Language': 'python', 'Paths': ['src/world'], 'ImportRoots': ['src'], 'Seams': {'reader': {'Entrypoints': ['src/world/api.py', 'src/world/__init__.py']}}}, 'npc': {'Language': 'python', 'Paths': ['src/npc'], 'ImportRoots': ['src']}}}
        s = module.analyze({'version': 1, 'root': str(root), 'config': config})
        self.assertFalse(s['diagnostics'])
        self.assertTrue(any(sid.endswith('Reader.read') for sid in s['seams'][0]['contractMembers']))
        self.assertTrue(any(r['kind'] == 'uses' for r in s['relations']))

    def test_property_accessors_keep_both_declared_signatures(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root/'world').mkdir()
            (root/'world/api.py').write_text("class Reader:\n    @property\n    def value(self) -> str: return 'world'\n    @value.setter\n    def value(self, new_value: str) -> None: pass\n")
            config = {'Modules': {'world': {'Language': 'python', 'Paths': ['world'], 'Seams': {'reader': {'Entrypoints': ['world/api.py']}}}}}
            snapshot = module.analyze({'version': 1, 'root': str(root), 'config': config})
            self.assertFalse(snapshot['diagnostics'])
            symbol = next(s for s in snapshot['symbols'] if s['id'].endswith('Reader.value'))
            self.assertIn('@property', symbol['displaySignature'])
            self.assertIn('@value.setter', symbol['displaySignature'])
            self.assertIn('-> str', symbol['displaySignature'])
            self.assertIn('new_value: str', symbol['displaySignature'])

    def test_bad_protocol(self):
        with self.assertRaises(ValueError):
            module.analyze({'version': 99})

if __name__ == '__main__':
    unittest.main()
