package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type uploadedLocalImage struct {
	Alt          string
	FileUploadID string
	Placeholder  string
	ResolvedPath string
}

func prepareLocalImageUploads(cmdCtx *Context, ctx context.Context, sourceFile, markdown string) (string, []uploadedLocalImage, error) {
	rewritten, placements, err := cli.RewriteStandaloneLocalImages(markdown, sourceFile)
	if err != nil {
		return "", nil, err
	}
	if len(placements) == 0 {
		return markdown, nil, nil
	}

	apiClient, err := cli.RequireOfficialAPIClient(officialAPIOverrides(cmdCtx))
	if err != nil {
		return "", nil, err
	}

	uploadIDByPath := make(map[string]string, len(placements))
	uploads := make([]uploadedLocalImage, 0, len(placements))
	for _, placement := range placements {
		uploadID, ok := uploadIDByPath[placement.Resolved]
		if !ok {
			fileData, err := os.ReadFile(placement.Resolved)
			if err != nil {
				return "", nil, fmt.Errorf("read local image %q: %w", placement.Resolved, err)
			}
			uploadID, err = apiClient.UploadFile(ctx, placement.Resolved, fileData)
			if err != nil {
				return "", nil, fmt.Errorf("upload local image %q: %w", placement.Resolved, err)
			}
			uploadIDByPath[placement.Resolved] = uploadID
		}

		uploads = append(uploads, uploadedLocalImage{
			Alt:          placement.Alt,
			FileUploadID: uploadID,
			Placeholder:  placement.Placeholder,
			ResolvedPath: placement.Resolved,
		})
	}

	return rewritten, uploads, nil
}

func requireLocalImageParent(uploads []uploadedLocalImage, parent, parentDB string) error {
	if len(uploads) == 0 {
		return nil
	}
	if strings.TrimSpace(parent) != "" || strings.TrimSpace(parentDB) != "" {
		return nil
	}
	return &output.UserError{
		Message: "standalone local image upload requires --parent or --parent-db shared with your Notion integration",
	}
}

func substituteUploadedLocalImages(cmdCtx *Context, ctx context.Context, pageID string, uploads []uploadedLocalImage) error {
	if strings.TrimSpace(pageID) == "" || len(uploads) == 0 {
		return nil
	}

	apiClient, err := cli.RequireOfficialAPIClient(officialAPIOverrides(cmdCtx))
	if err != nil {
		return err
	}

	blocks, err := apiClient.ListAllBlockChildren(ctx, pageID)
	if err != nil {
		return err
	}

	blocksByPlaceholder := make(map[string]api.Block, len(uploads))
	for _, block := range blocks {
		if block.Type != "paragraph" || block.Paragraph == nil {
			continue
		}
		text := paragraphPlainText(block)
		if text == "" {
			continue
		}
		blocksByPlaceholder[text] = block
	}

	for _, upload := range uploads {
		block, ok := blocksByPlaceholder[upload.Placeholder]
		if !ok {
			return fmt.Errorf("could not find placeholder block for %q", upload.ResolvedPath)
		}
		if err := apiClient.AppendUploadedImageAfter(ctx, pageID, block.ID, api.UploadedImageBlock{
			FileUploadID: upload.FileUploadID,
			Caption:      upload.Alt,
		}); err != nil {
			return err
		}
		if err := apiClient.DeleteBlock(ctx, block.ID); err != nil {
			return err
		}
	}

	return nil
}

func rollbackSyncedPage(ctx context.Context, client *mcp.Client, pageID string, snapshot *api.PageMarkdown) error {
	if snapshot == nil || strings.TrimSpace(snapshot.Markdown) == "" {
		return nil
	}
	return client.UpdatePage(ctx, mcp.UpdatePageRequest{
		PageID:     pageID,
		Command:    "replace_content",
		NewContent: snapshot.Markdown,
	})
}

func pageIDFromCreateResponse(resp *mcp.CreatePageResponse) string {
	if resp == nil {
		return ""
	}
	if strings.TrimSpace(resp.ID) != "" {
		return strings.TrimSpace(resp.ID)
	}
	if strings.TrimSpace(resp.URL) == "" {
		return ""
	}
	id, _ := cli.ExtractNotionUUID(resp.URL)
	return id
}

func paragraphPlainText(block api.Block) string {
	if block.Paragraph == nil {
		return ""
	}

	var builder strings.Builder
	for _, part := range block.Paragraph.RichText {
		builder.WriteString(part.PlainText)
	}
	return builder.String()
}
