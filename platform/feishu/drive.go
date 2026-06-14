package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
	larksheets "github.com/larksuite/oapi-sdk-go/v3/service/sheets/v3"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
)

// driveFileType describes the supported Feishu/Lark cloud file types.
type driveFileType string

const (
	driveFileTypeDoc      driveFileType = "doc"
	driveFileTypeDocx     driveFileType = "docx"
	driveFileTypeSheet    driveFileType = "sheet"
	driveFileTypeBitable  driveFileType = "bitable"
	driveFileTypeMindnote driveFileType = "mindnote"
)

// driveClient wraps cloud file operations on top of a Feishu/Lark client.
type driveClient struct {
	platform *Platform
}

// newDriveClient creates a drive client bound to a platform instance.
func newDriveClient(p *Platform) *driveClient {
	return &driveClient{platform: p}
}

// CreateFile creates a new cloud file of the given type and title.
// Returns the file token, a human-readable URL, and the canonical type.
func (d *driveClient) CreateFile(ctx context.Context, ft driveFileType, title, folderToken string) (string, string, driveFileType, error) {
	switch ft {
	case driveFileTypeDoc, driveFileTypeDocx:
		return d.createDocx(ctx, title, folderToken)
	case driveFileTypeSheet:
		return d.createSheet(ctx, title, folderToken)
	case driveFileTypeBitable:
		return d.createBitable(ctx, title, folderToken)
	case driveFileTypeMindnote:
		return d.createMindnote(ctx, title, folderToken)
	default:
		return "", "", "", fmt.Errorf("%s: unsupported drive file type: %s", d.platform.tag(), ft)
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

func (d *driveClient) createBitable(ctx context.Context, title, folderToken string) (string, string, driveFileType, error) {
	req := larkbitable.NewCreateAppReqBuilder().
		ReqApp(larkbitable.NewReqAppBuilder().
			Name(title).
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

func (d *driveClient) createMindnote(ctx context.Context, title, folderToken string) (string, string, driveFileType, error) {
	token, url, err := d.createDriveFile(ctx, "mindnote", title, folderToken)
	if err != nil {
		return "", "", "", err
	}
	return token, url, driveFileTypeMindnote, nil
}

// ReadSheet returns the values of the first sheet in a spreadsheet.
func (d *driveClient) ReadSheet(ctx context.Context, token string) (string, error) {
	sheets, err := d.listSheets(ctx, token)
	if err != nil {
		return "", err
	}
	if len(sheets) == 0 {
		return "", fmt.Errorf("%s: spreadsheet has no sheets", d.platform.tag())
	}
	sheet := sheets[0]
	sheetID := sheet.SheetID
	if sheetID == "" {
		return "", fmt.Errorf("%s: first sheet has no id", d.platform.tag())
	}

	values, err := d.getSheetValues(ctx, token, sheetID+"!A1:Z1000")
	if err != nil {
		return "", err
	}
	if len(values) == 0 {
		return "(empty sheet)", nil
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

// WriteSheet writes values to a spreadsheet range.
// Values are parsed from a JSON array of arrays, or a simple tabular format.
func (d *driveClient) WriteSheet(ctx context.Context, token, sheetRange string, values [][]any) error {
	body := map[string]any{
		"valueRange": map[string]any{
			"range":  sheetRange,
			"values": values,
		},
	}
	_, err := d.doDriveRequest(ctx, "PUT", fmt.Sprintf("/open-apis/sheets/v2/spreadsheets/%s/values", token), body)
	return err
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

// ListBitableRecords lists records from the first table of a bitable app.
func (d *driveClient) ListBitableRecords(ctx context.Context, appToken string) (string, error) {
	tables, err := d.listBitableTables(ctx, appToken)
	if err != nil {
		return "", err
	}
	if len(tables) == 0 {
		return "", fmt.Errorf("%s: bitable has no tables", d.platform.tag())
	}
	tableID := ""
	if tables[0].TableId != nil {
		tableID = *tables[0].TableId
	}
	if tableID == "" {
		return "", fmt.Errorf("%s: first table has no id", d.platform.tag())
	}

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

// CreateBitableRecord adds a record to the first table of a bitable app.
func (d *driveClient) CreateBitableRecord(ctx context.Context, appToken string, fields map[string]any) (string, error) {
	tables, err := d.listBitableTables(ctx, appToken)
	if err != nil {
		return "", err
	}
	if len(tables) == 0 {
		return "", fmt.Errorf("%s: bitable has no tables", d.platform.tag())
	}
	tableID := ""
	if tables[0].TableId != nil {
		tableID = *tables[0].TableId
	}
	if tableID == "" {
		return "", fmt.Errorf("%s: first table has no id", d.platform.tag())
	}

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

// UpdateBitableRecord updates a record in the first table of a bitable app.
func (d *driveClient) UpdateBitableRecord(ctx context.Context, appToken, recordID string, fields map[string]any) error {
	tables, err := d.listBitableTables(ctx, appToken)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return fmt.Errorf("%s: bitable has no tables", d.platform.tag())
	}
	tableID := ""
	if tables[0].TableId != nil {
		tableID = *tables[0].TableId
	}
	if tableID == "" {
		return fmt.Errorf("%s: first table has no id", d.platform.tag())
	}

	body := map[string]any{"fields": fields}
	_, err = d.doDriveRequest(ctx, "PUT", fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records/%s", appToken, tableID, recordID), body)
	return err
}

// DeleteBitableRecord deletes a record from the first table of a bitable app.
func (d *driveClient) DeleteBitableRecord(ctx context.Context, appToken, recordID string) error {
	tables, err := d.listBitableTables(ctx, appToken)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return fmt.Errorf("%s: bitable has no tables", d.platform.tag())
	}
	tableID := ""
	if tables[0].TableId != nil {
		tableID = *tables[0].TableId
	}
	if tableID == "" {
		return fmt.Errorf("%s: first table has no id", d.platform.tag())
	}

	_, err = d.doDriveRequest(ctx, "DELETE", fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records/%s", appToken, tableID, recordID), nil)
	return err
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

// doDriveRequest performs a raw HTTP request against Feishu/Lark drive APIs.
func (d *driveClient) doDriveRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyJSON []byte
	var err error
	if body != nil {
		bodyJSON, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%s: marshal request body: %w", d.platform.tag(), err)
		}
	}

	var respBody []byte
	err = d.platform.withTransientRetry(ctx, "drive request", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "drive request", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			tokenResp, err := client.GetTenantAccessTokenBySelfBuiltApp(ctx, &larkcore.SelfBuiltTenantAccessTokenReq{
				AppID:     d.platform.appID,
				AppSecret: d.platform.appSecret,
			})
			if err != nil {
				return fmt.Errorf("%s: get tenant access token: %w", d.platform.tag(), err)
			}
			if !tokenResp.Success() {
				return fmt.Errorf("%s: get tenant access token code=%d msg=%s", d.platform.tag(), tokenResp.Code, tokenResp.Msg)
			}

			baseURL := "https://open.feishu.cn"
			if d.platform.platformName == "lark" {
				baseURL = "https://open.larksuite.com"
			}

			var bodyReader io.Reader
			if len(bodyJSON) > 0 {
				bodyReader = bytes.NewReader(bodyJSON)
			}
			req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
			if err != nil {
				return fmt.Errorf("%s: build %s request: %w", d.platform.tag(), method, err)
			}
			req.Header.Set("Authorization", "Bearer "+tokenResp.TenantAccessToken)
			if method != http.MethodGet && method != http.MethodDelete {
				req.Header.Set("Content-Type", "application/json; charset=utf-8")
			}

			httpResp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("%s: %s request: %w", d.platform.tag(), method, err)
			}
			defer httpResp.Body.Close()

			respBody, err = io.ReadAll(httpResp.Body)
			if err != nil {
				return fmt.Errorf("%s: read response: %w", d.platform.tag(), err)
			}

			var result struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
			}
			if err := json.Unmarshal(respBody, &result); err == nil && result.Code != 0 {
				return fmt.Errorf("%s: api failed code=%d msg=%s", d.platform.tag(), result.Code, result.Msg)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

// createDriveFile calls POST /open-apis/drive/v1/files for file types not covered by dedicated SDKs.
func (d *driveClient) createDriveFile(ctx context.Context, fileType, title, folderToken string) (string, string, error) {
	body := map[string]any{
		"type": fileType,
		"name": title,
	}
	if folderToken != "" {
		body["folder_token"] = folderToken
	}

	respBody, err := d.doDriveRequest(ctx, "POST", "/open-apis/drive/v1/files", body)
	if err != nil {
		return "", "", err
	}

	var result struct {
		Data *struct {
			Token string `json:"token"`
			URL   string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("%s: parse create drive file response: %w", d.platform.tag(), err)
	}
	if result.Data == nil || result.Data.Token == "" {
		return "", "", fmt.Errorf("%s: create drive file: no token returned", d.platform.tag())
	}

	url := result.Data.URL
	if url == "" {
		url = d.fileURL(driveFileTypeMindnote, result.Data.Token)
	}
	return result.Data.Token, url, nil
}

// GetDocumentContent fetches the raw text content of a docx document.
func (d *driveClient) GetDocumentContent(ctx context.Context, docToken string) (string, error) {
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

// UpdateDocumentBlocks replaces the docx body with paragraphs built from text.
func (d *driveClient) UpdateDocumentBlocks(ctx context.Context, docToken string, content string) error {
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

// DeleteFile moves a cloud file to the recycle bin.
func (d *driveClient) DeleteFile(ctx context.Context, fileType driveFileType, token string) error {
	req := larkdrive.NewDeleteFileReqBuilder().
		FileToken(token).
		Type(string(fileType)).
		Build()

	return d.platform.withTransientRetry(ctx, "delete file", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "delete file", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			resp, err := client.Drive.File.Delete(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: delete file api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: delete file failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
}

// GrantEditPermission gives the target user edit access to a file.
func (d *driveClient) GrantEditPermission(ctx context.Context, fileType driveFileType, token, userOpenID string) error {
	req := larkdrive.NewCreatePermissionMemberReqBuilder().
		Token(token).
		Type(string(fileType)).
		BaseMember(larkdrive.NewBaseMemberBuilder().
			MemberType("openid").
			MemberId(userOpenID).
			Perm("edit").
			Build()).
		Build()

	return d.platform.withTransientRetry(ctx, "grant edit permission", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "grant edit permission", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			resp, err := client.Drive.PermissionMember.Create(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: grant edit permission api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: grant edit permission failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
}

func (d *driveClient) fileURL(ft driveFileType, token string) string {
	base := "https://www.feishu.cn"
	if d.platform.platformName == "lark" {
		base = "https://www.larksuite.com"
	}
	var path string
	switch ft {
	case driveFileTypeSheet:
		path = "sheets"
	case driveFileTypeBitable:
		path = "base"
	case driveFileTypeMindnote:
		path = "mindnote"
	default:
		path = "docx"
	}
	return fmt.Sprintf("%s/%s/%s", base, path, token)
}

// parseFileToken extracts a file token from a Feishu/Lark URL or returns the token as-is.
func parseFileToken(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	for _, prefix := range []string{"/docx/", "/sheets/", "/base/", "/mindnote/", "/file/"} {
		if idx := strings.LastIndex(input, prefix); idx != -1 {
			candidate := input[idx+len(prefix):]
			candidate = strings.Split(candidate, "?")[0]
			candidate = strings.Split(candidate, "#")[0]
			candidate = strings.TrimRight(candidate, "/")
			return candidate
		}
	}

	return input
}

// parseDriveFileType normalizes a user-typed type string.
func parseDriveFileType(s string) (driveFileType, bool) {
	switch strings.ToLower(s) {
	case "doc", "docx":
		return driveFileTypeDocx, true
	case "sheet", "sheets", "excel":
		return driveFileTypeSheet, true
	case "bitable", "base", "多维表格":
		return driveFileTypeBitable, true
	case "mindnote", "mind", "思维笔记":
		return driveFileTypeMindnote, true
	default:
		return "", false
	}
}

// handleDriveCommand parses /drive subcommands and executes them.
func (p *Platform) handleDriveCommand(ctx context.Context, rctx replyContext, text, userID string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}

	// /doc remains a shortcut for /drive doc ...
	if fields[0] == "/doc" {
		fields[0] = "/drive"
		fields = append([]string{"/drive", "doc"}, fields[1:]...)
	}

	if fields[0] != "/drive" {
		return false
	}

	client := newDriveClient(p)

	if len(fields) < 2 {
		p.Reply(ctx, rctx, driveHelp())
		return true
	}

	sub := strings.ToLower(fields[1])

	switch sub {
	case "create":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Please specify file type and title, e.g. /drive create doc My Notes")
			return true
		}
		ft, ok := parseDriveFileType(fields[2])
		if !ok {
			p.Reply(ctx, rctx, fmt.Sprintf("Unsupported file type: %s\n\n%s", fields[2], driveHelp()))
			return true
		}
		title := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/drive create %s", fields[2])))
		if title == "" {
			title = "Untitled"
		}
		token, url, actualType, err := client.CreateFile(ctx, ft, title, "")
		if err != nil {
			p.Reply(ctx, rctx, "Create file failed: "+err.Error())
			return true
		}

		shareMsg := ""
		if userID != "" {
			if err := client.GrantEditPermission(ctx, actualType, token, userID); err != nil {
				shareMsg = fmt.Sprintf("\nNote: failed to grant you edit permission (%s).", err.Error())
			} else {
				shareMsg = "\nYou have been granted edit permission."
			}
		}

		p.Reply(ctx, rctx, fmt.Sprintf("Created %s: %s\nURL: %s\nToken: %s%s", actualType, title, url, token, shareMsg))
		return true

	case "read":
		if len(fields) < 3 {
			p.Reply(ctx, rctx, "Please provide a docx URL or token, e.g. /drive read <URL>")
			return true
		}
		docToken := parseFileToken(fields[2])
		content, err := client.GetDocumentContent(ctx, docToken)
		if err != nil {
			p.Reply(ctx, rctx, "Read document failed: "+err.Error())
			return true
		}
		if len(content) > 4000 {
			content = content[:4000] + "\n\n...(truncated)"
		}
		p.Reply(ctx, rctx, content)
		return true

	case "update":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Please provide a docx URL/token and content, e.g. /drive update <URL> hello")
			return true
		}
		docToken := parseFileToken(fields[2])
		content := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/drive update %s", fields[2])))
		if content == "" {
			p.Reply(ctx, rctx, "Update content cannot be empty")
			return true
		}
		if err := client.UpdateDocumentBlocks(ctx, docToken, content); err != nil {
			p.Reply(ctx, rctx, "Update document failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "Document updated: "+client.fileURL(driveFileTypeDocx, docToken))
		return true

	case "delete":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Please provide file type and URL/token, e.g. /drive delete doc <URL>")
			return true
		}
		ft, ok := parseDriveFileType(fields[2])
		if !ok {
			p.Reply(ctx, rctx, fmt.Sprintf("Unsupported file type: %s\n\n%s", fields[2], driveHelp()))
			return true
		}
		token := parseFileToken(fields[3])
		if err := client.DeleteFile(ctx, ft, token); err != nil {
			p.Reply(ctx, rctx, "Delete file failed: "+err.Error())
			return true
		}
		p.Reply(ctx, rctx, "File deleted (moved to recycle bin)")
		return true

	case "sheet":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage:\n/drive sheet read <URL/token>\n/drive sheet write <URL/token> <range> <JSON values>")
			return true
		}
		token := parseFileToken(fields[3])
		switch strings.ToLower(fields[2]) {
		case "read":
			content, err := client.ReadSheet(ctx, token)
			if err != nil {
				p.Reply(ctx, rctx, "Read sheet failed: "+err.Error())
				return true
			}
			if len(content) > 4000 {
				content = content[:4000] + "\n\n...(truncated)"
			}
			p.Reply(ctx, rctx, content)
			return true

		case "write":
			if len(fields) < 6 {
				p.Reply(ctx, rctx, "Usage: /drive sheet write <URL/token> <range> <JSON values>")
				return true
			}
			sheetRange := fields[4]
			valueJSON := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/drive sheet write %s %s %s", fields[3], fields[4], fields[5])))
			if valueJSON == "" {
				valueJSON = strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/drive sheet write %s %s", fields[3], fields[4])))
			}
			if valueJSON == "" {
				p.Reply(ctx, rctx, "Values cannot be empty")
				return true
			}
			var values [][]any
			if err := json.Unmarshal([]byte(valueJSON), &values); err != nil {
				p.Reply(ctx, rctx, "Invalid JSON values: "+err.Error())
				return true
			}
			if err := client.WriteSheet(ctx, token, sheetRange, values); err != nil {
				p.Reply(ctx, rctx, "Write sheet failed: "+err.Error())
				return true
			}
			p.Reply(ctx, rctx, "Sheet updated")
			return true

		default:
			p.Reply(ctx, rctx, "Usage:\n/drive sheet read <URL/token>\n/drive sheet write <URL/token> <range> <JSON values>")
			return true
		}

	case "bitable":
		if len(fields) < 4 {
			p.Reply(ctx, rctx, "Usage:\n/drive bitable list <URL/token>\n/drive bitable add <URL/token> <JSON fields>\n/drive bitable update <URL/token> <record_id> <JSON fields>\n/drive bitable delete <URL/token> <record_id>")
			return true
		}
		token := parseFileToken(fields[3])
		switch strings.ToLower(fields[2]) {
		case "list":
			content, err := client.ListBitableRecords(ctx, token)
			if err != nil {
				p.Reply(ctx, rctx, "List bitable records failed: "+err.Error())
				return true
			}
			if len(content) > 4000 {
				content = content[:4000] + "\n\n...(truncated)"
			}
			p.Reply(ctx, rctx, content)
			return true

		case "add":
			jsonStr := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/drive bitable add %s", fields[3])))
			if jsonStr == "" {
				p.Reply(ctx, rctx, "Fields JSON cannot be empty")
				return true
			}
			var fieldsMap map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &fieldsMap); err != nil {
				p.Reply(ctx, rctx, "Invalid JSON fields: "+err.Error())
				return true
			}
			recordID, err := client.CreateBitableRecord(ctx, token, fieldsMap)
			if err != nil {
				p.Reply(ctx, rctx, "Add bitable record failed: "+err.Error())
				return true
			}
			p.Reply(ctx, rctx, "Record added: "+recordID)
			return true

		case "update":
			if len(fields) < 6 {
				p.Reply(ctx, rctx, "Usage: /drive bitable update <URL/token> <record_id> <JSON fields>")
				return true
			}
			recordID := fields[4]
			jsonStr := strings.TrimSpace(strings.TrimPrefix(text, fmt.Sprintf("/drive bitable update %s %s", fields[3], recordID)))
			if jsonStr == "" {
				p.Reply(ctx, rctx, "Fields JSON cannot be empty")
				return true
			}
			var fieldsMap map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &fieldsMap); err != nil {
				p.Reply(ctx, rctx, "Invalid JSON fields: "+err.Error())
				return true
			}
			if err := client.UpdateBitableRecord(ctx, token, recordID, fieldsMap); err != nil {
				p.Reply(ctx, rctx, "Update bitable record failed: "+err.Error())
				return true
			}
			p.Reply(ctx, rctx, "Record updated")
			return true

		case "delete":
			if len(fields) < 5 {
				p.Reply(ctx, rctx, "Usage: /drive bitable delete <URL/token> <record_id>")
				return true
			}
			recordID := fields[4]
			if err := client.DeleteBitableRecord(ctx, token, recordID); err != nil {
				p.Reply(ctx, rctx, "Delete bitable record failed: "+err.Error())
				return true
			}
			p.Reply(ctx, rctx, "Record deleted")
			return true

		default:
			p.Reply(ctx, rctx, "Usage:\n/drive bitable list <URL/token>\n/drive bitable add <URL/token> <JSON fields>\n/drive bitable update <URL/token> <record_id> <JSON fields>\n/drive bitable delete <URL/token> <record_id>")
			return true
		}

	default:
		p.Reply(ctx, rctx, driveHelp())
		return true
	}
}

func driveHelp() string {
	return "Feishu cloud drive commands:\n" +
		"/drive create doc <title>       Create a docx document\n" +
		"/drive create sheet <title>     Create a spreadsheet\n" +
		"/drive create bitable <title>   Create a bitable\n" +
		"/drive create mindnote <title>  Create a mindnote\n" +
		"/drive read <docx URL/token>    Read docx content\n" +
		"/drive update <docx URL/token> <content>  Update docx (overwrite)\n" +
		"/drive delete <type> <URL/token>  Delete file\n" +
		"/drive sheet read <URL/token>          Read first sheet\n" +
		"/drive sheet write <URL/token> <range> <JSON values>  Write cells\n" +
		"/drive bitable list <URL/token>         List first table records\n" +
		"/drive bitable add <URL/token> <JSON fields>  Add record\n" +
		"/drive bitable update <URL/token> <record_id> <JSON fields>  Update record\n" +
		"/drive bitable delete <URL/token> <record_id>  Delete record\n" +
		"/doc ...                        Shortcut for /drive doc ..."
}

// handleDocCommand retains backward compatibility for /doc commands.
func (p *Platform) handleDocCommand(ctx context.Context, rctx replyContext, text, userID string) bool {
	return p.handleDriveCommand(ctx, rctx, text, userID)
}
