//go:build !nogithub

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"hubnot/internal/forge"
	"hubnot/internal/githubapi"
)

func projectFixture() githubapi.ProjectV2 {
	return githubapi.ProjectV2{ID: "PVT_A", Title: "Roadmap", Readme: "Plan", Number: 1, URL: "https://github.com/users/a/projects/1", CreatedAt: "2026-01-02T03:04:05Z", UpdatedAt: "2026-01-02T03:04:05Z", Fields: []githubapi.ProjectField{{ID: "F_s", Name: "Status", DataType: "SINGLE_SELECT", Options: []githubapi.ProjectOption{{ID: "O_t", Name: "Todo"}}}, {ID: "F_n", Name: "Estimate", DataType: "NUMBER"}}, Items: []githubapi.ProjectItem{{ID: "I_A", ProjectID: "PVT_A", CreatedAt: "2026-01-02T03:04:05Z", UpdatedAt: "2026-01-02T03:04:05Z", Content: &githubapi.ProjectContent{Type: "DraftIssue", ID: "D_A", Title: "Task", Body: "Work"}, Values: []githubapi.ProjectValue{{Type: "ProjectV2ItemFieldNumberValue", FieldID: "F_n", Number: json.Number("1.25")}, {Type: "ProjectV2ItemFieldSingleSelectValue", FieldID: "F_s", OptionID: "O_t"}}}}}
}
func TestGithubProjectNativeMapping(t *testing.T) {
	p := projectFixture()
	g, err := githubProjectGraph(p, forge.Selection{Project: "user:a/1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 2 || g.Nodes[0].Project.Fields[0].ID == "F_s" {
		t.Fatalf("bad graph: %+v", g)
	}
	n := g.Nodes[1]
	if n.Item.Target.Kind != "draft" || n.Item.Values[githubProjectID("F_n")].Number != "1.25" {
		t.Fatal(n)
	}
}
func TestGithubProjectUnsupportedRequiresSpecificAcknowledgement(t *testing.T) {
	p := projectFixture()
	p.Workflows = 1
	g, err := githubProjectGraph(p, forge.Selection{Project: "user:a/1"})
	if err != nil {
		t.Fatal(err)
	}
	if !forgeResourcePlanReport(g, transferOptions{resource: forge.ResourceProject}).Blocked() {
		t.Fatal("automation silently omitted")
	}
	key := githubProjectOmitKey("automation")
	g, err = githubProjectGraph(p, forge.Selection{Project: "user:a/1", Mappings: map[string]string{key: "true"}})
	if err != nil || forgeResourcePlanReport(g, transferOptions{}).Blocked() {
		t.Fatal(g, err)
	}
}

type githubProjectFake struct {
	p         *githubapi.ProjectV2
	mutations int
	lost      string
	serial    int
}

func projectFakeHeader(p githubapi.ProjectV2) map[string]any {
	return map[string]any{"__typename": "ProjectV2", "id": p.ID, "number": p.Number, "title": p.Title, "readme": p.Readme, "shortDescription": p.ShortDescription, "url": p.URL, "createdAt": p.CreatedAt, "updatedAt": p.UpdatedAt, "creator": map[string]any{"login": "source"}, "closed": p.Closed, "public": p.Public, "template": p.Template, "owner": map[string]any{"__typename": "User", "id": "U_A", "login": "a"}, "workflows": map[string]any{"totalCount": p.Workflows}, "repositories": map[string]any{"totalCount": p.Repositories}, "teams": map[string]any{"totalCount": p.Teams}}
}
func projectFakeField(f githubapi.ProjectField) map[string]any {
	typ := "ProjectV2Field"
	if f.DataType == "SINGLE_SELECT" {
		typ = "ProjectV2SingleSelectField"
	}
	return map[string]any{"__typename": typ, "id": f.ID, "name": f.Name, "dataType": f.DataType, "options": f.Options}
}
func projectFakeItem(i githubapi.ProjectItem) map[string]any {
	var content any
	if i.Content != nil {
		content = map[string]any{"__typename": i.Content.Type, "id": i.Content.ID, "title": i.Content.Title, "body": i.Content.Body, "url": i.Content.URL}
	}
	return map[string]any{"id": i.ID, "project": map[string]any{"id": i.ProjectID}, "createdAt": i.CreatedAt, "updatedAt": i.UpdatedAt, "creator": nil, "isArchived": i.Archived, "content": content}
}
func projectFakeView(v githubapi.ProjectView) map[string]any {
	count := func(n int) any { return map[string]any{"totalCount": n} }
	return map[string]any{"id": v.ID, "name": v.Name, "layout": v.Layout, "filter": v.Filter, "groupByFields": count(v.Groups), "verticalGroupByFields": count(v.VerticalGroups), "sortByFields": count(v.Sorts), "configuration": map[string]any{"visibleFields": count(v.VisibleFields)}}
}
func projectFakeConn(rows []any, after any) any {
	start := 0
	if s, ok := after.(string); ok && s != "" {
		start, _ = strconv.Atoi(s)
	}
	end := start + 1
	if end > len(rows) {
		end = len(rows)
	}
	next := end < len(rows)
	return map[string]any{"nodes": rows[start:end], "pageInfo": map[string]any{"hasNextPage": next, "endCursor": strconv.Itoa(end)}}
}
func (f *githubProjectFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Query     string
		Variables map[string]any
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		panic("request")
	}
	q := in.Query
	v := in.Variables
	input, _ := v["input"].(map[string]any)
	str := func(k string) string { s, _ := input[k].(string); return s }
	data := map[string]any{}
	rows := []any{}
	mut := ""
	payload := ""
	var obj any
	switch {
	case strings.Contains(q, "query HNProjectUserOwner"):
		data["user"] = map[string]any{"__typename": "User", "id": "U_A", "login": "a"}
	case strings.Contains(q, "query HNUserProjects"):
		if f.p != nil {
			rows = append(rows, projectFakeHeader(*f.p))
		}
		data["user"] = map[string]any{"projectsV2": projectFakeConn(rows, v["after"])}
	case strings.Contains(q, "query HNProjectID"):
		if f.p != nil && v["id"] == f.p.ID {
			data["node"] = projectFakeHeader(*f.p)
		}
	case strings.Contains(q, "query HNUserProject("):
		if f.p != nil {
			data["user"] = map[string]any{"projectV2": projectFakeHeader(*f.p)}
		}
	case strings.Contains(q, "query HNProjectFields"):
		for _, a := range f.p.Fields {
			rows = append(rows, projectFakeField(a))
		}
		data["node"] = map[string]any{"id": f.p.ID, "fields": projectFakeConn(rows, v["after"])}
	case strings.Contains(q, "query HNProjectItems"):
		for _, a := range f.p.Items {
			rows = append(rows, projectFakeItem(a))
		}
		data["node"] = map[string]any{"id": f.p.ID, "items": projectFakeConn(rows, v["after"])}
	case strings.Contains(q, "query HNProjectValues"):
		for _, a := range f.p.Items {
			if a.ID != v["id"] {
				continue
			}
			for j, b := range a.Values {
				var field githubapi.ProjectField
				for _, ff := range f.p.Fields {
					if ff.ID == b.FieldID {
						field = ff
					}
				}
				rows = append(rows, map[string]any{"__typename": b.Type, "id": a.ID + "-V" + strconv.Itoa(j), "field": projectFakeField(field), "text": b.Text, "date": b.Date, "number": b.Number, "optionId": b.OptionID})
			}
			data["node"] = map[string]any{"id": a.ID, "project": map[string]any{"id": a.ProjectID}, "fieldValues": projectFakeConn(rows, v["after"])}
		}
	case strings.Contains(q, "query HNProjectItemHeader"):
		for _, a := range f.p.Items {
			if a.ID == v["id"] {
				data["node"] = projectFakeItem(a)
			}
		}
	case strings.Contains(q, "query HNProjectViews"):
		for _, a := range f.p.Views {
			rows = append(rows, projectFakeView(a))
		}
		data["node"] = map[string]any{"id": f.p.ID, "views": projectFakeConn(rows, v["after"])}
	case strings.Contains(q, "mutation HNCreateProject("):
		mut = "create"
		payload = "createProjectV2"
		f.p = &githubapi.ProjectV2{ID: "PVT_NEW", Number: 2, Title: str("title"), URL: "https://example.test/project/2", CreatedAt: "2026-01-02T03:04:05Z", UpdatedAt: "2026-01-02T03:04:05Z", Owner: githubapi.ProjectOwner{ID: "U_A", Kind: "User", Login: "a"}}
		obj = map[string]any{"projectV2": projectFakeHeader(*f.p)}
	case strings.Contains(q, "mutation HNUpdateProject("):
		mut = "update"
		payload = "updateProjectV2"
		f.p.Title = str("title")
		f.p.Readme = str("readme")
		f.p.Closed, _ = input["closed"].(bool)
		obj = map[string]any{"projectV2": projectFakeHeader(*f.p)}
	case strings.Contains(q, "mutation HNCreateProjectField"):
		mut = "field-create"
		payload = "createProjectV2Field"
		f.serial++
		a := githubapi.ProjectField{ID: fmt.Sprintf("F_%d", f.serial), Name: str("name"), DataType: str("dataType")}
		opts, _ := input["singleSelectOptions"].([]any)
		for j, o := range opts {
			m := o.(map[string]any)
			a.Options = append(a.Options, githubapi.ProjectOption{ID: fmt.Sprintf("O_%d_%d", f.serial, j), Name: m["name"].(string)})
		}
		f.p.Fields = append(f.p.Fields, a)
		obj = map[string]any{"projectV2Field": projectFakeField(a)}
	case strings.Contains(q, "mutation HNUpdateProjectField"):
		mut = "field-update"
		payload = "updateProjectV2Field"
		for j := range f.p.Fields {
			a := &f.p.Fields[j]
			if a.ID == str("fieldId") {
				a.Name = str("name")
				obj = map[string]any{"projectV2Field": projectFakeField(*a)}
			}
		}
	case strings.Contains(q, "mutation HNAddProjectDraft"):
		mut = "item-draft"
		payload = "addProjectV2DraftIssue"
		f.serial++
		a := githubapi.ProjectItem{ID: fmt.Sprintf("I_%d", f.serial), ProjectID: f.p.ID, CreatedAt: "2026-01-02T03:04:05Z", UpdatedAt: "2026-01-02T03:04:05Z", Content: &githubapi.ProjectContent{ID: fmt.Sprintf("D_%d", f.serial), Type: "DraftIssue", Title: str("title"), Body: str("body")}}
		f.p.Items = append(f.p.Items, a)
		obj = map[string]any{"projectItem": projectFakeItem(a)}
	case strings.Contains(q, "mutation HNUpdateProjectDraft"):
		mut = "draft-update"
		payload = "updateProjectV2DraftIssue"
		for j := range f.p.Items {
			a := &f.p.Items[j]
			if a.Content != nil && a.Content.ID == str("draftIssueId") {
				a.Content.Title = str("title")
				a.Content.Body = str("body")
				obj = map[string]any{"draftIssue": map[string]any{"id": a.Content.ID, "title": a.Content.Title, "body": a.Content.Body}}
			}
		}
	case strings.Contains(q, "mutation HNSetProjectValue"):
		mut = "value-set"
		payload = "updateProjectV2ItemFieldValue"
		for j := range f.p.Items {
			a := &f.p.Items[j]
			if a.ID != str("itemId") {
				continue
			}
			value := input["value"].(map[string]any)
			b := githubapi.ProjectValue{FieldID: str("fieldId")}
			for k, val := range value {
				switch k {
				case "text":
					b.Type = "ProjectV2ItemFieldTextValue"
					b.Text = val.(string)
				case "number":
					b.Type = "ProjectV2ItemFieldNumberValue"
					b.Number = json.Number(strconv.FormatFloat(val.(float64), 'f', -1, 64))
				case "date":
					b.Type = "ProjectV2ItemFieldDateValue"
					b.Date = val.(string)
				case "singleSelectOptionId":
					b.Type = "ProjectV2ItemFieldSingleSelectValue"
					b.OptionID = val.(string)
				}
			}
			found := false
			for i := range a.Values {
				if a.Values[i].FieldID == b.FieldID {
					a.Values[i] = b
					found = true
				}
			}
			if !found {
				a.Values = append(a.Values, b)
			}
			obj = map[string]any{"projectV2Item": projectFakeItem(*a)}
		}
	case strings.Contains(q, "mutation HNClearProjectValue"):
		mut = "value-clear"
		payload = "clearProjectV2ItemFieldValue"
		for j := range f.p.Items {
			a := &f.p.Items[j]
			if a.ID != str("itemId") {
				continue
			}
			filtered := []githubapi.ProjectValue{}
			for _, v := range a.Values {
				if v.FieldID != str("fieldId") {
					filtered = append(filtered, v)
				}
			}
			a.Values = filtered
			obj = map[string]any{"projectV2Item": projectFakeItem(*a)}
		}
	case strings.Contains(q, "mutation HNArchiveProjectItem") || strings.Contains(q, "mutation HNUnarchiveProjectItem"):
		mut = "item-archive"
		payload = "archiveProjectV2Item"
		archive := strings.Contains(q, "mutation HNArchive")
		if !archive {
			mut = "item-unarchive"
			payload = "unarchiveProjectV2Item"
		}
		for j := range f.p.Items {
			a := &f.p.Items[j]
			if a.ID == str("itemId") {
				a.Archived = archive
				obj = map[string]any{"item": projectFakeItem(*a)}
			}
		}
	case strings.Contains(q, "mutation HNPositionProjectItem"):
		mut = "item-position"
		payload = "updateProjectV2ItemPosition"
		var selected githubapi.ProjectItem
		filtered := []githubapi.ProjectItem{}
		for _, a := range f.p.Items {
			if a.ID == str("itemId") {
				selected = a
			} else {
				filtered = append(filtered, a)
			}
		}
		index := 0
		for j, a := range filtered {
			if a.ID == str("afterId") {
				index = j + 1
			}
		}
		filtered = append(filtered, githubapi.ProjectItem{})
		copy(filtered[index+1:], filtered[index:])
		filtered[index] = selected
		f.p.Items = filtered
		obj = map[string]any{"clientMutationId": nil}
	case strings.Contains(q, "mutation HNCreateProjectView"):
		mut = "view-create"
		payload = "createProjectV2View"
		f.serial++
		a := githubapi.ProjectView{ID: fmt.Sprintf("V_%d", f.serial), Name: str("name"), Layout: str("layout")}
		f.p.Views = append(f.p.Views, a)
		obj = map[string]any{"projectV2View": projectFakeView(a)}
	case strings.Contains(q, "mutation HNUpdateProjectView"):
		mut = "view-update"
		payload = "updateProjectV2View"
		for j := range f.p.Views {
			a := &f.p.Views[j]
			if a.ID == str("viewId") {
				a.Name = str("name")
				a.Layout = str("layout")
				obj = map[string]any{"projectV2View": projectFakeView(*a)}
			}
		}
	default:
		panic("unhandled query " + q)
	}
	if mut != "" {
		f.mutations++
		if f.lost == mut {
			f.lost = ""
			w.WriteHeader(503)
			return
		}
		data[payload] = obj
	}
	json.NewEncoder(w).Encode(map[string]any{"data": data})
}
func githubProjectServer(t *testing.T, f *githubProjectFake) string {
	t.Helper()
	s := httptest.NewServer(f)
	t.Cleanup(s.Close)
	t.Setenv("HN_GITHUB_TOKEN", "fake-token")
	return s.URL
}
func githubProjectOptions(command, endpoint, selector, id string) transferOptions {
	return transferOptions{command: command, provider: "github", endpoint: endpoint, project: selector, id: id, resource: forge.ResourceProject}
}
func githubProjectCreateNative(t *testing.T) (ResourceMutationResult, ResourceMutationResult) {
	t.Helper()
	p := ProjectState{Schema: "hn.project/1", Title: "Native roadmap", Body: "Plan", Fields: []ProjectField{{ID: "status", Name: "Status", Type: "single_select", Options: []ProjectOption{{ID: "todo", Name: "Todo"}}}, {ID: "size", Name: "Size", Type: "number"}}, Views: []ProjectView{{ID: "table", Name: "Tasks", Layout: "table"}}}
	root, err := MutateResource(ResourceMutation{Family: ResourceProject, Intent: "open", State: resourceState(t, ResourceProject, p)})
	if err != nil {
		t.Fatal(err)
	}
	item, err := MutateResource(ResourceMutation{Family: ResourceProjectItem, Intent: "open", State: resourceState(t, ResourceProjectItem, ProjectItemState{Schema: "hn.project-item/1", ProjectID: root.ResourceID, ProjectRevision: root.EventID, Target: ProjectTarget{Kind: "draft", Title: "Do it", Body: "Exact draft"}, Position: "1", Values: map[string]ProjectValue{"status": {Type: "single_select", OptionID: "todo"}, "size": {Type: "number", Number: "1.25"}}})})
	if err != nil {
		t.Fatal(err)
	}
	return *root, *item
}
func TestGithubProjectPublicImportAndConflict(t *testing.T) {
	issueCommandTestRepo(t)
	p := projectFixture()
	f := &githubProjectFake{p: &p}
	endpoint := githubProjectServer(t, f)
	o := githubProjectOptions("import", endpoint, "user:a/1", "")
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	cat := bridgeCatalog(t)
	if len(cat.IDs) != 2 {
		t.Fatal(cat.IDs)
	}
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if len(bridgeCatalog(t).IDs) != 2 {
		t.Fatal("duplicate")
	}
	if f.mutations != 0 {
		t.Fatal("import wrote provider")
	}
	root := bridgeFamily(bridgeCatalog(t), ResourceProject)[0]
	var state ProjectState
	json.Unmarshal(root.State, &state)
	state.Title = "local"
	if _, err := MutateResource(ResourceMutation{Family: ResourceProject, Intent: "revise", ID: root.ID, Expected: []string{root.Heads[0].ID}, State: resourceState(t, ResourceProject, state)}); err != nil {
		t.Fatal(err)
	}
	f.p.Title = "remote"
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("overwrote competing native edit")
	}
}
func TestGithubProjectPublicExportAndRepeat(t *testing.T) {
	issueCommandTestRepo(t)
	root, _ := githubProjectCreateNative(t)
	f := &githubProjectFake{}
	endpoint := githubProjectServer(t, f)
	o := githubProjectOptions("export", endpoint, "user:a", root.ResourceID)
	o.dryRun = true
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if f.mutations != 0 {
		t.Fatal("preview mutation")
	}
	o.dryRun = false
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	before := f.mutations
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if before != f.mutations {
		t.Fatal("repeat duplicated mutation")
	}
	if len(f.p.Fields) != 2 || len(f.p.Items) != 1 || len(f.p.Views) != 1 {
		t.Fatal(f.p)
	}
	if _, proof, ok := githubParseProjectBody(f.p.Readme); !ok || proof.NativeID != root.ResourceID {
		t.Fatal("missing exact publication proof")
	}
	importOptions := githubProjectOptions("import", endpoint, "user:a/2", "")
	if err := runForgeTransfer(importOptions); err == nil {
		t.Fatal("duplicated own publication")
	}
	if len(bridgeCatalog(t).IDs) != 2 {
		t.Fatal("duplicate native root")
	}
	f.p.Title = "remote edit"
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("overwrote remote edit")
	}
}
func TestGithubProjectLostMutationsReconcile(t *testing.T) {
	for _, action := range []string{"create", "field-create", "item-draft", "value-set", "view-create", "update"} {
		t.Run(action, func(t *testing.T) {
			issueCommandTestRepo(t)
			root, _ := githubProjectCreateNative(t)
			f := &githubProjectFake{lost: action}
			endpoint := githubProjectServer(t, f)
			o := githubProjectOptions("export", endpoint, "user:a", root.ResourceID)
			if err := runForgeTransfer(o); err == nil {
				t.Fatal("lost response success")
			}
			if err := runForgeTransfer(o); err != nil {
				t.Fatal("reconcile", err)
			}
			if len(f.p.Fields) != 2 || len(f.p.Items) != 1 || len(f.p.Views) != 1 {
				t.Fatal("duplicate", f.p)
			}
			before := f.mutations
			if err := runForgeTransfer(o); err != nil {
				t.Fatal(err)
			}
			if before != f.mutations {
				t.Fatal("repeat after recovery wrote")
			}
		})
	}
}

func TestGithubProjectSourceExportLocalEdit(t *testing.T) {
	issueCommandTestRepo(t)
	p := projectFixture()
	f := &githubProjectFake{p: &p}
	endpoint := githubProjectServer(t, f)
	if err := runForgeTransfer(githubProjectOptions("import", endpoint, "user:a/1", "")); err != nil {
		t.Fatal(err)
	}
	root := bridgeFamily(bridgeCatalog(t), ResourceProject)[0]
	var state ProjectState
	json.Unmarshal(root.State, &state)
	state.Title = "Native changed title"
	if _, err := MutateResource(ResourceMutation{Family: ResourceProject, Intent: "revise", ID: root.ID, Expected: []string{root.Heads[0].ID}, State: resourceState(t, ResourceProject, state)}); err != nil {
		t.Fatal(err)
	}
	o := githubProjectOptions("export", endpoint, "user:a/1", root.ID)
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if f.p.Title != state.Title || f.p.Readme != "Plan" || f.p.Items[0].Content.Body != "Work" {
		t.Fatal("source echo")
	}
	before := f.mutations
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if before != f.mutations {
		t.Fatal("duplicate source mutation")
	}
}
func TestGithubProjectCheckpointRemovalRecoversPublishedNative(t *testing.T) {
	issueCommandTestRepo(t)
	root, _ := githubProjectCreateNative(t)
	f := &githubProjectFake{}
	endpoint := githubProjectServer(t, f)
	o := githubProjectOptions("export", endpoint, "user:a", root.ResourceID)
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	scope := githubProjectScope(endpoint, o.project, o.id)
	if err := forgeRemoveCheckpoint(githubProjectCheckpointName(scope)); err != nil {
		t.Fatal(err)
	}
	before := f.mutations
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	if len(f.p.Fields) != 2 || len(f.p.Items) != 1 || len(f.p.Views) != 1 {
		t.Fatal("cloneless duplicates")
	}
	if f.mutations != before {
		t.Log("revalidated existing exact publication through bounded idempotent update steps")
	}
}
func TestGithubProjectCheckpointCannotInjectSteps(t *testing.T) {
	issueCommandTestRepo(t)
	root, _ := githubProjectCreateNative(t)
	f := &githubProjectFake{lost: "create"}
	endpoint := githubProjectServer(t, f)
	o := githubProjectOptions("export", endpoint, "user:a", root.ResourceID)
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("lost success")
	}
	name := githubProjectCheckpointName(githubProjectScope(endpoint, o.project, o.id))
	var cp githubProjectCheckpoint
	if ok, err := forgeLoadJSON(name, &cp); err != nil || !ok {
		t.Fatal(err)
	}
	cp.Steps[len(cp.Steps)-1].Input["title"] = "forged"
	if err := forgeStoreJSON(name, cp); err != nil {
		t.Fatal(err)
	}
	before := f.mutations
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("checkpoint selected non-native mutation")
	}
	if f.mutations != before {
		t.Fatal("mutated after forged step")
	}
}
func TestGithubProjectNegativeRecoveryNeverRetries(t *testing.T) {
	issueCommandTestRepo(t)
	root, _ := githubProjectCreateNative(t)
	f := &githubProjectFake{lost: "create"}
	endpoint := githubProjectServer(t, f)
	o := githubProjectOptions("export", endpoint, "user:a", root.ResourceID)
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("lost success")
	}
	f.p = nil
	before := f.mutations
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("negative scan success")
	}
	if f.mutations != before {
		t.Fatal("retried unknown creation")
	}
}
func TestGithubProjectNumericPrecisionAndBudget(t *testing.T) {
	for _, s := range []string{"NaN", "1e309", "9007199254740993", "0.12345678901234567890123456789"} {
		if _, err := githubProjectNumber(s); err == nil {
			t.Fatal("rounded", s)
		}
	}
	if got, err := githubProjectNumber("1e3"); err != nil || got != "1000" {
		t.Fatal(got, err)
	}
	issueCommandTestRepo(t)
	root, _ := githubProjectCreateNative(t)
	snapshot, err := forgeSnapshotResource(root.ResourceID, forge.ResourceProject)
	if err != nil {
		t.Fatal(err)
	}
	r, err := githubProjectRoot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for i := range snapshot.Graph.Nodes {
		if snapshot.Graph.Nodes[i].Project != nil {
			snapshot.Graph.Nodes[i].Project.Body = strings.Repeat("<", 3<<20)
		}
	}
	cp := githubProjectCheckpoint{Schema: "hn.github-project-export/1", Scope: eventID([]byte("budget")), Endpoint: "https://api.github.com", Selector: "user:a", Snapshot: snapshot, Bindings: map[string]string{"owner": "U"}}
	o := githubProjectOptions("export", cp.Endpoint, cp.Selector, root.ResourceID)
	steps, _, err := githubPlanProject(snapshot, o, cp.Endpoint, nil, cp.Bindings, false)
	if err != nil {
		t.Fatal(err)
	}
	cp.Steps = steps
	if err := githubProjectCheckpointBudget(cp); err == nil {
		t.Fatal("oversize future payload accepted", r.Record.ID)
	}
}

func TestGithubProjectUnchangedSourceExportDoesNotWrite(t *testing.T) {
	issueCommandTestRepo(t)
	p := projectFixture()
	f := &githubProjectFake{p: &p}
	endpoint := githubProjectServer(t, f)
	if err := runForgeTransfer(githubProjectOptions("import", endpoint, "user:a/1", "")); err != nil {
		t.Fatal(err)
	}
	root := bridgeFamily(bridgeCatalog(t), ResourceProject)[0]
	if err := runForgeTransfer(githubProjectOptions("export", endpoint, "node:PVT_A", root.ID)); err != nil {
		t.Fatal(err)
	}
	if f.mutations != 0 {
		t.Fatal("unchanged imported project wrote provider", f.mutations)
	}
}
func TestGithubProjectFutureBindingBudgetRefusesBeforeWrites(t *testing.T) {
	issueCommandTestRepo(t)
	p := ProjectState{Schema: "hn.project/1", Title: "Large schema"}
	for i := 0; i < 32; i++ {
		f := ProjectField{ID: fmt.Sprintf("field%d", i), Name: fmt.Sprintf("Field%d", i), Type: "single_select"}
		for j := 0; j < 64; j++ {
			f.Options = append(f.Options, ProjectOption{ID: fmt.Sprintf("opt%d", j), Name: fmt.Sprintf("Option%d", j)})
		}
		p.Fields = append(p.Fields, f)
	}
	opened, err := MutateResource(ResourceMutation{Family: ResourceProject, Intent: "open", State: resourceState(t, ResourceProject, p)})
	if err != nil {
		t.Fatal(err)
	}
	fake := &githubProjectFake{}
	endpoint := githubProjectServer(t, fake)
	o := githubProjectOptions("export", endpoint, "user:a", opened.ResourceID)
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("predictably oversize journal accepted")
	}
	if fake.mutations != 0 {
		t.Fatal("budget failed after first remote mutation")
	}
}
func TestGithubProjectUnsupportedAndMissingMappingZeroWrites(t *testing.T) {
	issueCommandTestRepo(t)
	p := projectFixture()
	p.Workflows = 1
	fake := &githubProjectFake{p: &p}
	endpoint := githubProjectServer(t, fake)
	if err := runForgeTransfer(githubProjectOptions("import", endpoint, "user:a/1", "")); err == nil {
		t.Fatal("silently omitted automation")
	}
	if len(bridgeCatalog(t).IDs) != 0 || fake.mutations != 0 {
		t.Fatal("wrote blocked plan")
	}
	root, item := githubProjectCreateNative(t)
	var state ProjectItemState
	v := bridgeCatalog(t).Resources[item.ResourceID]
	json.Unmarshal(v.State, &state)
	state.Target = ProjectTarget{Kind: "external", URL: "https://example.test/mapped"}
	if _, err := MutateResource(ResourceMutation{Family: ResourceProjectItem, Intent: "revise", ID: item.ResourceID, Expected: []string{item.EventID}, State: resourceState(t, ResourceProjectItem, state)}); err != nil {
		t.Fatal(err)
	}
	fake.p = nil
	if err := runForgeTransfer(githubProjectOptions("export", endpoint, "user:a", root.ResourceID)); err == nil {
		t.Fatal("guessed external target")
	}
	if fake.mutations != 0 {
		t.Fatal("missing target detected after mutation")
	}
}

func TestGithubProjectSignedJournalRejectsForgedProgressAndBindings(t *testing.T) {
	for _, attack := range []string{"progress", "binding", "pending", "endpoint", "signer"} {
		t.Run(attack, func(t *testing.T) {
			issueCommandTestRepo(t)
			root, _ := githubProjectCreateNative(t)
			fake := &githubProjectFake{lost: "item-draft"}
			endpoint := githubProjectServer(t, fake)
			o := githubProjectOptions("export", endpoint, "user:a", root.ResourceID)
			if err := runForgeTransfer(o); err == nil {
				t.Fatal("expected interrupted draft")
			}
			name := githubProjectCheckpointName(githubProjectScope(endpoint, o.project, o.id))
			var cp githubProjectCheckpoint
			if ok, err := forgeLoadJSON(name, &cp); err != nil || !ok {
				t.Fatal(err)
			}
			switch attack {
			case "progress":
				cp.Next = len(cp.Steps) - 1
				cp.Pending = nil
			case "binding":
				cp.Bindings["field:status"] = "HUMAN_FIELD"
			case "pending":
				cp.Pending.ResultID = "HUMAN_ITEM"
			case "endpoint":
				cp.Endpoint = "https://different.example"
			case "signer":
				cp.JournalPublicKey = testIdentity(t, "forged").PublicKey
			}
			if err := forgeStoreJSON(name, cp); err != nil {
				t.Fatal(err)
			}
			before := fake.mutations
			if err := runForgeTransfer(o); err == nil {
				t.Fatal("forged journal was accepted")
			}
			if fake.mutations != before {
				t.Fatal("forged progress authorized mutation")
			}
		})
	}
}
func TestGithubProjectIdentityChangeAndKeyFreePreview(t *testing.T) {
	issueCommandTestRepo(t)
	root, _ := githubProjectCreateNative(t)
	fake := &githubProjectFake{lost: "create"}
	endpoint := githubProjectServer(t, fake)
	o := githubProjectOptions("export", endpoint, "user:a", root.ResourceID)
	if err := runForgeTransfer(o); err == nil {
		t.Fatal("expected interrupted create")
	}
	before := fake.mutations
	writeActiveTestIdentity(t, testIdentity(t, "Different signer"))
	if err := runForgeTransfer(o); err == nil || !strings.Contains(err.Error(), "different native identity") {
		t.Fatal(err)
	}
	if fake.mutations != before {
		t.Fatal("identity change wrote")
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(paths.active); err != nil {
		t.Fatal(err)
	}
	o.dryRun = true
	if err := runForgeTransfer(o); err != nil {
		t.Fatal("key-free saved preview", err)
	}
	if fake.mutations != before {
		t.Fatal("preview wrote")
	}
}
func TestGithubProjectExportSubsequentNativeUpdate(t *testing.T) {
	issueCommandTestRepo(t)
	root, item := githubProjectCreateNative(t)
	fake := &githubProjectFake{}
	endpoint := githubProjectServer(t, fake)
	o := githubProjectOptions("export", endpoint, "user:a", root.ResourceID)
	if err := runForgeTransfer(o); err != nil {
		t.Fatal(err)
	}
	var state ProjectItemState
	v := bridgeCatalog(t).Resources[item.ResourceID]
	json.Unmarshal(v.State, &state)
	state.Values["size"] = ProjectValue{Type: "number", Number: "2.5"}
	if _, err := MutateResource(ResourceMutation{Family: ResourceProjectItem, Intent: "revise", ID: item.ResourceID, Expected: []string{item.EventID}, State: resourceState(t, ResourceProjectItem, state)}); err != nil {
		t.Fatal(err)
	}
	if err := runForgeTransfer(o); err != nil {
		t.Fatal("second native version", err)
	}
	if len(fake.p.Items) != 1 {
		t.Fatal("duplicated item")
	}
	found := false
	for _, val := range fake.p.Items[0].Values {
		if val.Type == "ProjectV2ItemFieldNumberValue" && val.Number == "2.5" {
			found = true
		}
	}
	if !found {
		t.Fatal("did not update value")
	}
}

func TestGithubProjectBadDateAndOptionImportZeroFacts(t *testing.T) {
	for _, kind := range []string{"date", "option"} {
		t.Run(kind, func(t *testing.T) {
			issueCommandTestRepo(t)
			p := projectFixture()
			if kind == "date" {
				p.Fields = append(p.Fields, githubapi.ProjectField{ID: "F_d", Name: "Date", DataType: "DATE"})
				p.Items[0].Values = append(p.Items[0].Values, githubapi.ProjectValue{Type: "ProjectV2ItemFieldDateValue", FieldID: "F_d", Date: "2026-02-29"})
			} else {
				p.Items[0].Values[1].OptionID = "MISSING"
			}
			fake := &githubProjectFake{p: &p}
			endpoint := githubProjectServer(t, fake)
			if err := runForgeTransfer(githubProjectOptions("import", endpoint, "user:a/1", "")); err == nil {
				t.Fatal("accepted invalid typed value")
			}
			if len(bridgeCatalog(t).IDs) != 0 {
				t.Fatal("partially imported invalid project")
			}
			if fake.mutations != 0 {
				t.Fatal("wrote provider")
			}
		})
	}
}
func TestGithubProjectUnrepresentableNativeNumberZeroWrites(t *testing.T) {
	issueCommandTestRepo(t)
	root, item := githubProjectCreateNative(t)
	v := bridgeCatalog(t).Resources[item.ResourceID]
	var state ProjectItemState
	json.Unmarshal(v.State, &state)
	state.Values["size"] = ProjectValue{Type: "number", Number: "9007199254740993"}
	if _, err := MutateResource(ResourceMutation{Family: ResourceProjectItem, Intent: "revise", ID: item.ResourceID, Expected: []string{item.EventID}, State: resourceState(t, ResourceProjectItem, state)}); err != nil {
		t.Fatal(err)
	}
	fake := &githubProjectFake{}
	endpoint := githubProjectServer(t, fake)
	if err := runForgeTransfer(githubProjectOptions("export", endpoint, "user:a", root.ResourceID)); err == nil {
		t.Fatal("rounded native value")
	}
	if fake.mutations != 0 {
		t.Fatal("precision failed after write")
	}
}
