package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
)

// handleBaseCommand parses /base subcommands and executes them.
func (p *Platform) handleBaseCommand(ctx context.Context, rctx replyContext, text, userID string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 || fields[0] != "/base" {
		return false
	}

	client := newDriveClient(p)
	if len(fields) < 2 {
		p.Reply(ctx, rctx, baseHelp())
		return true
	}

	sub := strings.ToLower(fields[1])
	switch sub {
	case "create":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Please provide a name, e.g. /base create Project Tracker")
			return true
		}
		name := strings.TrimSpace(strings.TrimPrefix(text, "/base create "))
		if name == "" {
			name = "Untitled"
		}
		token, url, _, err := client.createBitable(ctx, name, "")
		if err != nil {
			p.Reply(ctx, rctx, "Create base failed: "+err.Error())
			return true
		}
		shareMsg := grantPermissionMessage(ctx, client, driveFileTypeBitable, token, userID, "full_access")
		p.Reply(ctx, rctx, fmt.Sprintf("Created base: %s\nURL: %s\nToken: %s%s", name, url, token, shareMsg))
		return true

	case "table-list":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Usage: /base table-list <URL/token>")
			return true
		}
		token := parseFileToken(fields[2])
		tables, err := client.listBitableTables(ctx, token)
		if err != nil {
			p.Reply(ctx, rctx, "List tables failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, formatTableList(tables))
		return true

	case "table-create":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage: /base table-create <URL/token> <name>")
			return true
		}
		token := parseFileToken(fields[2])
		name := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/base table-create %s", fields[2])))
		tableID, err := client.createBitableTable(ctx, token, name)
		if err != nil {
			p.Reply(ctx, rctx, "Create table failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, fmt.Sprintf("Table created: %s\nTable ID: %s", name, tableID))
		return true

	case "table-delete":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage: /base table-delete <URL/token> <table_id>")
			return true
		}
		token := parseFileToken(fields[2])
		tableID := fields[3]
		if err := client.deleteBitableTable(ctx, token, tableID); err != nil {
			p.Reply(ctx, rctx, "Delete table failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Table deleted")
		return true

	case "record-list":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage: /base record-list <URL/token> <table_id>")
			return true
		}
		token := parseFileToken(fields[2])
		tableID := fields[3]
		content, err := client.listBitableRecords(ctx, token, tableID)
		if err != nil {
			p.Reply(ctx, rctx, "List records failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, truncate(content, 4000))
		return true

	case "record-create":
		if len(fields) < 5 {
			p.Reply(ctx, rctx, "Usage: /base record-create <URL/token> <table_id> <JSON fields>")
			return true
		}
		token := parseFileToken(fields[2])
		tableID := fields[3]
		jsonStr := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/base record-create %s %s", fields[2], fields[3])))
		if jsonStr == "" {
			p.Reply(ctx, rctx, "Fields JSON cannot be empty")
			return true
		}
		var fieldsMap map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &fieldsMap); err != nil {
			p.Reply(ctx, rctx, "Invalid JSON fields: "+err.Error())
			return true
		}
		recordID, err := client.createBitableRecord(ctx, token, tableID, fieldsMap)
		if err != nil {
			p.Reply(ctx, rctx, "Create record failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Record created: "+recordID)
		return true

	case "record-update":
		if len(fields) < 6 {
			p.Reply(ctx, rctx, "Usage: /base record-update <URL/token> <table_id> <record_id> <JSON fields>")
			return true
		}
		token := parseFileToken(fields[2])
		tableID := fields[3]
		recordID := fields[4]
		jsonStr := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/base record-update %s %s %s", fields[2], fields[3], fields[4])))
		if jsonStr == "" {
			p.Reply(ctx, rctx, "Fields JSON cannot be empty")
			return true
		}
		var fieldsMap map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &fieldsMap); err != nil {
			p.Reply(ctx, rctx, "Invalid JSON fields: "+err.Error())
			return true
		}
		if err := client.updateBitableRecord(ctx, token, tableID, recordID, fieldsMap); err != nil {
			p.Reply(ctx, rctx, "Update record failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Record updated")
		return true

	case "record-delete":
		if len(fields) < 5 {
			p.Reply(ctx, rctx, "Usage: /base record-delete <URL/token> <table_id> <record_id>")
			return true
		}
		token := parseFileToken(fields[2])
		tableID := fields[3]
		recordID := fields[4]
		if err := client.deleteBitableRecord(ctx, token, tableID, recordID); err != nil {
			p.Reply(ctx, rctx, "Delete record failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Record deleted")
		return true

	case "delete":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Usage: /base delete <URL/token>")
			return true
		}
		token := parseFileToken(fields[2])
		if err := client.DeleteFile(ctx, driveFileTypeBitable, token); err != nil {
			p.Reply(ctx, rctx, "Delete base failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Base deleted (moved to recycle bin)")
		return true

	case "grant":
		if len(fields) < 5 {
			p.Reply(ctx, rctx, "Usage: /base grant <URL/token> <user_open_id> <view|edit|full_access>")
			return true
		}
		token := parseFileToken(fields[2])
		perm := fields[4]
		if err := client.GrantPermission(ctx, driveFileTypeBitable, token, fields[3], perm); err != nil {
			p.Reply(ctx, rctx, "Grant permission failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, fmt.Sprintf("%s permission granted.", perm))
		return true

	case "apply":
		if len(fields) < 5 {
			p.Reply(ctx, rctx, "Usage: /base apply <URL/token> <view|edit> [remark]")
			return true
		}
		token := parseFileToken(fields[2])
		perm := fields[3]
		remark := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/base apply %s %s", fields[2], fields[3])))
		if err := client.ApplyPermission(ctx, driveFileTypeBitable, token, perm, remark); err != nil {
			p.Reply(ctx, rctx, "Apply permission failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, fmt.Sprintf("Applied for %s permission. Wait for owner approval.", perm))
		return true

	default:
		p.Reply(ctx, rctx, baseHelp())
		return true
	}
}

func (d *driveClient) createBitable(ctx context.Context, name, folderToken string) (string, string, driveFileType, error) {
	req := larkbitable.NewCreateAppReqBuilder().
		ReqApp(larkbitable.NewReqAppBuilder().
			Name(name).
			FolderToken(folderToken).
			Build()).
		Build()

	var resp *larkbitable.CreateAppResp
	err := d.platform.withTransientRetry(ctx, "create bitable", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "create bitable", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Bitable.App.Create(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: create bitable api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: create bitable failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
	if err != nil {
		return "", "", "", err
	}
	if resp.Data == nil || resp.Data.App == nil || resp.Data.App.AppToken == nil {
		return "", "", "", fmt.Errorf("%s: create bitable: no app_token returned", d.platform.tag())
	}

	token := *resp.Data.App.AppToken
	return token, d.fileURL(driveFileTypeBitable, token), driveFileTypeBitable, nil
}

func (d *driveClient) listBitableTables(ctx context.Context, appToken string) ([]*larkbitable.AppTable, error) {
	req := larkbitable.NewListAppTableReqBuilder().
		AppToken(appToken).
		Build()

	var resp *larkbitable.ListAppTableResp
	err := d.platform.withTransientRetry(ctx, "list bitable tables", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "list bitable tables", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Bitable.AppTable.List(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: list bitable tables api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: list bitable tables failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if resp.Data == nil || resp.Data.Items == nil {
		return nil, nil
	}
	return resp.Data.Items, nil
}

func (d *driveClient) createBitableTable(ctx context.Context, appToken, name string) (string, error) {
	body := map[string]any{"name": name}
	respBody, err := d.doDriveRequest(ctx, "POST", fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables", appToken), body)
	if err != nil {
		return "", err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data *struct {
			TableId string `json:"table_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("%s: parse create table response: %w", d.platform.tag(), err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("%s: create table failed code=%d msg=%s", d.platform.tag(), result.Code, result.Msg)
	}
	if result.Data == nil || result.Data.TableId == "" {
		return "", fmt.Errorf("%s: create table: no table_id returned", d.platform.tag())
	}
	return result.Data.TableId, nil
}

func (d *driveClient) deleteBitableTable(ctx context.Context, appToken, tableID string) error {
	_, err := d.doDriveRequest(ctx, "DELETE", fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s", appToken, tableID), nil)
	return err
}

func (d *driveClient) listBitableRecords(ctx context.Context, appToken, tableID string) (string, error) {
	respBody, err := d.doDriveRequest(ctx, "GET", fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records?page_size=500", appToken, tableID), nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data *struct {
			Items []*struct {
				RecordId string         `json:"record_id"`
				Fields   map[string]any `json:"fields"`
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("%s: parse bitable records: %w", d.platform.tag(), err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("%s: list bitable records failed code=%d msg=%s", d.platform.tag(), result.Code, result.Msg)
	}
	if result.Data == nil || len(result.Data.Items) == 0 {
		return "(no records)", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total: %d\n\n", result.Data.Total))
	for _, item := range result.Data.Items {
		sb.WriteString(fmt.Sprintf("Record: %s\n", item.RecordId))
		for k, v := range item.Fields {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func (d *driveClient) createBitableRecord(ctx context.Context, appToken, tableID string, fields map[string]any) (string, error) {
	body := map[string]any{"fields": fields}
	respBody, err := d.doDriveRequest(ctx, "POST", fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records", appToken, tableID), body)
	if err != nil {
		return "", err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data *struct {
			Record *struct {
				RecordId string `json:"record_id"`
			} `json:"record"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("%s: parse create bitable record response: %w", d.platform.tag(), err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("%s: create bitable record failed code=%d msg=%s", d.platform.tag(), result.Code, result.Msg)
	}
	if result.Data == nil || result.Data.Record == nil {
		return "", fmt.Errorf("%s: create bitable record: no record returned", d.platform.tag())
	}
	return result.Data.Record.RecordId, nil
}

func (d *driveClient) updateBitableRecord(ctx context.Context, appToken, tableID, recordID string, fields map[string]any) error {
	body := map[string]any{"fields": fields}
	_, err := d.doDriveRequest(ctx, "PUT", fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records/%s", appToken, tableID, recordID), body)
	return err
}

func (d *driveClient) deleteBitableRecord(ctx context.Context, appToken, tableID, recordID string) error {
	_, err := d.doDriveRequest(ctx, "DELETE", fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records/%s", appToken, tableID, recordID), nil)
	return err
}

func formatTableList(tables []*larkbitable.AppTable) string {
	if len(tables) == 0 {
		return "No tables found."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tables (%d):\n", len(tables)))
	for _, t := range tables {
		name := ""
		if t.Name != nil {
			name = *t.Name
		}
		tableID := ""
		if t.TableId != nil {
			tableID = *t.TableId
		}
		sb.WriteString(fmt.Sprintf("  %s (tableId: %s)\n", name, tableID))
	}
	return sb.String()
}

func baseHelp() string {
	return "Base commands:\n" +
		"/base create <name>\n" +
		"/base table-list <URL/token>\n" +
		"/base table-create <URL/token> <name>\n" +
		"/base table-delete <URL/token> <table_id>\n" +
		"/base record-list <URL/token> <table_id>\n" +
		"/base record-create <URL/token> <table_id> <JSON fields>\n" +
		"/base record-update <URL/token> <table_id> <record_id> <JSON fields>\n" +
		"/base record-delete <URL/token> <table_id> <record_id>\n" +
		"/base delete <URL/token>\n" +
		"/base grant <URL/token> <user_open_id> <view|edit|full_access>\n" +
		"/base apply <URL/token> <view|edit> [remark]"
}
