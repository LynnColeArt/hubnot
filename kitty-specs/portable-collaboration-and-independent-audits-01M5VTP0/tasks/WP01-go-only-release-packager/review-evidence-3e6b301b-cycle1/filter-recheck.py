"""Independent reviewer-only checks of the original implicit-filter defect."""
from pathlib import Path
import hashlib
import json
import os
import shlex
import subprocess
import tempfile
import time

repo=Path('/home/lynn/projects/nichthub/.worktrees/portable-collaboration-and-independent-audits-01M5VTP0-lane-a')
source='3e6b301bb2e3142eff40727edf57a462b3e63e02'
root=Path(tempfile.mkdtemp(prefix='hn-release-rereview-'))
binary=root/'hn-release'
subprocess.run(['go','build','-trimpath','-o',str(binary),'./cmd/hn-release'],cwd=repo,check=True)
tree=subprocess.check_output(['git','rev-parse','HEAD^{tree}'],cwd=repo).decode().strip()
assert tree==subprocess.check_output(['git','rev-parse',source+'^{tree}'],cwd=repo).decode().strip()
print('Validated source:',source,'review tree:',tree)
print('Fresh reviewer packager SHA256:',hashlib.sha256(binary.read_bytes()).hexdigest())
for scope in ('local-clean','conditional-include-process','worktree-clean'):
    fixture=root/scope
    fixture.mkdir()
    def git(*args):
        return subprocess.check_output(['git',*args],cwd=fixture,stderr=subprocess.STDOUT)
    for name,data in {'main.go':'package main\nvar version="dev"\nfunc main(){}\n','go.mod':'module fixture\n\ngo 1.26\n','LICENSE':'license\n','README.md':'readme\n','.gitattributes':'README.md filter=REVIEW\n'}.items():
        (fixture/name).write_text(data)
    git('init','-q');git('config','user.name','Review');git('config','user.email','review@example.invalid')
    git('add','.');git('commit','-qm','fixture')
    marker=root/(scope+'-unexpected-execution')
    key='filter.REVIEW.'+('process' if scope=='conditional-include-process' else 'clean')
    command='touch '+shlex.quote(str(marker))+'; cat'
    if scope=='conditional-include-process':
        included=fixture/'.git/review-config'
        git('config','--file',str(included),key,command)
        branch=git('symbolic-ref','--short','HEAD').decode().strip()
        git('config','includeIf.onbranch:'+branch+'.path',str(included))
    elif scope=='worktree-clean':
        git('config','extensions.worktreeConfig','true')
        git('config','--worktree',key,command)
    else:
        git('config',key,command)
    time.sleep(.01)
    os.utime(fixture/'README.md',None)
    def state():
        return {str(p.relative_to(fixture)):hashlib.sha256(p.read_bytes()).hexdigest() for p in fixture.rglob('*') if p.is_file()}
    before=state()
    output=root/(scope+'-output')
    result=subprocess.run([str(binary),'--source',str(fixture),'--rev','HEAD','--version','1.2.3','--out',str(output),'--targets','linux/amd64','--variants','standard'],capture_output=True,text=True)
    assert result.returncode!=0 and 'filter' in result.stderr
    assert not marker.exists() and not output.exists()
    assert before==state()
    assert not list(root.glob('.hn-release-*'))
    print('PASS',scope,': refusal before execution; source/config/index unchanged; no marker/output/staging;',result.stderr.strip())
print('PASS original clean-filter defect and conditional/include/worktree variants independently rechecked')
