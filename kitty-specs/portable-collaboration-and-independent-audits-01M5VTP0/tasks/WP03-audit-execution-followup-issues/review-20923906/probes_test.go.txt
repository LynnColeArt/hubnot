package main

import (
 "testing"
 "os/exec"
 "strings"
)

func reviewWP03Result(t *testing.T, id *Identity, at string, findings []AuditFinding) *StoredEvent {
 t.Helper()
 request, err := AuditRequest(at, "check", "", id)
 if err != nil { t.Fatal(err) }
 result, err := RecordAuditResult(request.ID, "", id, executionResult{Outcome:"failed",ExitCode:1,Log:[]byte("review fixture")}, findings, "host", "linux/amd64", "independent-review")
 if err != nil { t.Fatal(err) }
 return result
}

func TestWP03ReviewAuthenticatedLinksAndConcurrentAssociations(t *testing.T) {
 _, id, at := auditFixture(t)
 auditAuthorizeFixture(t,id,at)
 result := reviewWP03Result(t,id,at,[]AuditFinding{{Key:"mixed",Title:"actual finding"}})
 identity := auditFindingIdentity(result.Event.Audit," MiXeD ")
 // A signed issue with a wrong request link must not suppress work.
 bad := IssueState{Title:"unrelated",Status:"open",Metadata:map[string]string{"audit/finding":identity,"audit/result":result.ID,"audit/request":eventID([]byte("counterfeit"))}}
 if _,err:=executeIssueMutation(issueOptions{command:"open",state:&bad});err!=nil{t.Fatal(err)}
 first,err:=followUpAudit(auditOptions{id:result.ID,finding:" MIXED ",operation:"first"},id)
 if err!=nil || len(first.Associations)!=1 || first.Associations[0].Existing {t.Fatalf("counterfeit link suppressed create: %#v %v",first,err)}
 // A second genuine association remains visible and is reported for reconciliation.
 good := IssueState{Title:"other publisher",Status:"open",Metadata:map[string]string{"audit/finding":identity,"audit/result":result.ID,"audit/request":result.Event.Audit.Request}}
 if _,err:=executeIssueMutation(issueOptions{command:"open",state:&good});err!=nil{t.Fatal(err)}
 again,err:=followUpAudit(auditOptions{id:result.ID,operation:"second"},id)
 if err!=nil || len(again.Associations)!=2 || !again.Concurrent {t.Fatalf("concurrent association hidden: %#v %v",again,err)}
 if auditFindingIdentity(&AuditFact{Scope:"other",Rule:result.Event.Audit.Rule},"mixed")==identity || auditFindingIdentity(&AuditFact{Scope:result.Event.Audit.Scope,Rule:"other"},"mixed")==identity {t.Fatal("dedup crossed scope/rule")}
}

func TestWP03ReviewNormalizedDuplicateAcceptedResult(t *testing.T) {
 binary:=buildOperationalBinary(t)
 root,id,at:=auditFixture(t)
 auditAuthorizeFixture(t,id,at)
 result:=reviewWP03Result(t,id,at,[]AuditFinding{{Key:"X",Title:"first spelling"},{Key:"x",Title:"second spelling"}})
 before,err:=collectIssueEventsForAuditTest();if err!=nil{t.Fatal(err)}
 beforeRefs:=mustGitText(t,"for-each-ref","--format=%(refname) %(objectname)","refs/hn")
 t.Logf("accepted refs before (%d):\n%s",strings.Count(beforeRefs,"\n")+1,beforeRefs)
 auditPublic(t,binary,root,false,"follow-up",result.ID,"--finding","x","--operation","select-collision","--json")
 if got:=mustGitText(t,"for-each-ref","--format=%(refname) %(objectname)","refs/hn");got!=beforeRefs {t.Fatal("single ambiguous selection changed refs")}
 cmd:=exec.Command(binary,"audit","follow-up",result.ID,"--operation","normalized-collision","--json")
 cmd.Dir=root;cmd.Env=operationalTestEnvironment(t.TempDir())
 output,commandErr:=cmd.CombinedOutput()
 t.Logf("first public CLI exit error=%v response=%s",commandErr,output)
 if commandErr==nil{t.Fatal("expected deterministic operation conflict")}
 afterRefs:=mustGitText(t,"for-each-ref","--format=%(refname) %(objectname)","refs/hn")
 t.Logf("accepted refs after (%d):\n%s",strings.Count(afterRefs,"\n")+1,afterRefs)
 retry:=auditPublic(t,binary,root,true,"follow-up",result.ID,"--operation","normalized-collision","--json")
 t.Logf("retry response: %s", retry)
 after,readErr:=collectIssueEventsForAuditTest();if readErr!=nil{t.Fatal(readErr)}
 t.Logf("accepted signed normalized collision; issue count before=%d after=%d",len(before),len(after))
 if len(after)!=len(before){t.Fatalf("follow-up refused an accepted result only AFTER publishing %d issue(s)",len(after)-len(before))}
}
