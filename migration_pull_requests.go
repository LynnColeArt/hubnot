package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"time"

	"hubnot/internal/migration"
)

type importedPullRequest struct {
	Source   migration.Key `json:"source"`
	Revision string        `json:"revision"`
	Number   int           `json:"number"`
	Title    string        `json:"title"`
	State    string        `json:"state"`
	Author   string        `json:"author"`
	URL      string        `json:"url"`
	Created  string        `json:"created_at"`
	Updated  string        `json:"updated_at"`
}

func listImportedPullRequests(ctx context.Context, gitDir string) ([]importedPullRequest, error) {
	limits := migration.DefaultLimits()
	imports, err := ListLocalMigrations(gitDir, limits)
	if err != nil {
		return nil, err
	}
	latest := map[migration.Key]importedPullRequest{}
	for _, id := range imports {
		archive, err := ReadLocalMigration(ctx, gitDir, id, limits)
		if err != nil {
			return nil, err
		}
		err = collectImportedPullRequests(ctx, archive, latest)
		archive.Close()
		if err != nil {
			return nil, err
		}
	}
	rows := make([]importedPullRequest, 0, len(latest))
	for _, row := range latest {
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

func collectImportedPullRequests(ctx context.Context, archive *migration.Archive, latest map[migration.Key]importedPullRequest) error {
	for _, record := range archive.Snapshot.Records {
		if record.Key.Kind != "change_request" || record.Key.Provider != "github" {
			continue
		}
		pr, err := importedGitHubPullRequest(ctx, archive, record)
		if err != nil {
			return err
		}
		old, exists := latest[record.Key]
		if !exists || pr.Updated > old.Updated || pr.Updated == old.Updated && pr.Revision > old.Revision {
			latest[record.Key] = pr
		}
	}
	return nil
}

func importedGitHubPullRequest(ctx context.Context, archive *migration.Archive, record migration.Record) (importedPullRequest, error) {
	var metadata migration.PreservedContent
	if err := json.Unmarshal(record.Content, &metadata); err != nil {
		return importedPullRequest{}, err
	}
	if metadata.Schema != "hn.migration.preserved/1" {
		return importedPullRequest{}, fmt.Errorf("unsupported preserved pull request schema")
	}
	reader, err := archive.Open(ctx, metadata.SourceBlob)
	if err != nil {
		return importedPullRequest{}, err
	}
	defer reader.Close()
	const maxPRBytes = 4 << 20
	raw, err := io.ReadAll(io.LimitReader(reader, maxPRBytes+1))
	if err != nil || len(raw) > maxPRBytes {
		return importedPullRequest{}, fmt.Errorf("cannot read preserved pull request")
	}
	var source struct {
		Number   int    `json:"number"`
		Title    string `json:"title"`
		State    string `json:"state"`
		MergedAt string `json:"merged_at"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return importedPullRequest{}, err
	}
	number, err := strconv.Atoi(record.Selector)
	if err != nil || number <= 0 || number != source.Number || (source.State != "open" && source.State != "closed") {
		return importedPullRequest{}, fmt.Errorf("invalid preserved pull request identity or state")
	}
	if source.State == "closed" && source.MergedAt != "" {
		source.State = "merged"
	}
	link, err := url.Parse(record.SourceURL)
	if err != nil || link.Scheme != "https" || link.Hostname() == "" || link.User != nil {
		return importedPullRequest{}, fmt.Errorf("invalid preserved pull request link")
	}
	normalize := func(value string) string {
		date, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return ""
		}
		return date.UTC().Format("2006-01-02T15:04:05.000000000Z")
	}
	return importedPullRequest{
		Source: record.Key, Revision: record.Revision, Number: number, Title: source.Title, State: source.State,
		Author: record.Author, URL: record.SourceURL, Created: normalize(record.CreatedAt), Updated: normalize(record.UpdatedAt),
	}, nil
}
