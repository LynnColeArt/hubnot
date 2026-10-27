package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
)

// executeAudit uses the ordinary isolated snapshot runner with separately
// authorized definition bytes. Neither the target checkout nor its policy
// selects those bytes or grants execution authority.
func executeAudit(ctx context.Context, request, policyRef string, id *Identity, backend executionBackend) (*StoredEvent, error) {
	selected, err := ValidateAuditExecution(request, id.Actor, policyRef)
	if err != nil {
		return nil, err
	}
	checked := &auditExecutionBackend{executionBackend: backend}
	result, err := executePipeline(ctx, selected.Request.Event.Audit.CodeCommit, selected.Request.Event.Audit.Rule, selected.Pipeline, checked)
	if err != nil {
		return nil, err
	}
	if checked.transport != nil {
		return nil, fmt.Errorf("audit runner transport failed: %w", checked.transport)
	}
	// The shared runner adds a truncation notice beyond its cap. Keep the same
	// notice inside the audit protocol's strict 1 MiB attachment budget.
	if len(result.Log) > maxRunLogSize {
		notice := []byte("\n[log truncated by Hubnot]\n")
		result.Log = append(result.Log[:maxRunLogSize-len(notice)], notice...)
	}
	findings, err := auditFindings(result)
	if err != nil {
		return nil, err
	}
	// RecordAuditResult independently re-resolves current anchored policy, roles
	// and rule heads immediately before publication, including mid-run withdrawal.
	return RecordAuditResult(request, policyRef, id, result, findings, backend.Name(), runtime.GOOS+"/"+runtime.GOARCH, "hn/"+version)
}

type auditExecutionBackend struct {
	executionBackend
	transport error
}

func (b *auditExecutionBackend) RunStep(ctx context.Context, root string, step PipelineStep, environment []string, output io.Writer) error {
	err := b.executionBackend.RunStep(ctx, root, step, environment, output)
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) && ctx.Err() == nil {
		b.transport = err
	}
	return err
}

func normalizeAuditFindingKey(key string) string { return strings.ToLower(strings.TrimSpace(key)) }

// Structured records are line-delimited inert JSON emitted on stdout/stderr.
// Parse only the capped authenticated log, never files supplied by the target.
func auditFindings(result executionResult) ([]AuditFinding, error) {
	findings := []AuditFinding{}
	seen := map[string]bool{}
	const marker = "HN_AUDIT_FINDING "
	for _, line := range bytes.Split(result.Log, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte(marker)) {
			continue
		}
		if len(findings) == 128 {
			return nil, fmt.Errorf("audit output exceeds 128 findings")
		}
		var finding AuditFinding
		if err := strictResourceJSON(line[len(marker):], &finding, 32<<10); err != nil {
			return nil, fmt.Errorf("invalid structured audit finding: %w", err)
		}
		finding.Key = normalizeAuditFindingKey(finding.Key)
		if !issueIdentifier(finding.Key, 128) || seen[finding.Key] || !issueText(finding.Title, 512, true) || !issueText(finding.Body, 16<<10, false) || (finding.Path != "" && (!safeRelativePath(finding.Path) || len(finding.Path) > 1024)) || finding.Line < 0 || (finding.Line > 0 && finding.Path == "") {
			return nil, fmt.Errorf("invalid or duplicate structured audit finding")
		}
		seen[finding.Key] = true
		findings = append(findings, finding)
	}
	if len(findings) == 0 && result.Outcome == "failed" {
		findings = append(findings, AuditFinding{Key: "rule-failure", Title: "Audit rule failed", Body: "The audit pipeline failed without structured findings. Inspect the authenticated result log for details."})
	}
	return findings, nil
}
