package cmd

import (
	"context"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type CommentCmd struct {
	List   CommentListCmd   `cmd:"" help:"List comments on a page"`
	Create CommentCreateCmd `cmd:"" help:"Create a comment on a page"`
}

type CommentListCmd struct {
	PageID string `arg:"" help:"Page ID or URL"`
	JSON   bool   `help:"Output as JSON" short:"j"`
}

func (c *CommentListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runCommentList(ctx, c.PageID)
}

func runCommentList(ctx *Context, pageID string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}

	bgCtx := context.Background()
	resolvedPageID, err := cli.ResolvePageID(bgCtx, client, pageID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	req := mcp.GetCommentsRequest{
		PageID:           resolvedPageID,
		IncludeAllBlocks: true,
	}

	resp, err := client.GetComments(bgCtx, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	comments := convertComments(resp.Comments)
	return output.PrintComments(comments, ctx.JSON)
}

func convertComments(mcpComments []mcp.Comment) []output.Comment {
	comments := make([]output.Comment, 0, len(mcpComments))
	for _, c := range mcpComments {
		comments = append(comments, output.Comment{
			ID:             c.ID,
			DiscussionID:   c.DiscussionID,
			CreatedTime:    c.CreatedTime,
			LastEditedTime: c.LastEditedTime,
			CreatedBy:      c.CreatedBy.ID,
			Content:        extractRichText(c.RichText),
		})
	}
	return comments
}

func extractRichText(richText []mcp.RichText) string {
	var content string
	for _, rt := range richText {
		content += rt.PlainText
	}
	return content
}

type CommentCreateCmd struct {
	PageID  string `arg:"" help:"Page ID or URL"`
	Content string `help:"Comment content" short:"c" required:""`
	JSON    bool   `help:"Output as JSON" short:"j"`
}

func (c *CommentCreateCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runCommentCreate(ctx, c.PageID, c.Content)
}

func runCommentCreate(ctx *Context, pageID, content string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}

	bgCtx := context.Background()
	resolvedPageID, err := cli.ResolvePageID(bgCtx, client, pageID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	req := mcp.CreateCommentRequest{
		PageID: resolvedPageID,
		Text:   content,
	}

	comment, err := client.CreateComment(bgCtx, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if ctx.JSON {
		outComments := []output.Comment{{
			ID:             comment.ID,
			DiscussionID:   comment.DiscussionID,
			CreatedTime:    comment.CreatedTime,
			LastEditedTime: comment.LastEditedTime,
			CreatedBy:      comment.CreatedBy.ID,
			Content:        extractRichText(comment.RichText),
		}}
		return output.PrintComments(outComments, true)
	}

	output.PrintSuccess("Comment created")
	return nil
}
