package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/output"
)

type PagePropertyCmd struct {
	Get PagePropertyGetCmd `cmd:"" help:"Get complete values for a page property"`
}

type PagePropertyGetCmd struct {
	Page       string `arg:"" help:"Page URL, name, or ID"`
	Name       string `help:"Property name (preferred)" short:"n"`
	PropertyID string `help:"Property ID (skips name lookup)" name:"property-id"`
	JSON       bool   `help:"Output as JSON" short:"j"`
}

type pagePropertyGetOutput struct {
	PageID       string `json:"page_id"`
	PropertyName string `json:"property_name,omitempty"`
	PropertyID   string `json:"property_id"`
	ItemCount    int    `json:"item_count"`
	Items        []any  `json:"items"`
}

func (c *PagePropertyGetCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPagePropertyGet(ctx, c.Page, c.Name, c.PropertyID)
}

func runPagePropertyGet(ctx *Context, page, propertyName, propertyID string) error {
	propertyName = strings.TrimSpace(propertyName)
	propertyID = strings.TrimSpace(propertyID)
	if propertyName == "" && propertyID == "" {
		return &output.UserError{Message: "specify --name or --property-id"}
	}
	if propertyName != "" && propertyID != "" {
		return &output.UserError{Message: "use either --name or --property-id, not both"}
	}

	bgCtx := context.Background()
	pageID, err := resolvePageIDForPropertyRead(bgCtx, page)
	if err != nil {
		output.PrintError(err)
		return err
	}

	apiClient, err := cli.RequireOfficialAPIClient()
	if err != nil {
		output.PrintError(err)
		return err
	}

	if propertyID == "" {
		props, err := apiClient.RetrievePageProperties(bgCtx, pageID)
		if err != nil {
			output.PrintError(err)
			return err
		}

		var found bool
		propertyID, found = findPropertyIDByName(props, propertyName)
		if !found {
			available := make([]string, 0, len(props))
			for name := range props {
				available = append(available, name)
			}
			sort.Strings(available)
			err := fmt.Errorf("property %q not found. Available properties: %s", propertyName, strings.Join(available, ", "))
			output.PrintError(err)
			return err
		}
	}

	items, err := apiClient.RetrievePagePropertyItems(bgCtx, pageID, propertyID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	out := pagePropertyGetOutput{
		PageID:       pageID,
		PropertyName: propertyName,
		PropertyID:   propertyID,
		ItemCount:    len(items),
		Items:        items,
	}

	if ctx.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	label := propertyID
	if propertyName != "" {
		label = propertyName + " (" + propertyID + ")"
	}
	fmt.Printf("Page: %s\n", pageID)
	fmt.Printf("Property: %s\n", label)
	fmt.Printf("Items: %d\n\n", len(items))

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

func resolvePageIDForPropertyRead(ctx context.Context, page string) (string, error) {
	ref := cli.ParsePageRef(page)
	switch ref.Kind {
	case cli.RefID:
		return ref.ID, nil
	case cli.RefURL:
		if id, ok := cli.ExtractNotionUUID(page); ok {
			return id, nil
		}
		return "", &output.UserError{Message: fmt.Sprintf("could not extract page ID from URL: %s\nUse the page ID directly instead.", page)}
	case cli.RefName:
		client, err := cli.RequireClient()
		if err != nil {
			return "", err
		}
		defer func() { _ = client.Close() }()
		return cli.ResolvePageID(ctx, client, page)
	default:
		return "", fmt.Errorf("unsupported page reference: %s", page)
	}
}

func findPropertyIDByName(properties map[string]api.PagePropertyMeta, propertyName string) (string, bool) {
	if meta, ok := properties[propertyName]; ok && strings.TrimSpace(meta.ID) != "" {
		return meta.ID, true
	}

	lowerName := strings.ToLower(propertyName)
	for name, meta := range properties {
		if strings.ToLower(name) == lowerName && strings.TrimSpace(meta.ID) != "" {
			return meta.ID, true
		}
	}

	return "", false
}
