"""Review-only reproducer for implicit execution during Git cleanliness check."""
from pathlib import Path
import hashlib
import os
import shlex
import subprocess
import tempfile
import time

repo=Path('/home/lynn/projects/nichthub/.worktrees/portable-collaboration-and-independent-audits-01M5VTP0-lane-a')
expected='86dc00678c42d495c6442addfcc8da8abd983ffa'
source=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo).decode().strip()
assert source==expected
root=Path(tempfile.mkdtemp(prefix='hn-release-review-filter-'))
packager=root/'hn-release-review'
subprocess.run(['go','build','-trimpath','-o',str(packager),'./cmd/hn-release'],cwd=repo,check=True)
print('Reviewed packager source:',source,flush=True)
print('Packager SHA256:',hashlib.sha256(packager.read_bytes()).hexdigest(),flush=True)
fixture=root/'fixture'
fixture.mkdir()
def git(*args):
    return subprocess.check_output(['git',*args],cwd=fixture,stderr=subprocess.STDOUT)
for name,data in {
    'main.go':'package main\nimport "fmt"\nvar version="dev"\nfunc main(){fmt.Println(version)}\n',
    'go.mod':'module fixture\n\ngo 1.26\n',
    'LICENSE':'license\n','README.md':'readme\n',
    '.gitattributes':'README.md filter=review\n',
}.items():
    (fixture/name).write_text(data)
git('init','-q')
git('config','user.name','review')
git('config','user.email','review@example.invalid')
git('add','.')
git('commit','-qm','clean fixture')
# Configure only after committing: nothing has run this filter before Package.
marker=root/'unexpected-execution'
git('config','filter.review.clean','touch '+shlex.quote(str(marker))+'; cat')
time.sleep(0.01)
os.utime(fixture/'README.md',None)
assert not marker.exists()
args=[str(packager),'--source',str(fixture),'--rev','HEAD','--version','1.2.3','--out',str(root/'output'),'--targets','linux/amd64','--variants','standard']
result=subprocess.run(args,capture_output=True,text=True)
print('Invocation:',args)
print('Exit:',result.returncode)
print('Stdout:',result.stdout.strip())
print('Stderr:',result.stderr.strip())
print('Filter shell/external touch invoked:',marker.exists())
print('Output successfully published:',(root/'output/PROVENANCE.json').exists())
assert result.returncode==0 and marker.exists()
assert (root/'output/PROVENANCE.json').exists()
print('CONFIRMED BLOCKER: a clean source invokes repository-configured shell commands during packaging')
