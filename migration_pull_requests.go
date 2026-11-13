package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"hubnot/internal/migration"
)

type importedPullRequest struct {
	Source   migration.Key    `json:"source"`
	Revision string           `json:"revision"`
	Number   int              `json:"number"`
	Body     string           `json:"body"`
	Title    string           `json:"title"`
	State    string           `json:"state"`
	Author   string           `json:"author"`
	URL      string           `json:"url"`
	Created  string           `json:"created_at"`
	Base     string           `json:"base_commit,omitempty"`
	Head     string           `json:"head_commit,omitempty"`
	Merge    string           `json:"merge_commit,omitempty"`
	Updated  string           `json:"updated_at"`
	Reviews  []importedReview `json:"reviews,omitempty"`
}

type importedReview struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Author string `json:"author"`
	Body   string `json:"body"`
	State  string `json:"state,omitempty"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Date   string `json:"date,omitempty"`
	Commit string `json:"commit,omitempty"`
}

func listImportedPullRequests(ctx context.Context, gitDir string) ([]importedPullRequest, error) {
	limits := migration.DefaultLimits()
	imports, err := ListLocalMigrations(gitDir, limits)
	if err != nil {
		return nil, err
	}
	latest := map[migration.Key]importedPullRequest{}
	reviews := map[migration.Key]map[string]importedReview{}
	for _, id := range imports {
		archive, err := ReadLocalMigration(ctx, gitDir, id, limits)
		if err != nil {
			return nil, err
		}
		err = collectImportedPullRequests(ctx, archive, latest, reviews)
		archive.Close()
		if err != nil {
			return nil, err
		}
	}
	rows := make([]importedPullRequest, 0, len(latest))
	for _, row := range latest {
		for _, review := range reviews[row.Source] {
			row.Reviews = append(row.Reviews, review)
		}
		sort.Slice(row.Reviews, func(i, j int) bool {
			a, b := row.Reviews[i], row.Reviews[j]
			if a.Date != b.Date {
				return a.Date < b.Date
			}
			return a.ID < b.ID
		})
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Created != b.Created {
			return a.Created > b.Created
		}
		return fmt.Sprint(a.Source) < fmt.Sprint(b.Source)
	})
	return rows, nil
}

func collectImportedPullRequests(ctx context.Context, archive *migration.Archive, latest map[migration.Key]importedPullRequest, reviews map[migration.Key]map[string]importedReview) error {
	for _, record := range archive.Snapshot.Records {
		switch record.Key.Kind {
		case "change_request":
			pr, err := importedPullRequestRecord(ctx, archive, record)
			if err != nil {
				return err
			}
			if pr.Number == 0 {
				continue
			}
			old, exists := latest[record.Key]
			if !exists || pr.Updated > old.Updated || pr.Updated == old.Updated && pr.Revision > old.Revision {
				latest[record.Key] = pr
			}
		case "review", "inline_comment", "reply":
			for _, link := range record.Links {
				if link.Relation != "change_request" {
					continue
				}
				raw, err := importedRecordBytes(ctx, archive, record)
				if err != nil {
					return err
				}
				var data struct {
					Body      string `json:"body"`
					State     string `json:"state"`
					Path      string `json:"path"`
					Line      int    `json:"line"`
					Commit    string `json:"commit_id"`
					Submitted string `json:"submitted_at"`
					Note      string `json:"note"`
				}
				if err := json.Unmarshal(raw, &data); err != nil {
					return err
				}
				if data.Body == "" {
					data.Body = data.Note
				}
				if data.Submitted == "" {
					data.Submitted = record.CreatedAt
				}
				if reviews[link.Target] == nil {
					reviews[link.Target] = map[string]importedReview{}
				}
				id := record.Key.Kind + ":" + record.Key.ID
				reviews[link.Target][id] = importedReview{id, record.Key.Kind, record.Author, data.Body, data.State, data.Path, data.Line, data.Submitted, data.Commit}
			}
		}
	}
	return nil
}

func importedRecordBytes(ctx context.Context, blobs migration.BlobSource, record migration.Record) ([]byte, error) {
	var metadata migration.PreservedContent
	if err := json.Unmarshal(record.Content, &metadata); err != nil {
		return nil, err
	}
	if metadata.Schema != "hn.migration.preserved/1" {
		return nil, fmt.Errorf("unsupported preserved record schema")
	}
	reader, err := blobs.Open(ctx, metadata.SourceBlob)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	const maxPRBytes = 4 << 20
	raw, err := io.ReadAll(io.LimitReader(reader, maxPRBytes+1))
	if err != nil || len(raw) > maxPRBytes {
		return nil, fmt.Errorf("cannot read preserved pull request")
	}
	return raw, nil
}

func importedPullRequestRecord(ctx context.Context, blobs migration.BlobSource, record migration.Record) (importedPullRequest, error) {
	raw, err := importedRecordBytes(ctx, blobs, record)
	if err != nil {
		return importedPullRequest{}, err
	}
	var source struct {
		Number      int    `json:"number"`
		IID         int    `json:"iid"`
		PRID        int    `json:"pullRequestId"`
		ID          int    `json:"id"`
		Description string `json:"description"`
		Status      string `json:"status"`
		URL         string `json:"web_url"`
		SHA         string `json:"sha"`
		DiffRefs    struct {
			Base string `json:"base_sha"`
			Head string `json:"head_sha"`
		} `json:"diff_refs"`
		Source struct {
			Commit struct {
				Hash string `json:"hash"`
			} `json:"commit"`
		} `json:"source"`
		Destination struct {
			Commit struct {
				Hash string `json:"hash"`
			} `json:"commit"`
		} `json:"destination"`
		LastSource struct {
			Commit string `json:"commitId"`
		} `json:"lastMergeSourceCommit"`
		LastTarget struct {
			Commit string `json:"commitId"`
		} `json:"lastMergeTargetCommit"`
		LastMerge struct {
			Commit string `json:"commitId"`
		} `json:"lastMergeCommit"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		State    string `json:"state"`
		MergedAt string `json:"merged_at"`
		Merge    string `json:"merge_commit_sha"`
		Base     struct {
			SHA string `json:"sha"`
		} `json:"base"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return importedPullRequest{}, err
	}
	if source.Number == 0 {
		source.Number = source.IID
	}
	if source.Number == 0 {
		source.Number = source.PRID
	}
	if source.Number == 0 && record.Key.Provider == "bitbucket" {
		source.Number = source.ID
	}
	if source.Number == 0 {
		source.Number, _ = strconv.Atoi(record.Selector)
	}
	if source.Number <= 0 {
		return importedPullRequest{}, nil
	}
	if source.Body == "" {
		source.Body = source.Description
	}
	if source.State == "" {
		source.State = source.Status
	}
	switch strings.ToLower(source.State) {
	case "open", "opened", "active":
		source.State = "open"
	case "merged", "completed":
		source.State = "merged"
	case "closed", "declined", "abandoned", "superseded":
		source.State = "closed"
	default:
		return importedPullRequest{}, nil
	}
	if source.State == "closed" && source.MergedAt != "" {
		source.State = "merged"
	}
	if source.Base.SHA == "" {
		source.Base.SHA = firstPRValue(source.DiffRefs.Base, source.Destination.Commit.Hash, source.LastTarget.Commit)
	}
	if source.Head.SHA == "" {
		source.Head.SHA = firstPRValue(source.DiffRefs.Head, source.SHA, source.Source.Commit.Hash, source.LastSource.Commit)
	}
	if source.Merge == "" {
		source.Merge = source.LastMerge.Commit
	}
	link, err := url.Parse(firstPRValue(record.SourceURL, source.URL))
	sourceURL := ""
	if err == nil && link.Scheme == "https" && link.Hostname() != "" && link.User == nil {
		sourceURL = link.String()
	}
	normalize := func(value string) string {
		date, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return ""
		}
		return date.UTC().Format("2006-01-02T15:04:05.000000000Z")
	}
	return importedPullRequest{
		Source: record.Key, Revision: record.Revision, Number: source.Number, Title: source.Title, Body: source.Body, State: source.State,
		Base: source.Base.SHA, Head: source.Head.SHA, Merge: source.Merge,
		Author: record.Author, URL: sourceURL, Created: normalize(record.CreatedAt), Updated: normalize(record.UpdatedAt),
	}, nil
}

func firstPRValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
