package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larksheets "github.com/larksuite/oapi-sdk-go/v3/service/sheets/v3"
)

// handleSheetCommand parses /sheet subcommands and executes them.
func (p *Platform) handleSheetCommand(ctx context.Context, rctx replyContext, text, userID string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 || fields[0] != "/sheet" {
		return false
	}

	client := newDriveClient(p)
	if len(fields) < 2 {
		p.Reply(ctx, rctx, sheetHelp())
		return true
	}

	sub := strings.ToLower(fields[1])
	switch sub {
	case "create":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Please provide a title, e.g. /sheet create My Sheet")
			return true
		}
		title := strings.TrimSpace(strings.TrimPrefix(text, "/sheet create "))
		if title == "" {
			title = "Untitled"
		}
		token, url, _, err := client.createSheet(ctx, title, "")
		if err != nil {
			p.Reply(ctx, rctx, "Create sheet failed: "+err.Error())
			return true
		}
		shareMsg := grantEditMessage(ctx, client, driveFileTypeSheet, token, userID)
		p.Reply(ctx, rctx, fmt.Sprintf("Created sheet: %s\nURL: %s\nToken: %s%s", title, url, token, shareMsg))
		return true

	case "workbook-info":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Usage: /sheet workbook-info <URL/token>")
			return true
		}
		token := parseFileToken(fields[2])
		info, err := client.listSheets(ctx, token)
		if err != nil {
			p.Reply(ctx, rctx, "List worksheets failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, formatSheetInfo(info))
		return true

	case "worksheet-create":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage: /sheet worksheet-create <URL/token> <title>")
			return true
		}
		token := parseFileToken(fields[2])
		title := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/sheet worksheet-create %s", fields[2])))
		sheetID, err := client.createWorksheet(ctx, token, title)
		if err != nil {
			p.Reply(ctx, rctx, "Create worksheet failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, fmt.Sprintf("Worksheet created: %s\nSheet ID: %s", title, sheetID))
		return true

	case "worksheet-delete":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage: /sheet worksheet-delete <URL/token> <sheet_id>")
			return true
		}
		token := parseFileToken(fields[2])
		sheetID := fields[3]
		if err := client.deleteWorksheet(ctx, token, sheetID); err != nil {
			p.Reply(ctx, rctx, "Delete worksheet failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Worksheet deleted")
		return true

	case "cells-get":
		if len(fields) < 5 {
			p.Reply(ctx, rctx, "Usage: /sheet cells-get <URL/token> <sheet_id> <range>")
			return true
		}
		token := parseFileToken(fields[2])
		sheetID := fields[3]
		sheetRange := fields[4]
		content, err := client.getSheetCells(ctx, token, sheetID, sheetRange)
		if err != nil {
			p.Reply(ctx, rctx, "Read cells failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, truncate(content, 4000))
		return true

	case "cells-set":
		if len(fields) < 6 {
			p.Reply(ctx, rctx, "Usage: /sheet cells-set <URL/token> <sheet_id> <range> <JSON values>")
			return true
		}
		token := parseFileToken(fields[2])
		sheetID := fields[3]
		sheetRange := fields[4]
		valueJSON := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/sheet cells-set %s %s %s", fields[2], fields[3], fields[4])))
		if valueJSON == "" {
			p.Reply(ctx, rctx, "Values cannot be empty")
			return true
		}
		var values [][]any
		if err := json.Unmarshal([]byte(valueJSON), &values); err != nil {
			p.Reply(ctx, rctx, "Invalid JSON values: "+err.Error())
			return true
		}
		if err := client.setSheetCells(ctx, token, sheetID, sheetRange, values); err != nil {
			p.Reply(ctx, rctx, "Write cells failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Cells updated")
		return true

	case "delete":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Usage: /sheet delete <URL/token>")
			return true
		}
		token := parseFileToken(fields[2])
		if err := client.DeleteFile(ctx, driveFileTypeSheet, token); err != nil {
			p.Reply(ctx, rctx, "Delete sheet failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Sheet deleted (moved to recycle bin)")
		return true

	case "grant":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage: /sheet grant <URL/token> <user_open_id>")
			return true
		}
		token := parseFileToken(fields[2])
		if err := client.GrantEditPermission(ctx, driveFileTypeSheet, token, fields[3]); err != nil {
			p.Reply(ctx, rctx, "Grant permission failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Edit permission granted.")
		return true

	default:
		p.Reply(ctx, rctx, sheetHelp())
		return true
	}
}

func (d *driveClient) createSheet(ctx context.Context, title, folderToken string) (string, string, driveFileType, error) {
	req := larksheets.NewCreateSpreadsheetReqBuilder().
		Spreadsheet(larksheets.NewSpreadsheetBuilder().
			Title(title).
			FolderToken(folderToken).
			Build()).
		Build()

	var resp *larksheets.CreateSpreadsheetResp
	err := d.platform.withTransientRetry(ctx, "create sheet", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "create sheet", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Sheets.Spreadsheet.Create(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: create sheet api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: create sheet failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
	if err != nil {
		return "", "", "", err
	}
	if resp.Data == nil || resp.Data.Spreadsheet == nil || resp.Data.Spreadsheet.SpreadsheetToken == nil {
		return "", "", "", fmt.Errorf("%s: create sheet: no spreadsheet_token returned", d.platform.tag())
	}

	token := *resp.Data.Spreadsheet.SpreadsheetToken
	return token, d.fileURL(driveFileTypeSheet, token), driveFileTypeSheet, nil
}

func (d *driveClient) listSheets(ctx context.Context, token string) ([]*sheetInfo, error) {
	respBody, err := d.doDriveRequest(ctx, "GET", fmt.Sprintf("/open-apis/sheets/v2/spreadsheets/%s/metainfo", token), nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data *struct {
			Sheets []*sheetInfo `json:"sheets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("%s: parse sheet metainfo: %w", d.platform.tag(), err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("%s: get sheet metainfo failed code=%d msg=%s", d.platform.tag(), result.Code, result.Msg)
	}
	if result.Data == nil {
		return nil, nil
	}
	return result.Data.Sheets, nil
}

type sheetInfo struct {
	SheetID string `json:"sheetId"`
	Title   string `json:"title"`
	Index   int    `json:"index"`
}

func (d *driveClient) createWorksheet(ctx context.Context, token, title string) (string, error) {
	body := map[string]any{
		"requests": []map[string]any{
			{
				"addSheet": map[string]any{
					"properties": map[string]any{
						"title": title,
					},
				},
			},
		},
	}
	respBody, err := d.doDriveRequest(ctx, "POST", fmt.Sprintf("/open-apis/sheets/v2/spreadsheets/%s/sheets_batch_update", token), body)
	if err != nil {
		return "", err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data *struct {
			Replies []map[string]any `json:"replies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("%s: parse create worksheet response: %w", d.platform.tag(), err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("%s: create worksheet failed code=%d msg=%s", d.platform.tag(), result.Code, result.Msg)
	}
	if len(result.Data.Replies) == 0 {
		return "", fmt.Errorf("%s: create worksheet: no reply returned", d.platform.tag())
	}
	reply := result.Data.Replies[0]
	addSheet, ok := reply["addSheet"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s: create worksheet: unexpected reply shape", d.platform.tag())
	}
	props, ok := addSheet["properties"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s: create worksheet: missing properties", d.platform.tag())
	}
	sheetID, _ := props["sheetId"].(string)
	if sheetID == "" {
		return "", fmt.Errorf("%s: create worksheet: no sheetId returned", d.platform.tag())
	}
	return sheetID, nil
}

func (d *driveClient) deleteWorksheet(ctx context.Context, token, sheetID string) error {
	body := map[string]any{
		"requests": []map[string]any{
			{
				"deleteSheet": map[string]any{
					"sheetId": sheetID,
				},
			},
		},
	}
	_, err := d.doDriveRequest(ctx, "POST", fmt.Sprintf("/open-apis/sheets/v2/spreadsheets/%s/sheets_batch_update", token), body)
	return err
}

func (d *driveClient) getSheetCells(ctx context.Context, token, sheetID, sheetRange string) (string, error) {
	values, err := d.getSheetValues(ctx, token, sheetID+"!"+sheetRange)
	if err != nil {
		return "", err
	}
	if len(values) == 0 {
		return "(empty range)", nil
	}

	var sb strings.Builder
	for _, row := range values {
		cells := make([]string, len(row))
		for i, cell := range row {
			if cell == nil {
				cells[i] = ""
				continue
			}
			switch v := cell.(type) {
			case string:
				cells[i] = v
			default:
				cells[i] = fmt.Sprint(v)
			}
		}
		sb.WriteString(strings.Join(cells, "\t"))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func (d *driveClient) setSheetCells(ctx context.Context, token, sheetID, sheetRange string, values [][]any) error {
	body := map[string]any{
		"valueRange": map[string]any{
			"range":  sheetID + "!" + sheetRange,
			"values": values,
		},
	}
	_, err := d.doDriveRequest(ctx, "PUT", fmt.Sprintf("/open-apis/sheets/v2/spreadsheets/%s/values", token), body)
	return err
}

func (d *driveClient) getSheetValues(ctx context.Context, token, sheetRange string) ([][]any, error) {
	respBody, err := d.doDriveRequest(ctx, "GET", fmt.Sprintf("/open-apis/sheets/v2/spreadsheets/%s/values/%s", token, sheetRange), nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data *struct {
			ValueRange *struct {
				Range  string   `json:"range"`
				Values [][]any  `json:"values"`
			} `json:"valueRange"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("%s: parse sheet values: %w", d.platform.tag(), err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("%s: get sheet values failed code=%d msg=%s", d.platform.tag(), result.Code, result.Msg)
	}
	if result.Data == nil || result.Data.ValueRange == nil {
		return nil, nil
	}
	return result.Data.ValueRange.Values, nil
}

func formatSheetInfo(sheets []*sheetInfo) string {
	if len(sheets) == 0 {
		return "No worksheets found."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Worksheets (%d):\n", len(sheets)))
	for _, s := range sheets {
		sb.WriteString(fmt.Sprintf("  [%d] %s (sheetId: %s)\n", s.Index, s.Title, s.SheetID))
	}
	return sb.String()
}

func sheetHelp() string {
	return "Sheet commands:\n" +
		"/sheet create <title>\n" +
		"/sheet workbook-info <URL/token>\n" +
		"/sheet worksheet-create <URL/token> <title>\n" +
		"/sheet worksheet-delete <URL/token> <sheet_id>\n" +
		"/sheet cells-get <URL/token> <sheet_id> <range>\n" +
		"/sheet cells-set <URL/token> <sheet_id> <range> <JSON values>\n" +
		"/sheet delete <URL/token>\n" +
		"/sheet grant <URL/token> <user_open_id>"
}
