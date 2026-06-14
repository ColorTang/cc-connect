package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

// handleDocCommand parses /doc subcommands and executes them.
func (p *Platform) handleDocCommand(ctx context.Context, rctx replyContext, text, userID string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 || fields[0] != "/doc" {
		return false
	}

	client := newDriveClient(p)
	if len(fields) < 2 {
		p.Reply(ctx, rctx, docHelp())
		return true
	}

	sub := strings.ToLower(fields[1])
	switch sub {
	case "create":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Please provide a title, e.g. /doc create My Notes")
			return true
		}
		title := strings.TrimSpace(strings.TrimPrefix(text, "/doc create "))
		if title == "" {
			title = "Untitled"
		}
		token, url, _, err := client.createDocx(ctx, title, "")
		if err != nil {
			p.Reply(ctx, rctx, "Create doc failed: "+err.Error())
			return true
		}
		shareMsg := grantEditMessage(ctx, client, driveFileTypeDocx, token, userID)
		p.Reply(ctx, rctx, fmt.Sprintf("Created doc: %s\nURL: %s\nToken: %s%s", title, url, token, shareMsg))
		return true

	case "fetch", "read":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Please provide a docx URL or token, e.g. /doc fetch <URL>")
			return true
		}
		docToken := parseFileToken(fields[2])
		content, err := client.getDocumentContent(ctx, docToken)
		if err != nil {
			p.Reply(ctx, rctx, "Fetch doc failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, truncate(content, 4000))
		return true

	case "update":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Please provide a docx URL/token and content, e.g. /doc update <URL> hello")
			return true
		}
		docToken := parseFileToken(fields[2])
		content := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/doc update %s", fields[2])))
		if content == "" {
			p.Reply(ctx, rctx, "Update content cannot be empty")
			return true
		}
		if err := client.updateDocumentBlocks(ctx, docToken, content); err != nil {
			p.Reply(ctx, rctx, "Update doc failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Doc updated: "+client.fileURL(driveFileTypeDocx, docToken))
		return true

	case "delete":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Please provide a docx URL/token, e.g. /doc delete <URL>")
			return true
		}
		docToken := parseFileToken(fields[2])
		if err := client.DeleteFile(ctx, driveFileTypeDocx, docToken); err != nil {
			p.Reply(ctx, rctx, "Delete doc failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Doc deleted (moved to recycle bin)")
		return true

	case "grant":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage: /doc grant <URL/token> <user_open_id>")
			return true
		}
		docToken := parseFileToken(fields[2])
		if err := client.GrantEditPermission(ctx, driveFileTypeDocx, docToken, fields[3]); err != nil {
			p.Reply(ctx, rctx, "Grant permission failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Edit permission granted.")
		return true

	default:
		p.Reply(ctx, rctx, docHelp())
		return true
	}
}

func (d *driveClient) createDocx(ctx context.Context, title, folderToken string) (string, string, driveFileType, error) {
	req := larkdocx.NewCreateDocumentReqBuilder().
		Body(larkdocx.NewCreateDocumentReqBodyBuilder().
			Title(title).
			FolderToken(folderToken).
			Build()).
		Build()

	var resp *larkdocx.CreateDocumentResp
	err := d.platform.withTransientRetry(ctx, "create docx", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "create docx", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Docx.Document.Create(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: create docx api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: create docx failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
	if err != nil {
		return "", "", "", err
	}
	if resp.Data == nil || resp.Data.Document == nil || resp.Data.Document.DocumentId == nil {
		return "", "", "", fmt.Errorf("%s: create docx: no document_id returned", d.platform.tag())
	}

	token := *resp.Data.Document.DocumentId
	return token, d.fileURL(driveFileTypeDocx, token), driveFileTypeDocx, nil
}

func (d *driveClient) getDocumentContent(ctx context.Context, docToken string) (string, error) {
	req := larkdocx.NewRawContentDocumentReqBuilder().
		DocumentId(docToken).
		Build()

	var resp *larkdocx.RawContentDocumentResp
	err := d.platform.withTransientRetry(ctx, "get document content", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "get document content", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Docx.Document.RawContent(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: get document content api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: get document content failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
	if err != nil {
		return "", err
	}
	if resp.Data == nil {
		return "", nil
	}
	return stringValue(resp.Data.Content), nil
}

func (d *driveClient) updateDocumentBlocks(ctx context.Context, docToken string, content string) error {
	pageBlockID, err := d.getPageBlockID(ctx, docToken)
	if err != nil {
		return err
	}

	if err := d.deletePageBlockChildren(ctx, docToken, pageBlockID); err != nil {
		return err
	}

	lines := strings.Split(content, "\n")
	var blocks []*larkdocx.Block
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		blocks = append(blocks, d.buildTextBlock(line))
	}

	if len(blocks) == 0 {
		return nil
	}

	req := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(docToken).
		BlockId(pageBlockID).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
			Children(blocks).
			Build()).
		Build()

	return d.platform.withTransientRetry(ctx, "append document blocks", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "append document blocks", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			resp, err := client.Docx.DocumentBlockChildren.Create(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: append document blocks api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: append document blocks failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
}

func (d *driveClient) getPageBlockID(ctx context.Context, docToken string) (string, error) {
	req := larkdocx.NewListDocumentBlockReqBuilder().
		DocumentId(docToken).
		Build()

	var resp *larkdocx.ListDocumentBlockResp
	err := d.platform.withTransientRetry(ctx, "list document blocks", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "list document blocks", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Docx.DocumentBlock.List(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: list document blocks api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: list document blocks failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
	if err != nil {
		return "", err
	}
	if resp.Data == nil || len(resp.Data.Items) == 0 {
		return "", fmt.Errorf("%s: document has no blocks", d.platform.tag())
	}
	for _, item := range resp.Data.Items {
		if item == nil || item.BlockId == nil {
			continue
		}
		if item.Page != nil {
			return *item.BlockId, nil
		}
	}
	if resp.Data.Items[0].BlockId != nil {
		return *resp.Data.Items[0].BlockId, nil
	}
	return "", fmt.Errorf("%s: could not determine page block id", d.platform.tag())
}

func (d *driveClient) deletePageBlockChildren(ctx context.Context, docToken, pageBlockID string) error {
	req := larkdocx.NewGetDocumentBlockChildrenReqBuilder().
		DocumentId(docToken).
		BlockId(pageBlockID).
		Build()

	var resp *larkdocx.GetDocumentBlockChildrenResp
	err := d.platform.withTransientRetry(ctx, "get page block children", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "get page block children", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Docx.DocumentBlockChildren.Get(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: get page block children api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: get page block children failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	if resp.Data == nil || resp.Data.Items == nil || len(resp.Data.Items) == 0 {
		return nil
	}

	endIndex := len(resp.Data.Items)
	delReq := larkdocx.NewBatchDeleteDocumentBlockChildrenReqBuilder().
		DocumentId(docToken).
		BlockId(pageBlockID).
		Body(larkdocx.NewBatchDeleteDocumentBlockChildrenReqBodyBuilder().
			StartIndex(0).
			EndIndex(endIndex).
			Build()).
		Build()

	return d.platform.withTransientRetry(ctx, "delete page block children", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "delete page block children", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			resp, err := client.Docx.DocumentBlockChildren.BatchDelete(ctx, delReq, options...)
			if err != nil {
				return fmt.Errorf("%s: delete page block children api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: delete page block children failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
}

func (d *driveClient) buildTextBlock(line string) *larkdocx.Block {
	return larkdocx.NewBlockBuilder().
		Text(
			larkdocx.NewTextBuilder().
				Elements([]*larkdocx.TextElement{
					larkdocx.NewTextElementBuilder().
						TextRun(
							larkdocx.NewTextRunBuilder().
								Content(line).
								Build()).
						Build(),
				}).
				Build()).
		Build()
}

func docHelp() string {
	return "Doc commands:\n" +
		"/doc create <title>\n" +
		"/doc fetch <docx URL/token>\n" +
		"/doc update <docx URL/token> <content>\n" +
		"/doc delete <docx URL/token>\n" +
		"/doc grant <docx URL/token> <user_open_id>"
}

// grantEditMessage attempts to grant edit permission and returns a human-readable message.
func grantEditMessage(ctx context.Context, client *driveClient, ft driveFileType, token, userID string) string {
	if userID == "" {
		return ""
	}
	if err := client.GrantEditPermission(ctx, ft, token, userID); err != nil {
		return fmt.Sprintf("\nNote: failed to grant you edit permission (%s).", err.Error())
	}
	return "\nYou have been granted edit permission."
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n\n...(truncated)"
}


// _ keeps json import used by consumers of this file.
var _ = json.Marshal
