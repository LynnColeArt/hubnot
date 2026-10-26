"""Review-only inspection; not part of the Go/Git packaging implementation."""
import hashlib
import json
import pathlib
import subprocess
import tarfile

root = pathlib.Path('/tmp/hn-wp01-evidence')
repo = pathlib.Path('/home/lynn/projects/nichthub/.worktrees/portable-collaboration-and-independent-audits-01M5VTP0-lane-a')
source = '86dc00678c42d495c6442addfcc8da8abd983ffa'
def git(*args):
    return subprocess.check_output(['git', '--no-replace-objects', *args], cwd=repo).decode().strip()
def sha(data):
    return hashlib.sha256(data).hexdigest()
p = json.loads((root/'first/PROVENANCE.json').read_bytes())
assert p['schema'] == 'hn.release/1'
assert p['source_commit'] == source == git('rev-parse', 'HEAD')
assert p['source_tree'] == git('rev-parse', source+'^{tree}')
assert p['source_timestamp'] == int(git('show', '-s', '--format=%ct', source))
assert p['compiler'] == 'go1.26.4' and p['version'] == '0.0.2-dev'
expected = {'hubnot_0.0.2-dev_source.tar.gz', 'PROVENANCE.json', 'SHA256SUMS'}
for target in ('darwin_amd64', 'darwin_arm64', 'linux_amd64', 'linux_arm64'):
    for suffix in ('', '_native-only'):
        expected.add('hubnot_0.0.2-dev_'+target+suffix+'.tar.gz')
assert expected == {x.name for x in (root/'first').iterdir()}
assert expected == {x.name for x in (root/'second-different-path').iterdir()}
for name in expected:
    assert (root/'first'/name).read_bytes() == (root/'second-different-path'/name).read_bytes()
assert [a['name'] for a in p['assets']] == sorted(expected-{'PROVENANCE.json','SHA256SUMS'})
for a in p['assets']:
    assert sha((root/'first'/a['name']).read_bytes()) == a['sha256']
    if a['name'].endswith('_source.tar.gz'):
        assert 'build' not in a
        continue
    prefix = a['name'].removesuffix('.tar.gz')+'/'
    with tarfile.open(root/'first'/a['name']) as archive:
        names = archive.getnames()
        b = json.load(archive.extractfile(prefix+'BUILD.json'))
        assert a['build'] == b
        assert b['schema'] == 'hn.release-build/1'
        for key in ('source_commit','source_tree','source_timestamp','compiler','version'):
            assert b[key] == p[key], (a['name'],key)
        assert sha(archive.extractfile(prefix+'hn').read()) == b['executable_sha256']
        assert b['target'] in ('darwin/amd64','darwin/arm64','linux/amd64','linux/arm64')
        assert b['variant'] in ('standard','native-only')
        expected_flags=['-mod=readonly','-trimpath','-buildvcs=false','-ldflags=-s -w -buildid= -X main.version=0.0.2-dev']
        if b['variant']=='native-only':
            expected_flags.append('-tags=nogithub,noforge')
        assert b['flags'] == expected_flags
        env=dict(x.split('=',1) for x in b['environment'])
        assert env['CGO_ENABLED']=='0' and env['GOWORK']=='off' and env['GOTOOLCHAIN']=='local'
        assert env['GOPROXY']=='off' and env['GOSUMDB']=='off'
        assert env['GOFLAGS']==env['GOEXPERIMENT']==''
        assert env['GOOS']+'/'+env['GOARCH']==b['target']
        assert env['GOAMD64']=='v1' and env['GOARM64']=='v8.0'
        assert names==sorted(set(names))
        assert all(n.startswith(prefix) and '..' not in n.split('/') for n in names)
        print('PASS inventory and exact source/toolchain/build provenance:', a['name'])
tag_result=subprocess.run(['git','show-ref','--tags'],cwd=repo,stdout=subprocess.PIPE,check=False)
assert tag_result.returncode in (0,1)
current_tags=tag_result.stdout
assert current_tags==(root/'tags-before.txt').read_bytes()==(root/'tags-after.txt').read_bytes()
print('PASS 11-file matrix, 9 archive digests, manifest/build equality, exact commit/tree/time, controlled flags/environment, tags unchanged')
