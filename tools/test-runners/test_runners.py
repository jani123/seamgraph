"""Actual runner acceptance tests, using reviewed synthetic fixture behavior."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

REPO = Path(__file__).resolve().parents[2]
CLI = REPO/'.build/seamgraph'
ENV = {**os.environ, 'PATH': str(Path(sys.executable).parent)+os.pathsep+os.environ['PATH']}

class RunnerAcceptance(unittest.TestCase):
    def exercise(self, language, runner, test_file, test_source, test_id, implementation):
        with tempfile.TemporaryDirectory(prefix='seamgraph-real-runner-') as temporary:
            root = Path(temporary)
            shutil.copytree(REPO/'testdata/projects'/language, root, dirs_exist_ok=True)
            (root/test_file).write_text(test_source)
            if language == 'typescript':
                (root/'node_modules').symlink_to(REPO/'tools/ts-analyzer/node_modules', target_is_directory=True)
                (root/'package.json').write_text(json.dumps({'name':'seamgraph-test-fixture','private':True,'type':'module','devDependencies':{'vitest':'3.2.4'}}))
            if language == 'python':
                (root/'pytest.ini').write_text('[pytest]\npythonpath = src\n')
            # These requirements describe assertions authored in this fixture,
            # never requirements inferred for a user's application.
            (root/'seamgraph-contracts.yaml').write_text(f'''version: 1
requirements:
  - id: behavior:read-world
    seamId: seam:world.reader
    description: The public reader returns world
    kind: behavior
links:
  - testId: {test_id}
    requirementIds: [behavior:read-world]
    memberIds: []
    reviewed: true
policies:
  seam:world.reader: require-verified
''')
            for args in [['init','-q'],['add','.'],['-c','user.name=Test','-c','user.email=test@example.invalid','commit','-qm','reviewed contract fixture']]:
                subprocess.run(['git','-C',str(root),*args],check=True,capture_output=True)
            def invoke(*args, expected=0):
                result=subprocess.run([str(CLI),*args,'--root',str(root)],cwd=root,env=ENV,text=True,capture_output=True)
                self.assertEqual(result.returncode,expected,(args,result.stdout,result.stderr))
                return json.loads(result.stdout) if result.stdout.strip() else None
            invoke('test','--runner',runner)
            report=invoke('check','--contracts','--json')
            self.assertEqual(report['contracts']['results'][0]['status'],'verified')
            self.assertFalse(report['contracts']['results'][0]['memberCoverage']['known'])
            with (root/implementation).open('a') as source:
                source.write('\n# changed implementation input\n' if language == 'python' else '\n// changed implementation input\n')
            report=invoke('check','--contracts','--json',expected=2)
            self.assertEqual(report['contracts']['results'][0]['status'],'stale')
            invoke('test','--runner',runner)
            report=invoke('check','--contracts','--json')
            self.assertEqual(report['contracts']['results'][0]['status'],'verified')
            quote='"' if language == 'go' else "'"
            (root/test_file).write_text(test_source.replace(quote+'world'+quote,quote+'unexpected'+quote))
            invoke('test','--runner',runner,expected=2)
            report=invoke('check','--contracts','--json',expected=1)
            self.assertEqual(report['contracts']['results'][0]['status'],'failed')

    def test_go(self):
        self.exercise('go','go','world/contract_test.go','package world\nimport "testing"\nfunc TestContractRead(t *testing.T){if got:=Open().Read();got!="world"{t.Fatalf("unexpected result: %s",got)}}\n','example.test/architecture/world/TestContractRead','world/world.go')

    def test_typescript_vitest(self):
        self.exercise('typescript','vitest','contract.test.ts',"import {describe,it,expect} from 'vitest';\nimport {open} from './world/index';\ndescribe('contract',()=>{it('reads world',()=>{expect(open().read()).toBe('world')})});\n",'contract reads world','world/internal.ts')

    def test_python_pytest(self):
        self.exercise('python','pytest','test_contract.py',"from world.api import open_world\ndef test_read():\n    assert open_world().read() == 'world'\n",'test_contract/test_read','src/world/internal.py')

if __name__ == '__main__':
    unittest.main()
