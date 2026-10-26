package main

import (
 "encoding/json"
 "strings"
 "testing"
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
 bad := IssueState{Title:"unrelated",Status:"open",Metadata:map[string]string{"audit/finding":identity,"audit/result":result.ID,"audit/request":eventID([]byte("counterfeit"))}}
 if _,err:=executeIssueMutation(issueOptions{command:"open",state:&bad});err!=nil{t.Fatal(err)}
 first,err:=followUpAudit(auditOptions{id:result.ID,finding:" MIXED ",operation:"first"},id)
 if err!=nil || len(first.Associations)!=1 || first.Associations[0].Existing {t.Fatalf("counterfeit link suppressed create: %#v %v",first,err)}
 good := IssueState{Title:"other publisher",Status:"open",Metadata:map[string]string{"audit/finding":identity,"audit/result":result.ID,"audit/request":result.Event.Audit.Request}}
 if _,err:=executeIssueMutation(issueOptions{command:"open",state:&good});err!=nil{t.Fatal(err)}
 again,err:=followUpAudit(auditOptions{id:result.ID,operation:"second"},id)
 if err!=nil || len(again.Associations)!=2 || !again.Concurrent {t.Fatalf("concurrent association hidden: %#v %v",again,err)}
 if auditFindingIdentity(&AuditFact{Scope:"other",Rule:result.Event.Audit.Rule},"mixed")==identity || auditFindingIdentity(&AuditFact{Scope:result.Event.Audit.Scope,Rule:"other"},"mixed")==identity {t.Fatal("dedup crossed scope/rule")}
}

func TestWP03ReviewUnicodeCollisionSelectiveRecovery(t *testing.T) {
 binary:=buildOperationalBinary(t)
 root,id,at:=auditFixture(t)
 auditAuthorizeFixture(t,id,at)
 safe:=reviewWP03Result(t,id,at,[]AuditFinding{{Key:"safe",Title:"keep original title"}})
 first:=auditPublic(t,binary,root,true,"follow-up",safe.ID,"--operation","safe-original","--json")
 var original auditFollowUpResult
 if err:=json.Unmarshal(first,&original);err!=nil || len(original.Associations)!=1 {t.Fatal(string(first),err)}
 // Kelvin sign and ASCII K lowercase to k; EM SPACE is trimmed.
 collision:=reviewWP03Result(t,id,at,[]AuditFinding{{Key:"a-earlier",Title:"must remain uncreated"},{Key:"\u2003K\u2003",Title:"Kelvin"},{Key:"K",Title:"ASCII"},{Key:"safe",Title:"must not overwrite"}})
 beforeRefs:=mustGitText(t,"for-each-ref","--format=%(refname) %(objectname)","refs/hn")
 beforePrivate:=auditPrivateState(t,root)
 beforeIssues,err:=collectIssueEventsForAuditTest();if err!=nil {t.Fatal(err)}
 beforeResult:=auditPublic(t,binary,root,true,"show",collision.ID,"--json")
 for _,args:=range [][]string{{"--operation","unicode-all"},{"--finding","k","--operation","unicode-selected"},{"--finding","\u2003K\u2003","--operation","unicode-raw"},{"--operation","safe-original"},{}} {
  auditPublic(t,binary,root,false,append([]string{"follow-up",collision.ID,"--json"},args...)...)
  if got:=mustGitText(t,"for-each-ref","--format=%(refname) %(objectname)","refs/hn");got!=beforeRefs {t.Fatal("ref changed for",args)}
  if got:=auditPrivateState(t,root);got!=beforePrivate {t.Fatal("private intent changed for",args)}
  got,err:=collectIssueEventsForAuditTest();if err!=nil || len(got)!=len(beforeIssues){t.Fatal("issue changed for",args,err)}
  if got:=auditPublic(t,binary,root,true,"show",collision.ID,"--json");string(got)!=string(beforeResult){t.Fatal("signed result changed")}
 }
 selected:=auditPublic(t,binary,root,true,"follow-up",collision.ID,"--finding"," SAFE ","--operation","safe-selected","--json")
 var repeated auditFollowUpResult
 if err:=json.Unmarshal(selected,&repeated);err!=nil || len(repeated.Associations)!=1 || repeated.Associations[0].IssueID!=original.Associations[0].IssueID || !repeated.Associations[0].Existing {t.Fatal("selective dedup failed",string(selected),err)}
 shown:=issuePublicData[publicIssueView](t,issuePublicCall(t,binary,root,nil,"","show",original.Associations[0].IssueID,"--json"))
 if shown.State==nil || shown.State.Title!="keep original title" {t.Fatal("dedup overwrote original",shown)}
 next:=reviewWP03Result(t,id,at,[]AuditFinding{{Key:" SAFE ",Title:"new title"}})
 last:=auditPublic(t,binary,root,true,"follow-up",next.ID,"--json")
 if !strings.Contains(string(last),original.Associations[0].IssueID) {t.Fatal("cross-result normalized dedup failed",string(last))}
 t.Logf("five Unicode ambiguity selections preserved refs/private state/signed result/issue count=%d; selective and cross-result dedup returned %s",len(beforeIssues),original.Associations[0].IssueID)
}
