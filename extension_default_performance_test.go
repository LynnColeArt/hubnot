package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

var defaultBenchmarkBaseline = flag.String("hn-default-baseline-assets", "", "immutable accepted WP01 assets; opt-in paired measurement")
var defaultBenchmarkOutput = flag.String("hn-default-performance-output", "", "new external output directory (must not exist)")
var defaultBudgetMutant = flag.Bool("hn-default-budget-mutant", false, "deliberate elapsed/RSS regression control")

func defaultBudget(baseline, candidate, floor float64) bool {
	return candidate-baseline <= math.Max(.1*baseline, floor)
}
func TestExtensionDefaultPerformanceBudgets(t *testing.T) {
	for _, c := range []struct {
		name                       string
		baseline, candidate, floor float64
		accepted                   bool
	}{
		{"elapsed-floor-at", 1e6, 6e6, 5e6, true}, {"elapsed-floor-over", 1e6, 6e6 + 1, 5e6, false},
		{"elapsed-percent-at", 1e8, 1.1e8, 5e6, true}, {"elapsed-percent-over", 1e8, 1.1e8 + 1, 5e6, false},
		{"RSS-floor-at", 1000, 2024, 1024, true}, {"RSS-floor-over", 1000, 2025, 1024, false},
		{"RSS-percent-at", 20000, 22000, 1024, true}, {"RSS-percent-over", 20000, 22001, 1024, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if defaultBudget(c.baseline, c.candidate, c.floor) != c.accepted {
				t.Fatal("incorrect exact budget boundary")
			}
		})
	}
	if *defaultBudgetMutant {
		if !defaultBudget(1e6, 20e6, 5e6) || !defaultBudget(1000, 9000, 1024) {
			t.Fatal("deliberate elapsed/RSS regression rejected")
		}
	}
}

// The driver preserves the accepted Python monotonic / GNU Time bracket and
// executes real prebuilt, source-bound binaries. Python is qualification tooling,
// never a dependency of the native product. No accepted evidence is written.
func TestExtensionDefaultPerformanceCapture(t *testing.T) {
	if *defaultBenchmarkBaseline == "" {
		t.Skip("explicit baseline assets required for paired performance capture")
	}
	if *defaultBenchmarkOutput == "" || *defaultDonor == "" {
		t.Fatal("new output and public donor bundle required")
	}
	if _, err := os.Stat(*defaultBenchmarkOutput); !os.IsNotExist(err) {
		t.Fatal("output must not exist; preserve every previous attempt")
	}
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "capture.py")
	if err = os.WriteFile(script, []byte(defaultPerformanceDriver), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", script, source, *defaultBenchmarkBaseline, *defaultDonor, *defaultBenchmarkOutput)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		t.Fatal("paired capture failed; all observations retained", err)
	}
}

func TestExtensionDefaultPerformanceEvidence(t *testing.T) {
	path := "docs/qualification/optional-extensions/default/metrics.csv"
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		t.Skip("performance evidence not published yet")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 4321 {
		t.Fatalf("want 4320 complete observations, got %d", len(records)-1)
	}
	groups := map[string]map[string][]float64{}
	pairs := map[string]map[string]bool{}
	for _, r := range records[1:] {
		if len(r) != 13 {
			t.Fatal("wrong metric columns")
		}
		group := r[0] + "/" + r[1] + "/" + r[2] + "/" + r[3]
		pair := group + "/" + r[4]
		n, e := strconv.Atoi(r[4])
		if e != nil || n < 1 || n > 30 {
			t.Fatal("invalid pair index", r[4])
		}
		order := "0"
		if (n%2 == 1 && r[6] == "candidate") || (n%2 == 0 && r[6] == "baseline") {
			order = "1"
		}
		if r[5] != order {
			t.Fatal("pair order is not alternating", pair)
		}
		if pairs[pair] == nil {
			pairs[pair] = map[string]bool{}
		}
		if pairs[pair][r[6]] {
			t.Fatal("duplicate pair role", pair)
		}
		pairs[pair][r[6]] = true
		if r[9] != "0" {
			t.Fatal("failed observation", pair)
		}
		if groups[group] == nil {
			groups[group] = map[string][]float64{}
		}
		for i, metric := range []string{"elapsed", "RSS"} {
			v, e := strconv.ParseFloat(r[7+i], 64)
			if e != nil || v <= 0 {
				t.Fatal("invalid sample", r)
			}
			key := r[6] + "/" + metric
			groups[group][key] = append(groups[group][key], v)
		}
	}
	if len(groups) != 72 || len(pairs) != 2160 {
		t.Fatal("omitted group/pair", len(groups), len(pairs))
	}
	for _, setting := range []string{"frozen-cgo1", "packager-cgo0"} {
		for _, state := range []string{"absent", "large-unrelated"} {
			for _, variant := range []string{"standard", "native-only"} {
				for _, name := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow", "memory", "recall", "grep"} {
					if groups[setting+"/"+state+"/"+variant+"/"+name] == nil {
						t.Fatal("missing required measurement group", setting, state, variant, name)
					}
				}
			}
		}
	}
	for pair, roles := range pairs {
		if !roles["baseline"] || !roles["candidate"] || len(roles) != 2 {
			t.Fatal("incomplete pair", pair)
		}
	}
	median := func(values []float64) float64 { sort.Float64s(values); return (values[14] + values[15]) / 2 }
	for group, values := range groups {
		for _, metric := range []string{"elapsed", "RSS"} {
			b, c := values["baseline/"+metric], values["candidate/"+metric]
			if len(b) != 30 || len(c) != 30 {
				t.Fatal("not exactly30 observations per role", group)
			}
			floor := float64(5_000_000)
			if metric == "RSS" {
				floor = 1024
			}
			if !defaultBudget(median(b), median(c), floor) {
				t.Fatalf("retained %s regression: %s baseline=%g candidate=%g", metric, group, median(b), median(c))
			}
		}
	}
	var report struct {
		Status       string
		Observations int
	}
	raw, err := os.ReadFile("docs/qualification/optional-extensions/default/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &report) != nil || report.Status != "pass" || report.Observations != 4320 {
		t.Fatal("default qualification report is not a pass")
	}
	t.Log(fmt.Sprint("complete two-setting evidence: ", len(pairs), " pairs"))
}

const defaultPerformanceDriver = `import csv,hashlib,io,json,os,pathlib,platform,resource,shutil,statistics,subprocess,sys,tarfile,time

def sha(p): return hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
def write(p,v): pathlib.Path(p).write_text(json.dumps(v,indent=2,sort_keys=True)+'\n')
def measure(binary,args,repo,env,prefix):
    prefix=pathlib.Path(prefix); stdout=prefix.with_suffix('.stdout');stderr=prefix.with_suffix('.stderr');rss=prefix.with_suffix('.rss-kib')
    argv=['/usr/bin/time','-f','%M','-o',str(rss),'--',str(binary),*args]
    with stdout.open('wb') as out,stderr.open('wb') as err:
        start=time.perf_counter_ns();p=subprocess.run(argv,cwd=repo,env=env,stdout=out,stderr=err);elapsed=time.perf_counter_ns()-start
    row={'argv':argv,'elapsed_ns':elapsed,'peak_rss_kib':int(rss.read_text().strip().splitlines()[-1]),'exit_status':p.returncode,'output_sha256':sha(stdout),'stderr_sha256':sha(stderr),'loadavg':pathlib.Path('/proc/loadavg').read_text().strip()}
    write(prefix.with_suffix('.record.json'),row);return row
if len(sys.argv)>1 and sys.argv[1]=='--parent':
    amount=int(sys.argv[2]);allocation=bytearray(amount*1024*1024)
    for n in range(0,len(allocation),4096):allocation[n]=1
    v=json.loads(pathlib.Path(sys.argv[3]).read_text());r=measure(v['binary'],v['args'],v['repo'],v['env'],v['prefix']);r.update(parent_allocated_mib=amount,parent_high_water_kib=resource.getrusage(resource.RUSAGE_SELF).ru_maxrss);print(json.dumps(r));sys.exit(0)
source,baseline,donor,output=map(pathlib.Path,sys.argv[1:]);output.mkdir(mode=0o700) # refuse overwrite/retry
source=source.resolve();baseline=baseline.resolve();donor=donor.resolve();output=output.resolve()
runlog=[]
def run(argv,cwd=source,env=None):
    p=subprocess.run(list(map(str,argv)),cwd=cwd,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    runlog.append({'argv':list(map(str,argv)),'cwd':str(cwd),'exit_status':p.returncode,'stdout_sha256':hashlib.sha256(p.stdout).hexdigest(),'stderr':p.stderr.decode(errors='replace')})
    write(output/'setup-commands.json',runlog)
    if p.returncode:raise RuntimeError((argv,p.returncode,p.stderr.decode(errors='replace')))
    return p.stdout
head=run(['git','rev-parse','HEAD']).decode().strip();tree=run(['git','rev-parse','HEAD^{tree}']).decode().strip();assert not run(['git','diff','--name-only','HEAD']).strip()
frozen='eb3f4932c265c0cf1b1f5c19939f212cded40e2c'
manifest=json.loads((source/'docs/qualification/optional-extensions/baseline/manifest.json').read_text());observed=json.loads((source/'docs/qualification/optional-extensions/baseline/native-outcomes.json').read_text())['release_binary_observations'];cases=[r for r in observed if r['variant']=='standard'];assert len(cases)==9
for variant in ['standard','native-only']:assert sha(baseline/('hn-'+variant))==manifest['variants'][variant]['executable_sha256']
assert sha('/usr/bin/time')==manifest['measurement']['launcher']['sha256']
assert sha(baseline/'rss-child-control')==manifest['measurement']['launcher']['control_binary_sha256']
compiler=run(['go','version']).decode().strip();assert compiler=='go version go1.26.4 linux/amd64'
inputs={p:sha(source/p) for p in run(['git','ls-files']).decode().splitlines() if p.endswith('.go') or p in ['go.mod','go.sum']};write(output/'source-inputs.json',inputs)
base_env=dict(os.environ);base_env.update(GOENV='off',GOTOOLCHAIN='local',GOWORK='off',GOPROXY='off',GOSUMDB='off',GONOPROXY='none',GOPRIVATE='none',GOFLAGS='',GOEXPERIMENT='',GOOS='linux',GOARCH='amd64',GOAMD64='v1',GOARM64='v8.0')
for k in ['GIT_DIR','GIT_WORK_TREE']:base_env.pop(k,None)
settings={};binaries={}
for label,rev in [('baseline',frozen),('candidate',head)]:
    archive=run(['git','archive','--format=tar',rev]);folder=output/('source-'+label);folder.mkdir();tarfile.open(fileobj=io.BytesIO(archive)).extractall(folder,filter='data');assert not (folder/'.git').exists()
    archived={str(p.relative_to(folder)):sha(p) for p in folder.rglob('*') if p.is_file() and (p.suffix=='.go' or str(p.relative_to(folder)) in ['go.mod','go.sum'])}
    assert archived==(manifest['go_module_inputs'] if label=='baseline' else inputs),('archive source drift',label)
    for variant in ['standard','native-only']:
        flags=['-mod=readonly','-trimpath','-buildvcs=false','-ldflags=-s -w -buildid= -X main.version=0.0.0-optional-extensions-qualification.1']
        if variant=='native-only':flags+=['-tags=nogithub,noforge']
        binary=output/('packager-'+label)/variant;binary.parent.mkdir(exist_ok=True);env=dict(base_env,CGO_ENABLED='0');run(['go','build',*flags,'-o',binary,'.'],folder,env);binaries[('packager-cgo0',label,variant)]=binary;settings[('packager-cgo0',label,variant)]={'flags':flags,'env':{k:env[k] for k in base_env if k.startswith('GO') or k=='CGO_ENABLED'},'CGO_ENABLED':'0'}
for variant in ['standard','native-only']:
    binary=output/'frozen-candidate'/variant;binary.parent.mkdir(exist_ok=True);flags=['-trimpath','-ldflags','-s -w -X main.version=0.0.0-optional-baseline']
    if variant=='native-only':flags+=['-tags','nogithub,noforge']
    env=dict(base_env,CGO_ENABLED='1',GIT_DIR=run(['git','rev-parse','--absolute-git-dir']).decode().strip(),GIT_WORK_TREE=str(source));run(['go','build',*flags,'-o',binary,'.'],source,env)
    info=run(['go','version','-m',binary]).decode();assert 'vcs.revision='+head in info and 'vcs.modified=false' in info
    binaries[('frozen-cgo1','candidate',variant)]=binary;binaries[('frozen-cgo1','baseline',variant)]=baseline/('hn-'+variant);settings[('frozen-cgo1','candidate',variant)]={'flags':flags,'CGO_ENABLED':'1','GIT_DIR':'build-only assigned lane','GIT_WORK_TREE':'build-only assigned lane'};settings[('frozen-cgo1','baseline',variant)]=manifest['variants'][variant]
identities={'/'.join(k):{'path':str(v),'sha256':sha(v),'source':frozen if k[1]=='baseline' else head,'build':settings[k],'buildinfo':run(['go','version','-m',v]).decode()} for k,v in binaries.items()};write(output/'binaries.json',identities)
gitonly=output/'git-only';gitonly.mkdir();(gitonly/'git').symlink_to(shutil.which('git'))
corpora={};environments={};refs={};fixture=source/'testdata/compatibility/0.0.3-dev';expected=json.loads((fixture/'observations.json').read_text())
assert sha(fixture/'public.bundle')==manifest['measurement']['corpus_bundle_sha256']
for state in ['absent','large-unrelated']:
    repo=output/('corpus-'+state);home=output/('home-'+state);home.mkdir();env={'PATH':str(gitonly),'HOME':str(home),'GIT_CONFIG_NOSYSTEM':'1','GIT_CONFIG_GLOBAL':'/dev/null','GIT_TERMINAL_PROMPT':'0','GIT_ASKPASS':'','LANG':'C','LC_ALL':'C'}
    run(['git','-c','maintenance.auto=false','-c','gc.auto=0','clone','-q',fixture/'public.bundle',repo],output,env);run(['git','fetch','--no-write-fetch-head',fixture/'public.bundle','+refs/hn/*:refs/hn/*'],repo,env);initial=run(['git','for-each-ref','--format=%(refname) %(objectname)'],repo,env).decode().strip().splitlines();assert initial==manifest['measurement']['corpus_refs']
    if state=='large-unrelated':
        run(['git','fetch','--no-write-fetch-head',donor,'+refs/hn/artifacts/*:refs/hn/artifacts/*'],repo,env)
        custom_head=run(['git','for-each-ref','--format=%(objectname)','refs/hn/artifacts/'],repo,env).decode().strip();assert len(custom_head.splitlines())==1
        assert run(['git','rev-list','--count',custom_head],repo,env).decode().strip()=='10001'
    refs[state]=run(['git','for-each-ref','--format=%(refname) %(objectname)'],repo,env).decode().strip().splitlines();corpora[state]=repo;environments[state]=env
    assert [r for r in refs[state] if not r.startswith('refs/hn/artifacts/')]==initial
    for p in ['identity.json','identities','active','rotation.json','keyring']:assert not (repo/'.git/hn'/p).exists()
# Prove the large donor is a genuine signed stream through the live generic reader.
large=run([binaries[('frozen-cgo1','candidate','standard')],'artifact','list','--type','example.default/note','--limit','1','--json'],corpora['large-unrelated'],environments['large-unrelated']);page=json.loads(large);assert page['ok'] and len(page['data']['items'])==1 and page['data']['next_cursor']
write(output/'corpora.json',{'refs':refs,'environments':environments,'public_bundle_sha256':sha(fixture/'public.bundle'),'large_bundle_sha256':sha(donor),'large_records':10000,'generic_live_read_sha256':hashlib.sha256(large).hexdigest()})
def subset(w,g,path=''):
    if isinstance(w,dict):
        assert isinstance(g,dict),path
        for k,v in w.items():
            if not path and (k in ('queryDigest','warning') or (k=='snapshot' and isinstance(v,str))):continue
            assert k in g,path+'/'+k;subset(v,g[k],path+'/'+k)
    elif isinstance(w,list):
        assert isinstance(g,list) and len(w)==len(g),path
        for i,(a,b) in enumerate(zip(w,g)):subset(a,b,path+'/'+str(i))
    else:assert w==g,(path,w,g)
def verify(case,raw):
    got=json.loads(raw)
    if case!='grep':subset(expected[case],got)
    else:assert got['matched']==2 and got['returned']==2 and not got['truncated']
controls=[];control_dir=output/'controls';control_dir.mkdir();issue=next(r['argv'] for r in cases if r['case']=='issue');targets=[('native-zero',baseline/'rss-child-control',['0','self']),('native-child-64mib',baseline/'rss-child-control',['64','self']),('native-waited-descendant-64mib',baseline/'rss-child-control',['64','descendant']),('frozen-issue-read',baseline/'hn-standard',issue)]
for repeat in range(1,4):
    for parent in [0,64,128]:
        for name,binary,args in targets:
            prefix=control_dir/f'{name}-parent{parent}-run{repeat}';config=prefix.with_suffix('.config.json');write(config,{'binary':str(binary),'args':args,'repo':str(corpora['absent']),'env':environments['absent'],'prefix':str(prefix)})
            r=json.loads(run([sys.executable,__file__,'--parent',str(parent),config],output));r.update(case=name,repetition=repeat);controls.append(r)
medians={name:{str(p):statistics.median(r['peak_rss_kib'] for r in controls if r['case']==name and r['parent_allocated_mib']==p) for p in [0,64,128]} for name,_,_ in targets}
write(output/'controls.json',{'observations':controls,'medians':medians})
assert all(r['peak_rss_kib']<=4096 for r in controls if r['case']=='native-zero')
for name in ['native-child-64mib','native-waited-descendant-64mib']:assert all(v>=60*1024 for v in medians[name].values()) and max(medians[name].values())-min(medians[name].values())<=4096
assert max(medians['frozen-issue-read'].values())-min(medians['frozen-issue-read'].values())<=4096
assert all(r['parent_high_water_kib']>=128*1024 and r['peak_rss_kib']<32*1024 for r in controls if r['case']=='frozen-issue-read' and r['parent_allocated_mib']==128)
print('causal parent/direct/waited-descendant controls passed',flush=True)
rawdir=output/'raw';rawdir.mkdir();warm=output/'warmups';warm.mkdir();rows=[];failures=[]
for setting in ['frozen-cgo1','packager-cgo0']:
    # Warm every role/variant/corpus equivalently before retained observations.
    for state in corpora:
        for variant in ['standard','native-only']:
            for role in ['baseline','candidate']:
                for case in cases:
                    prefix=warm/f"{setting}-{state}-{variant}-{role}-{case['case']}";result=measure(binaries[(setting,role,variant)],case['argv'],corpora[state],environments[state],prefix);assert result['exit_status']==0;verify(case['case'],prefix.with_suffix('.stdout').read_bytes())
    with (output/(setting+'.csv')).open('w',newline='') as f:
        writer=csv.DictWriter(f,fieldnames=['setting','state','variant','case','pair','order','role','elapsed_ns','peak_rss_kib','exit_status','output_sha256','stderr_sha256','loadavg'],lineterminator='\n');writer.writeheader()
        for n in range(1,31):
            for state in corpora:
                for case in cases:
                    for variant in (['standard','native-only'] if n%2 else ['native-only','standard']):
                        for order,role in enumerate(['baseline','candidate'] if n%2 else ['candidate','baseline']):
                            prefix=rawdir/f"{setting}-{state}-{variant}-{case['case']}-{n:02d}-{role}";r=measure(binaries[(setting,role,variant)],case['argv'],corpora[state],environments[state],prefix);row={k:v for k,v in r.items() if k!='argv'};row.update(setting=setting,state=state,variant=variant,case=case['case'],pair=n,order=order,role=role);writer.writerow(row);f.flush();rows.append(row)
                            try:assert r['exit_status']==0;verify(case['case'],prefix.with_suffix('.stdout').read_bytes())
                            except Exception as e:failures.append({'observation':row,'error':str(e)})
            print(setting,'paired round',n,'/30',flush=True)
summary=[]
for setting in ['frozen-cgo1','packager-cgo0']:
    for state in corpora:
        for variant in ['standard','native-only']:
            for case in cases:
                chosen=[r for r in rows if r['setting']==setting and r['state']==state and r['variant']==variant and r['case']==case['case']];assert len(chosen)==60
                s={'setting':setting,'state':state,'variant':variant,'case':case['case'],'pairs':30}
                for metric,absolute in [('elapsed_ns',5_000_000),('peak_rss_kib',1024)]:
                    b=statistics.median(r[metric] for r in chosen if r['role']=='baseline');c=statistics.median(r[metric] for r in chosen if r['role']=='candidate');bound=max(.1*b,absolute);s[metric]={'baseline_median':b,'candidate_median':c,'delta':c-b,'bound':bound,'pass':c-b<=bound}
                    if c-b>bound:failures.append({'budget':s.copy(),'metric':metric})
                summary.append(s)
assert len(rows)==4320
for state in corpora:
    assert run(['git','for-each-ref','--format=%(refname) %(objectname)'],corpora[state],environments[state]).decode().strip().splitlines()==refs[state]
    for p in ['identity.json','identities','active','rotation.json','keyring']:assert not (corpora[state]/'.git/hn'/p).exists()
assert inputs=={p:sha(source/p) for p in inputs}
report={'schema':'hn.optional-extensions-default-performance/1','status':'pass' if not failures else 'fail','source':head,'tree':tree,'compiler':compiler,'host':{'os':platform.system(),'machine':platform.machine(),'kernel':platform.release(),'git':run(['git','--version']).decode().strip(),'filesystem':run(['findmnt','-n','-o','FSTYPE','-T',output]).decode().strip()},'observations':len(rows),'pairs_per_setting':1080,'method':manifest['measurement']['launcher'],'summary':summary,'failures':failures,'inputs_sha256':sha(output/'source-inputs.json'),'binary_manifest_sha256':sha(output/'binaries.json'),'corpora_sha256':sha(output/'corpora.json'),'controls_sha256':sha(output/'controls.json'),'csv_sha256':{s:sha(output/(s+'.csv')) for s in ['frozen-cgo1','packager-cgo0']}}
write(output/'report.json',report);print('FINAL',report['status'],'observations',len(rows),'failures',len(failures),flush=True)
if failures:sys.exit(1)
`
