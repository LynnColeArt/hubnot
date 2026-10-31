package main

import (
 "bytes"
 "encoding/json"
 "os"
 "path/filepath"
 "reflect"
 "strings"
 "testing"
)

func exchangeDestination(t *testing.T, source *artifactCLIFixture, format string) *artifactCLIFixture {
 t.Helper()
 root := t.TempDir()
 dst := &artifactCLIFixture{t:t,binary:source.binary,repo:filepath.Join(root,"repo"),home:filepath.Join(root,"home")}
 dst.git("init","--quiet","--object-format="+format)
 return dst
}
func exchangeWrite(t *testing.T, value any) string {
 t.Helper(); raw,e:=json.Marshal(value); if e!=nil { t.Fatal(e) }; path:=filepath.Join(t.TempDir(),"exchange.json"); if e=os.WriteFile(path,raw,0600);e!=nil {t.Fatal(e)};return path
}
func exchangeExport(f *artifactCLIFixture, ids ...string) (string,map[string]any) {
 f.t.Helper();path:=filepath.Join(f.t.TempDir(),"public.json");args:=[]string{"artifact","export","--output",path,"--json"};for _,id:=range ids {args=append(args,"--id",id)};return path,f.call("","",args...)["data"].(map[string]any)
}
func TestArtifactExchangeIndependentCLI(t *testing.T) {
 for _,format:=range []string{"sha1","sha256"} {t.Run(format,func(t *testing.T){
 src:=artifactCLIFormat(t,format); dst:=exchangeDestination(t,src,format)
 typ:=revisionEvent(src.publish(artifactStoreType,"type",""))
 opening:=revisionEvent(revisionCall(src,"open",revisionInput(t,nil,[]string{},revisionPayload(typ,"public exact text\n世界")),"open",""))
 tip:=src.git("rev-parse",artifactRef(src.actor))
 tail:=revisionEvent(revisionCall(src,"open",revisionInput(t,nil,[]string{},revisionPayload(typ,"unselected tail")),"tail",""))
 marker:="WP10-PRIVATE-CREDENTIAL-KEY-GRANT-JOURNAL-SENTINEL"
 for _,relative:=range []string{".hn/private-key-marker",".git/hn/extensions/grants/private",".git/hn/artifacts/intents/private"} {p:=filepath.Join(src.repo,relative);if e:=os.MkdirAll(filepath.Dir(p),0700);e!=nil{t.Fatal(e)};if e:=os.WriteFile(p,[]byte(marker),0600);e!=nil{t.Fatal(e)}}
 before:=src.snapshot();path,out:=exchangeExport(src,opening);src.unchanged(before)
 raw,e:=os.ReadFile(path);if e!=nil{t.Fatal(e)}
 if bytes.Contains(raw,[]byte(marker))||bytes.Contains(raw,[]byte(src.home))||bytes.Contains(raw,[]byte(src.repo)){t.Fatal("private export leak")}
 // A red control proves that the leak detector recognizes prohibited bytes.
 if !bytes.Contains(append(append([]byte{},raw...),[]byte(marker)...),[]byte(marker)){t.Fatal("leak detector ineffective")}
 if out["output_digest"]!=eventID(raw)||out["records"]!=float64(2){t.Fatal(out)}
 var wire map[string]any;if e=json.Unmarshal(raw,&wire);e!=nil{t.Fatal(e)}
 if wire["protocol"]!="hn-artifact-exchange/1"||wire["mode"]!="accepted"{t.Fatal(wire)}
 refs:=wire["refs"].([]any);if len(refs)!=1||refs[0].(map[string]any)["oid"]!=tip||refs[0].(map[string]any)["ref"]!=artifactRef(src.actor){t.Fatal("not minimal canonical prefix",refs)}
 again,_:=exchangeExport(src,opening);second,_:=os.ReadFile(again);if !bytes.Equal(raw,second){t.Fatal("nondeterministic export")}
 result:=dst.call("","","artifact","import","--input",path,"--json")["data"].(map[string]any)
 ids:=[]any{typ,opening};if strings.Compare(typ,opening)>0{ids=[]any{opening,typ}}
 if !reflect.DeepEqual(result["imported_ids"],ids)||result["source_cut"]==""{t.Fatal(result)}
 if got:=closureItems(dst,opening);!reflect.DeepEqual(got,closureItems(src,opening)){t.Fatal("public reconstruction differs",got)}
 dst.call("","not_found","artifact","show",tail,"--json")
 if dst.git("for-each-ref","--format=%(refname)","refs/hn/artifacts/")!=""{t.Fatal("import installed local signing root")}
 for _,p:=range []string{dst.home,filepath.Join(dst.repo,".hn"),filepath.Join(dst.repo,".git","hn")} {if _,e:=os.Stat(p);!os.IsNotExist(e){t.Fatal("import created private state",p,e)}}
 before=dst.snapshot();dst.call("","","artifact","import","--input",path,"--json");dst.unchanged(before)
 })}
}
func TestArtifactExchangeRefusalsCLI(t *testing.T) {
 src:=artifactCLI(t);typ:=revisionEvent(src.publish(artifactStoreType,"type",""));opening:=revisionEvent(revisionCall(src,"open",revisionInput(t,nil,[]string{},revisionPayload(typ,"accepted")),"open",""));path,_:=exchangeExport(src,opening);raw,_:=os.ReadFile(path)
 var wire map[string]any;_ =json.Unmarshal(raw,&wire)
 tests:=[]struct{name,code string;change func(map[string]any)}{
 {"missing-schema","verification_failure",func(v map[string]any){objects:=v["objects"].([]any);for i,o:=range objects{if o.(map[string]any)["type"]=="blob" {v["objects"]=append(objects[:i:i],objects[i+1:]...);return}}}},
 {"corrupt-hash","verification_failure",func(v map[string]any){v["objects"].([]any)[0].(map[string]any)["data"]="Y29ycnVwdA"}},
 {"padded-base64","invalid_input",func(v map[string]any){v["objects"].([]any)[0].(map[string]any)["data"]="YQ=="}},
 {"duplicate-object","invalid_input",func(v map[string]any){v["objects"]=append(v["objects"].([]any),v["objects"].([]any)[0])}},
 {"duplicate-ref","invalid_input",func(v map[string]any){v["refs"]=append(v["refs"].([]any),v["refs"].([]any)[0])}},
 {"private-ref","invalid_input",func(v map[string]any){v["refs"].([]any)[0].(map[string]any)["ref"]="refs/hn/private/key"}},
 {"null-objects","invalid_input",func(v map[string]any){v["objects"]=nil}},
 {"unknown-mode","invalid_input",func(v map[string]any){v["mode"]="automatic"}},
 {"tag-object","invalid_input",func(v map[string]any){v["objects"].([]any)[0].(map[string]any)["type"]="tag"}},
 }
 for _,tc:=range tests {t.Run(tc.name,func(t *testing.T){dst:=exchangeDestination(t,src,"sha1");var v map[string]any;_ =json.Unmarshal(raw,&v);tc.change(v);bad:=exchangeWrite(t,v);before:=dst.snapshot();dst.call("",tc.code,"artifact","import","--input",bad,"--json");dst.unchanged(before)})}
 for name,body:=range map[string]string{"duplicate-key":strings.Replace(string(raw),`"protocol":`,`"protocol":"hn-artifact-exchange/1","protocol":`,1),"trailing":string(raw)+" {}","null-refs":`{"protocol":"hn-artifact-exchange/1","mode":"accepted","refs":null,"objects":[]}`} {t.Run(name,func(t *testing.T){dst:=exchangeDestination(t,src,"sha1");bad:=filepath.Join(t.TempDir(),"bad");_ =os.WriteFile(bad,[]byte(body),0600);before:=dst.snapshot();dst.call("","invalid_input","artifact","import","--input",bad,"--json");dst.unchanged(before)})}
 t.Run("symlink",func(t *testing.T){dst:=exchangeDestination(t,src,"sha1");link:=filepath.Join(t.TempDir(),"link");_ =os.Symlink(path,link);before:=dst.snapshot();dst.call("","invalid_input","artifact","import","--input",link,"--json");dst.unchanged(before)})
 t.Run("object-format",func(t *testing.T){dst:=exchangeDestination(t,src,"sha256");before:=dst.snapshot();dst.call("","invalid_input","artifact","import","--input",path,"--json");dst.unchanged(before)})
}
func TestArtifactExchangeOpaqueCLI(t *testing.T) {
 src:=artifactCLI(t);dst:=exchangeDestination(t,src,"sha1")
 typ:=revisionEvent(src.publish(artifactStoreType,"type",""));path,_:=exchangeExport(src,typ)
 raw,_:=os.ReadFile(path);var bundle map[string]any;_ =json.Unmarshal(raw,&bundle)
 // Explicit preservation stores public bodies without admitting even valid signed data.
 before:=dst.snapshot();dst.call("","unsupported_protocol","artifact","import","--input",path,"--preserve-opaque","--json");dst.unchanged(before)
 bundle["mode"]="opaque";opaquePath:=exchangeWrite(t,bundle)
 before=dst.snapshot();dst.call("","unsupported_protocol","artifact","import","--input",opaquePath,"--json");dst.unchanged(before)
 result:=dst.call("","","artifact","import","--input",opaquePath,"--preserve-opaque","--json")["data"].(map[string]any)
 if len(result["accepted_refs"].([]any))!=0||len(result["imported_ids"].([]any))!=0{t.Fatal("opaque promoted",result)}
 ids:=result["opaque_ids"].([]any);if len(ids)!=len(bundle["objects"].([]any)){t.Fatal(result)}
 if dst.git("for-each-ref","--format=%(refname)","refs/hn")!=""{t.Fatal("opaque created accepted ref")}
 dst.call("","not_found","artifact","show",typ,"--json")
 id:=ids[0].(string);view:=dst.call("","","artifact","inspect","--opaque","--opaque-id",id,"--json")["data"].(map[string]any)
 if view["opaque_id"]!=id||view["validation"]!="opaque"{t.Fatal(view)}
 out:=filepath.Join(t.TempDir(),"opaque.json");exported:=dst.call("","","artifact","export","--opaque","--opaque-id",id,"--output",out,"--json")["data"].(map[string]any)
 if exported["records"]!=float64(0)||len(exported["refs"].([]any))!=0||!reflect.DeepEqual(exported["opaque_ids"],[]any{id}){t.Fatal(exported)}
 third:=exchangeDestination(t,src,"sha1");third.call("","","artifact","import","--input",out,"--preserve-opaque","--json")
 if !reflect.DeepEqual(view,third.call("","","artifact","inspect","--opaque","--opaque-id",id,"--json")["data"]){t.Fatal("opaque body did not survive independently")}
 // Opaque IDs address only the bounded index, never arbitrary reachable ODB objects.
 absent:="sha256:"+strings.Repeat("1",64);before=dst.snapshot();dst.call("","not_found","artifact","inspect","--opaque","--opaque-id",absent,"--json");dst.unchanged(before)
}
func TestArtifactExchangeSelectorRefusalsCLI(t *testing.T) {
 f:=artifactCLI(t);id:=revisionEvent(f.publish(artifactStoreType,"type",""));out:=filepath.Join(t.TempDir(),"out")
 for _,args:=range [][]string{
 {"export","--id",id,"--id",id,"--output",out},
 {"export","--id",id,"--output",out,"--output",out},
 {"export","--id",id,"--output",out,"--opaque"},
 {"export","--opaque","--opaque-id",id,"--id",id,"--output",out},
 {"export","--output",out},{"import","--input","-","--id",id},
 {"import","--input","-","--input","-"},{"inspect","--opaque","--id",id},
 }{before:=f.snapshot();f.call("","invalid_input",append([]string{"artifact"},append(args,"--json")...)...);f.unchanged(before)}
}
