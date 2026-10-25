//go:build !nogithub

package githubapi

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Discussion reads expose present-day snapshots, never complete edit/deletion
// history. Empty Author/AnswerChosenBy means absent/deleted source attribution.
type DiscussionRepository struct {
	ID, NameWithOwner     string
	HasDiscussionsEnabled bool
}
type DiscussionCategory struct {
	ID, Name, Slug, Description, Emoji string
	IsAnswerable                       bool
}
type Discussion struct {
	ID, RepositoryID                                                                string
	Number                                                                          int
	Title, Body, URL, Author, CreatedAt, UpdatedAt, LastEditedAt                    string
	Category                                                                        DiscussionCategory
	Closed, Locked                                                                  bool
	ActiveLockReason, StateReason, AnswerID, AnswerChosenAt, AnswerChosenBy, PollID string
	ReactionCount, UpvoteCount                                                      int
	EditCount                                                                       *int                // nil means edit-history information is inaccessible, not zero.
	Comments                                                                        []DiscussionComment // full flat thread only on GetDiscussion/ByID.
}
type DiscussionComment struct {
	ID, DiscussionID, ParentID, Body, URL, Author, CreatedAt, UpdatedAt, LastEditedAt, DeletedAt string
	IsAnswer, IsMinimized                                                                        bool
	MinimizedReason                                                                              string
	ReactionCount, UpvoteCount                                                                   int
	EditCount                                                                                    *int
}
type DiscussionInput struct{ Title, Body, CategoryID string }

type graphQLRef struct {
	ID string `json:"id"`
}
type graphQLActor struct {
	Login string `json:"login"`
}
type graphQLCount struct {
	TotalCount *int `json:"totalCount"`
}
type discussionCategoryNode struct {
	ID, Name, Slug, Description, Emoji string
	IsAnswerable                       *bool
}
type discussionNode struct {
	Type                                    string `json:"__typename"`
	ID                                      string
	Number                                  *int
	Title, Body                             *string
	URL, CreatedAt, UpdatedAt, LastEditedAt string
	Author                                  *graphQLActor
	Repository                              *graphQLRef
	Category                                *discussionCategoryNode
	Closed, Locked                          *bool
	ActiveLockReason, StateReason           string
	Answer                                  *graphQLRef
	AnswerChosenAt                          string
	AnswerChosenBy                          *graphQLActor
	Poll                                    *graphQLRef
	Reactions                               *graphQLCount
	UpvoteCount                             *int
	UserContentEdits                        *graphQLCount
}
type discussionCommentNode struct {
	Type                                                                string `json:"__typename"`
	ID                                                                  string
	Discussion, ReplyTo                                                 *graphQLRef
	Body                                                                *string
	URL, CreatedAt, UpdatedAt, LastEditedAt, DeletedAt, MinimizedReason string
	Author                                                              *graphQLActor
	IsAnswer, IsMinimized                                               *bool
	Reactions, UserContentEdits                                         *graphQLCount
	UpvoteCount                                                         *int
}

const discussionCategoryFields = `id name slug description emoji isAnswerable`
const discussionFields = `__typename id number title body url author { login } createdAt updatedAt lastEditedAt
 repository { id } category { ` + discussionCategoryFields + ` } closed locked activeLockReason stateReason
 answer { id } answerChosenAt answerChosenBy { login } poll { id }
 reactions(first:1) { totalCount } upvoteCount userContentEdits(first:1) { totalCount }`
const discussionCommentFields = `__typename id body url author { login } createdAt updatedAt lastEditedAt deletedAt
 discussion { id } replyTo { id } isAnswer isMinimized minimizedReason
 reactions(first:1) { totalCount } upvoteCount userContentEdits(first:1) { totalCount }`
const graphQLPageFields = `pageInfo { hasNextPage endCursor }`
const discussionRepositoryQuery = `query HNDiscussionRepository($owner:String!,$name:String!){repository(owner:$owner,name:$name){id nameWithOwner hasDiscussionsEnabled}}`
const discussionCategoriesQuery = `query HNDiscussionCategories($owner:String!,$name:String!,$after:String){repository(owner:$owner,name:$name){id discussionCategories(first:100,after:$after){nodes{` + discussionCategoryFields + `} ` + graphQLPageFields + `}}}`
const discussionsQuery = `query HNDiscussions($owner:String!,$name:String!,$after:String){repository(owner:$owner,name:$name){id discussions(first:100,after:$after){nodes{` + discussionFields + `} ` + graphQLPageFields + `}}}`
const discussionNumberQuery = `query HNDiscussionNumber($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){discussion(number:$number){` + discussionFields + `}}}`
const discussionIDQuery = `query HNDiscussionID($id:ID!){node(id:$id){__typename ... on Discussion {` + discussionFields + `}}}`
const discussionCommentsQuery = `query HNDiscussionComments($id:ID!,$after:String){node(id:$id){__typename ... on Discussion{id comments(first:100,after:$after){nodes{` + discussionCommentFields + `} ` + graphQLPageFields + `}}}}`
const discussionRepliesQuery = `query HNDiscussionReplies($id:ID!,$after:String){node(id:$id){__typename ... on DiscussionComment{id replies(first:100,after:$after){nodes{` + discussionCommentFields + `} ` + graphQLPageFields + `}}}}`
const createDiscussionMutation = `mutation HNCreateDiscussion($input:CreateDiscussionInput!){createDiscussion(input:$input){discussion{` + discussionFields + `}}}`
const updateDiscussionMutation = `mutation HNUpdateDiscussion($input:UpdateDiscussionInput!){updateDiscussion(input:$input){discussion{` + discussionFields + `}}}`
const addDiscussionCommentMutation = `mutation HNAddDiscussionComment($input:AddDiscussionCommentInput!){addDiscussionComment(input:$input){comment{` + discussionCommentFields + `}}}`
const updateDiscussionCommentMutation = `mutation HNUpdateDiscussionComment($input:UpdateDiscussionCommentInput!){updateDiscussionComment(input:$input){comment{` + discussionCommentFields + `}}}`
const markDiscussionAnswerMutation = `mutation HNMarkDiscussionAnswer($input:MarkDiscussionCommentAsAnswerInput!){markDiscussionCommentAsAnswer(input:$input){discussion{` + discussionFields + `}}}`
const unmarkDiscussionAnswerMutation = `mutation HNUnmarkDiscussionAnswer($input:UnmarkDiscussionCommentAsAnswerInput!){unmarkDiscussionCommentAsAnswer(input:$input){discussion{` + discussionFields + `}}}`
const closeDiscussionMutation = `mutation HNCloseDiscussion($input:CloseDiscussionInput!){closeDiscussion(input:$input){discussion{` + discussionFields + `}}}`
const reopenDiscussionMutation = `mutation HNReopenDiscussion($input:ReopenDiscussionInput!){reopenDiscussion(input:$input){discussion{` + discussionFields + `}}}`

func discussionError() error {
	return errors.New("invalid or inaccessible GitHub discussion data; no complete snapshot available")
}
func discussionRepoVariables(repo string) (map[string]any, error) {
	if !validRepo(repo) {
		return nil, errors.New("GitHub repository must be OWNER/NAME")
	}
	p := strings.Split(repo, "/")
	return map[string]any{"owner": p[0], "name": p[1]}, nil
}
func validSourceTime(s string) bool   { _, err := time.Parse(time.RFC3339Nano, s); return err == nil }
func validActor(a *graphQLActor) bool { return a == nil || (a.Login != "" && len(a.Login) <= 1024) }
func actorLogin(a *graphQLActor) string {
	if a == nil {
		return ""
	}
	return a.Login
}
func refID(r *graphQLRef) string {
	if r == nil {
		return ""
	}
	return r.ID
}
func countValue(c *graphQLCount) *int {
	if c == nil {
		return nil
	}
	return c.TotalCount
}
func validCount(c *graphQLCount) bool         { return c != nil && c.TotalCount != nil && *c.TotalCount >= 0 }
func validOptionalCount(c *graphQLCount) bool { return c == nil || validCount(c) }
func validCategory(n *discussionCategoryNode) bool {
	return n != nil && validGraphQLID(n.ID) && n.Name != "" && n.Slug != "" && n.IsAnswerable != nil
}
func (n discussionCategoryNode) value() DiscussionCategory {
	return DiscussionCategory{n.ID, n.Name, n.Slug, n.Description, n.Emoji, *n.IsAnswerable}
}
func validDiscussion(n *discussionNode) bool {
	if n == nil || n.Type != "Discussion" || !validGraphQLID(n.ID) || n.Number == nil || *n.Number <= 0 || n.Title == nil || strings.TrimSpace(*n.Title) == "" || n.Body == nil || n.URL == "" || !validSourceTime(n.CreatedAt) || !validSourceTime(n.UpdatedAt) || !validActor(n.Author) || !validActor(n.AnswerChosenBy) || n.Repository == nil || !validGraphQLID(n.Repository.ID) || !validCategory(n.Category) || n.Closed == nil || n.Locked == nil || !validCount(n.Reactions) || n.UpvoteCount == nil || *n.UpvoteCount < 0 || !validOptionalCount(n.UserContentEdits) {
		return false
	}
	for _, ref := range []*graphQLRef{n.Answer, n.Poll} {
		if ref != nil && !validGraphQLID(ref.ID) {
			return false
		}
	}
	switch n.StateReason {
	case "", "DUPLICATE", "OUTDATED", "REOPENED", "RESOLVED":
	default:
		return false
	}
	switch n.ActiveLockReason {
	case "", "OFF_TOPIC", "RESOLVED", "SPAM", "TOO_HEATED":
	default:
		return false
	}
	return (n.LastEditedAt == "" || validSourceTime(n.LastEditedAt)) && (n.AnswerChosenAt == "" || validSourceTime(n.AnswerChosenAt))
}
func (n discussionNode) value() Discussion {
	return Discussion{ID: n.ID, RepositoryID: n.Repository.ID, Number: *n.Number, Title: *n.Title, Body: *n.Body, URL: n.URL, Author: actorLogin(n.Author), CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, LastEditedAt: n.LastEditedAt, Category: n.Category.value(), Closed: *n.Closed, Locked: *n.Locked, ActiveLockReason: n.ActiveLockReason, StateReason: n.StateReason, AnswerID: refID(n.Answer), AnswerChosenAt: n.AnswerChosenAt, AnswerChosenBy: actorLogin(n.AnswerChosenBy), PollID: refID(n.Poll), ReactionCount: *n.Reactions.TotalCount, UpvoteCount: *n.UpvoteCount, EditCount: countValue(n.UserContentEdits)}
}
func validDiscussionComment(n *discussionCommentNode) bool {
	return n != nil && n.Type == "DiscussionComment" && validGraphQLID(n.ID) && n.Discussion != nil && validGraphQLID(n.Discussion.ID) && (n.ReplyTo == nil || validGraphQLID(n.ReplyTo.ID)) && n.Body != nil && n.URL != "" && validSourceTime(n.CreatedAt) && validSourceTime(n.UpdatedAt) && (n.LastEditedAt == "" || validSourceTime(n.LastEditedAt)) && (n.DeletedAt == "" || validSourceTime(n.DeletedAt)) && validActor(n.Author) && n.IsAnswer != nil && n.IsMinimized != nil && validCount(n.Reactions) && n.UpvoteCount != nil && *n.UpvoteCount >= 0 && validOptionalCount(n.UserContentEdits)
}
func (n discussionCommentNode) value() DiscussionComment {
	return DiscussionComment{ID: n.ID, DiscussionID: n.Discussion.ID, ParentID: refID(n.ReplyTo), Body: *n.Body, URL: n.URL, Author: actorLogin(n.Author), CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, LastEditedAt: n.LastEditedAt, DeletedAt: n.DeletedAt, IsAnswer: *n.IsAnswer, IsMinimized: *n.IsMinimized, MinimizedReason: n.MinimizedReason, ReactionCount: *n.Reactions.TotalCount, UpvoteCount: *n.UpvoteCount, EditCount: countValue(n.UserContentEdits)}
}

func (c *Client) GetDiscussionRepository(ctx context.Context, repo string) (DiscussionRepository, error) {
	vars, err := discussionRepoVariables(repo)
	if err != nil {
		return DiscussionRepository{}, err
	}
	var out struct {
		Repository *struct {
			ID, NameWithOwner     string
			HasDiscussionsEnabled *bool
		}
	}
	if err = c.graphQL(ctx, discussionRepositoryQuery, vars, &out, &graphQLBudget{}); err != nil {
		return DiscussionRepository{}, err
	}
	n := out.Repository
	if n == nil || !validGraphQLID(n.ID) || !strings.EqualFold(n.NameWithOwner, repo) || n.HasDiscussionsEnabled == nil {
		return DiscussionRepository{}, discussionError()
	}
	return DiscussionRepository{n.ID, n.NameWithOwner, *n.HasDiscussionsEnabled}, nil
}
func (c *Client) ListDiscussionCategories(ctx context.Context, repo string) ([]DiscussionCategory, error) {
	vars, err := discussionRepoVariables(repo)
	if err != nil {
		return nil, err
	}
	budget := &graphQLBudget{}
	repositoryID := ""
	rows, err := graphQLPages(func(after *string) (*graphQLConnection[discussionCategoryNode], error) {
		vars["after"] = after
		var out struct {
			Repository *struct {
				ID                   string
				DiscussionCategories *graphQLConnection[discussionCategoryNode]
			}
		}
		if err := c.graphQL(ctx, discussionCategoriesQuery, vars, &out, budget); err != nil {
			return nil, err
		}
		if out.Repository == nil || !validGraphQLID(out.Repository.ID) || (repositoryID != "" && repositoryID != out.Repository.ID) {
			return nil, discussionError()
		}
		repositoryID = out.Repository.ID
		return out.Repository.DiscussionCategories, nil
	}, validCategory, func(n *discussionCategoryNode) string { return n.ID })
	if err != nil {
		return nil, err
	}
	result := make([]DiscussionCategory, 0, len(rows))
	for _, n := range rows {
		result = append(result, n.value())
	}
	return result, nil
}

// ListDiscussions returns metadata snapshots. Use GetDiscussion/ByID to collect
// an entire selected thread before applying native mutations.
func (c *Client) ListDiscussions(ctx context.Context, repo string) ([]Discussion, error) {
	vars, err := discussionRepoVariables(repo)
	if err != nil {
		return nil, err
	}
	budget := &graphQLBudget{}
	repositoryID := ""
	rows, err := graphQLPages(func(after *string) (*graphQLConnection[discussionNode], error) {
		vars["after"] = after
		var out struct {
			Repository *struct {
				ID          string
				Discussions *graphQLConnection[discussionNode]
			}
		}
		if err := c.graphQL(ctx, discussionsQuery, vars, &out, budget); err != nil {
			return nil, err
		}
		if out.Repository == nil || !validGraphQLID(out.Repository.ID) || (repositoryID != "" && repositoryID != out.Repository.ID) {
			return nil, discussionError()
		}
		repositoryID = out.Repository.ID
		return out.Repository.Discussions, nil
	}, func(n *discussionNode) bool { return validDiscussion(n) && n.Repository.ID == repositoryID }, func(n *discussionNode) string { return n.ID })
	if err != nil {
		return nil, err
	}
	result := make([]Discussion, 0, len(rows))
	for _, n := range rows {
		result = append(result, n.value())
	}
	return result, nil
}
func (c *Client) GetDiscussion(ctx context.Context, repo string, number int) (Discussion, error) {
	vars, err := discussionRepoVariables(repo)
	if err != nil {
		return Discussion{}, err
	}
	if number <= 0 || number > 2147483647 {
		return Discussion{}, errors.New("GitHub discussion number must be a positive GraphQL Int")
	}
	vars["number"] = number
	budget := &graphQLBudget{}
	var out struct {
		Repository *struct{ Discussion *discussionNode }
	}
	if err = c.graphQL(ctx, discussionNumberQuery, vars, &out, budget); err != nil {
		return Discussion{}, err
	}
	if out.Repository == nil || !validDiscussion(out.Repository.Discussion) || *out.Repository.Discussion.Number != number {
		return Discussion{}, discussionError()
	}
	return c.discussionThread(ctx, out.Repository.Discussion, budget)
}
func (c *Client) GetDiscussionByID(ctx context.Context, id string) (Discussion, error) {
	if !validGraphQLID(id) {
		return Discussion{}, discussionError()
	}
	budget := &graphQLBudget{}
	var out struct{ Node *discussionNode }
	if err := c.graphQL(ctx, discussionIDQuery, map[string]any{"id": id}, &out, budget); err != nil {
		return Discussion{}, err
	}
	if !validDiscussion(out.Node) || out.Node.ID != id {
		return Discussion{}, discussionError()
	}
	return c.discussionThread(ctx, out.Node, budget)
}
func (c *Client) discussionThread(ctx context.Context, n *discussionNode, budget *graphQLBudget) (Discussion, error) {
	top, err := graphQLPages(func(after *string) (*graphQLConnection[discussionCommentNode], error) {
		var out struct {
			Node *struct {
				Type     string `json:"__typename"`
				ID       string
				Comments *graphQLConnection[discussionCommentNode]
			}
		}
		if err := c.graphQL(ctx, discussionCommentsQuery, map[string]any{"id": n.ID, "after": after}, &out, budget); err != nil {
			return nil, err
		}
		if out.Node == nil || out.Node.Type != "Discussion" || out.Node.ID != n.ID {
			return nil, discussionError()
		}
		return out.Node.Comments, nil
	}, func(v *discussionCommentNode) bool {
		return validDiscussionComment(v) && v.Discussion.ID == n.ID && v.ReplyTo == nil
	}, func(v *discussionCommentNode) string { return v.ID })
	if err != nil {
		return Discussion{}, err
	}
	result := n.value()
	result.Comments = make([]DiscussionComment, 0, len(top))
	seen := map[string]bool{}
	for _, v := range top {
		seen[v.ID] = true
		result.Comments = append(result.Comments, v.value())
	}
	for _, parent := range top {
		replies, err := graphQLPages(func(after *string) (*graphQLConnection[discussionCommentNode], error) {
			var out struct {
				Node *struct {
					Type    string `json:"__typename"`
					ID      string
					Replies *graphQLConnection[discussionCommentNode]
				}
			}
			if err := c.graphQL(ctx, discussionRepliesQuery, map[string]any{"id": parent.ID, "after": after}, &out, budget); err != nil {
				return nil, err
			}
			if out.Node == nil || out.Node.Type != "DiscussionComment" || out.Node.ID != parent.ID {
				return nil, discussionError()
			}
			return out.Node.Replies, nil
		}, func(v *discussionCommentNode) bool {
			return validDiscussionComment(v) && v.Discussion.ID == n.ID && refID(v.ReplyTo) == parent.ID
		}, func(v *discussionCommentNode) string { return v.ID })
		if err != nil {
			return Discussion{}, err
		}
		for _, v := range replies {
			if seen[v.ID] {
				return Discussion{}, discussionError()
			}
			seen[v.ID] = true
			result.Comments = append(result.Comments, v.value())
		}
	}
	if result.AnswerID != "" && !seen[result.AnswerID] {
		return Discussion{}, errors.New("GitHub discussion answer is absent from complete thread")
	}
	for _, comment := range result.Comments {
		if comment.IsAnswer != (comment.ID == result.AnswerID) {
			return Discussion{}, errors.New("GitHub discussion answer snapshot changed during traversal")
		}
	}
	return result, nil
}

func validDiscussionInput(input DiscussionInput) bool {
	return strings.TrimSpace(input.Title) != "" && utf8.ValidString(input.Title) && utf8.ValidString(input.Body) && len(input.Title)+len(input.Body) <= MaxResponseBytes && validGraphQLID(input.CategoryID)
}
func (c *Client) discussionMutation(ctx context.Context, document, field string, input map[string]any, check func(Discussion) bool) (Discussion, error) {
	var out map[string]*struct{ Discussion *discussionNode }
	if err := c.graphQL(ctx, document, map[string]any{"input": input}, &out, &graphQLBudget{}); err != nil {
		return Discussion{}, err
	}
	payload := out[field]
	if payload == nil || !validDiscussion(payload.Discussion) {
		return Discussion{}, discussionError()
	}
	d := payload.Discussion.value()
	if !check(d) {
		return Discussion{}, errors.New("GitHub discussion mutation returned mismatched identity/state; outcome may be unknown")
	}
	return d, nil
}
func (c *Client) CreateDiscussion(ctx context.Context, repositoryID string, input DiscussionInput) (Discussion, error) {
	if !validGraphQLID(repositoryID) || !validDiscussionInput(input) {
		return Discussion{}, discussionError()
	}
	return c.discussionMutation(ctx, createDiscussionMutation, "createDiscussion", map[string]any{"repositoryId": repositoryID, "categoryId": input.CategoryID, "title": input.Title, "body": input.Body}, func(d Discussion) bool {
		return d.RepositoryID == repositoryID && d.Category.ID == input.CategoryID && d.Title == input.Title && d.Body == input.Body
	})
}
func (c *Client) UpdateDiscussion(ctx context.Context, id string, input DiscussionInput) (Discussion, error) {
	if !validGraphQLID(id) || !validDiscussionInput(input) {
		return Discussion{}, discussionError()
	}
	return c.discussionMutation(ctx, updateDiscussionMutation, "updateDiscussion", map[string]any{"discussionId": id, "categoryId": input.CategoryID, "title": input.Title, "body": input.Body}, func(d Discussion) bool {
		return d.ID == id && d.Category.ID == input.CategoryID && d.Title == input.Title && d.Body == input.Body
	})
}
func (c *Client) discussionCommentMutation(ctx context.Context, document, field string, input map[string]any, check func(DiscussionComment) bool) (DiscussionComment, error) {
	var out map[string]*struct{ Comment *discussionCommentNode }
	if err := c.graphQL(ctx, document, map[string]any{"input": input}, &out, &graphQLBudget{}); err != nil {
		return DiscussionComment{}, err
	}
	p := out[field]
	if p == nil || !validDiscussionComment(p.Comment) {
		return DiscussionComment{}, discussionError()
	}
	v := p.Comment.value()
	if !check(v) {
		return DiscussionComment{}, errors.New("GitHub discussion comment mutation returned mismatched identity/state; outcome may be unknown")
	}
	return v, nil
}
func validDiscussionBody(body string) bool {
	return utf8.ValidString(body) && strings.TrimSpace(body) != "" && len(body) <= MaxResponseBytes
}
func (c *Client) AddDiscussionComment(ctx context.Context, discussionID, replyToID, body string) (DiscussionComment, error) {
	if !validGraphQLID(discussionID) || (replyToID != "" && !validGraphQLID(replyToID)) || !validDiscussionBody(body) {
		return DiscussionComment{}, discussionError()
	}
	input := map[string]any{"discussionId": discussionID, "body": body}
	if replyToID != "" {
		input["replyToId"] = replyToID
	}
	return c.discussionCommentMutation(ctx, addDiscussionCommentMutation, "addDiscussionComment", input, func(v DiscussionComment) bool {
		return v.DiscussionID == discussionID && v.ParentID == replyToID && v.Body == body
	})
}
func (c *Client) UpdateDiscussionComment(ctx context.Context, commentID, body string) (DiscussionComment, error) {
	if !validGraphQLID(commentID) || !validDiscussionBody(body) {
		return DiscussionComment{}, discussionError()
	}
	return c.discussionCommentMutation(ctx, updateDiscussionCommentMutation, "updateDiscussionComment", map[string]any{"commentId": commentID, "body": body}, func(v DiscussionComment) bool { return v.ID == commentID && v.Body == body })
}
func (c *Client) MarkDiscussionCommentAsAnswer(ctx context.Context, discussionID, commentID string) (Discussion, error) {
	if !validGraphQLID(discussionID) || !validGraphQLID(commentID) {
		return Discussion{}, discussionError()
	}
	return c.discussionMutation(ctx, markDiscussionAnswerMutation, "markDiscussionCommentAsAnswer", map[string]any{"id": commentID}, func(d Discussion) bool { return d.ID == discussionID && d.AnswerID == commentID })
}
func (c *Client) UnmarkDiscussionCommentAsAnswer(ctx context.Context, discussionID, commentID string) (Discussion, error) {
	if !validGraphQLID(discussionID) || !validGraphQLID(commentID) {
		return Discussion{}, discussionError()
	}
	return c.discussionMutation(ctx, unmarkDiscussionAnswerMutation, "unmarkDiscussionCommentAsAnswer", map[string]any{"id": commentID}, func(d Discussion) bool { return d.ID == discussionID && d.AnswerID == "" })
}
func (c *Client) CloseDiscussion(ctx context.Context, id, reason string) (Discussion, error) {
	if !validGraphQLID(id) || (reason != "RESOLVED" && reason != "OUTDATED" && reason != "DUPLICATE") {
		return Discussion{}, discussionError()
	}
	return c.discussionMutation(ctx, closeDiscussionMutation, "closeDiscussion", map[string]any{"discussionId": id, "reason": reason}, func(d Discussion) bool { return d.ID == id && d.Closed && d.StateReason == reason })
}
func (c *Client) ReopenDiscussion(ctx context.Context, id string) (Discussion, error) {
	if !validGraphQLID(id) {
		return Discussion{}, discussionError()
	}
	return c.discussionMutation(ctx, reopenDiscussionMutation, "reopenDiscussion", map[string]any{"discussionId": id}, func(d Discussion) bool { return d.ID == id && !d.Closed })
}
