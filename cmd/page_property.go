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

	apiClient, err := cli.RequireOfficialAPIClient(officialAPIOverrides(ctx))
	if err != nil {
		output.PrintError(err)
		return err
	}

	if propertyID == "" {
		properties, err := apiClient.RetrievePageProperties(bgCtx, pageID)
		if err != nil {
			output.PrintError(err)
			return err
		}

		var found bool
		propertyID, found = findPropertyIDByName(properties, propertyName)
		if !found {
			available := make([]string, 0, len(properties))
			for name := range properties {
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

	result := pagePropertyGetOutput{
		PageID:       pageID,
		PropertyName: propertyName,
		PropertyID:   propertyID,
		ItemCount:    len(items),
		Items:        items,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if ctx.JSON {
		return encoder.Encode(result)
	}

	label := propertyID
	if propertyName != "" {
		label = propertyName + " (" + propertyID + ")"
	}
	fmt.Printf("Page: %s\nProperty: %s\nItems: %d\n\n", pageID, label, len(items))
	return encoder.Encode(items)
}

func resolvePageIDForPropertyRead(ctx context.Context, page string) (string, error) {
	ref := cli.ParsePageRef(page)
	switch ref.Kind {
	case cli.RefID:
		return ref.ID, nil
	case cli.RefName:
		client, err := cli.RequireClient()
		if err != nil {
			return "", err
		}
		defer func() { _ = client.Close() }()
		return cli.ResolvePageID(ctx, client, page)
	default:
		return "", &output.UserError{Message: "page property get requires a page URL, name, or ID"}
	}
}

func findPropertyIDByName(properties map[string]api.PagePropertyMeta, propertyName string) (string, bool) {
	if meta, ok := properties[propertyName]; ok && strings.TrimSpace(meta.ID) != "" {
		return meta.ID, true
	}

	for name, meta := range properties {
		if strings.EqualFold(name, propertyName) && strings.TrimSpace(meta.ID) != "" {
			return meta.ID, true
		}
	}
	return "", false
}
