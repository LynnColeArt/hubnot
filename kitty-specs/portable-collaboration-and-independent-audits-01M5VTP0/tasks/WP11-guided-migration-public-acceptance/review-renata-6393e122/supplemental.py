import subprocess,pathlib,os,tempfile,hashlib,json
root=pathlib.Path('/tmp/wp11-independent-review-h44uvZ'); work=pathlib.Path(tempfile.mkdtemp(prefix='wp11-review-journey-')); repo=work/'repo'; repo.mkdir(); home=work/'home'; home.mkdir(); gitonly=work/'gitonly'; gitonly.mkdir(); (gitonly/'git').symlink_to('/usr/bin/git')
env={'PATH':str(gitonly),'HOME':str(home),'GIT_CONFIG_NOSYSTEM':'1','GIT_CONFIG_GLOBAL':os.devnull,'GIT_TERMINAL_PROMPT':'0'}; transcript=[]
def run(args,ok=True):
 p=subprocess.run(args,cwd=repo,env=env,input='',capture_output=True,text=True); transcript.append({'argv':args,'exit':p.returncode,'stdout':p.stdout,'stderr':p.stderr}); assert (p.returncode==0)==ok,(args,p.stdout,p.stderr); return p.stdout+p.stderr
run(['/usr/bin/git','init','--quiet','--initial-branch=main']); run(['/usr/bin/git','-c','user.name=Independent review','-c','user.email=review@example.invalid','commit','--quiet','--allow-empty','-m','independent public fixture'])
binary=str(root/'hn-noforge'); session=str(work/'session'); archive=work/'archive.hn'
run([binary,'migrate','plan','--source-kind','local','--source-repository',str(repo),'--ref','refs/heads/main','--destination-kind','archive','--destination-repository',str(archive),'--session',session]); assert not archive.exists()
view=json.loads(run([binary,'migrate','inspect','--session',session])); plan=view['plan']; assert len(plan['intents'])==1
for extra in [['--destination-repository',str(work/'other')],['--source-kind','local'],['--map','refs/heads/main=refs/heads/other'],['--decision','all=preserve'],['--yes']]:
 out=run([binary,'migrate','apply','--session',session,'--confirm-apply']+extra,False); assert 'not valid' in out or 'flag provided but not defined' in out; assert not archive.exists()
run([binary,'migrate','apply','--session',session,'--confirm-apply','--actor','reviewed-operator']); digest=hashlib.sha256(archive.read_bytes()).hexdigest()
out=run([binary,'migrate','resume','--session',session,'--confirm-apply','--actor','changed-operator'],False); assert 'migration actor changed' in out; assert hashlib.sha256(archive.read_bytes()).hexdigest()==digest
run([binary,'migrate','resume','--session',session,'--confirm-apply','--actor','reviewed-operator']); assert hashlib.sha256(archive.read_bytes()).hexdigest()==digest
assert json.loads(run([binary,'migrate','inspect','--session',session]))['plan']==plan
(root/'supplemental-processes.json').write_text(json.dumps(transcript,indent=2)+'\n'); (root/'supplemental-result.txt').write_text('PASS: real noforge CLI; Git-only PATH; isolated HOME; no credentials; read-only plan; five selection/blanket overrides refused before archive visibility; exact actor-bound apply/resume; wrong actor refused; archive digest and saved plan unchanged.\n'); print((root/'supplemental-result.txt').read_text())
