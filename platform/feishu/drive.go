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
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
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

// driveClient wraps cloud file operations on top of a Feishu/Lark platform.
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

// GrantPermission gives the target user a specific permission on a file.
// perm can be "view", "edit", or "full_access".
func (d *driveClient) GrantPermission(ctx context.Context, fileType driveFileType, token, userOpenID, perm string) error {
	if perm == "" {
		perm = "edit"
	}
	req := larkdrive.NewCreatePermissionMemberReqBuilder().
		Token(token).
		Type(string(fileType)).
		BaseMember(larkdrive.NewBaseMemberBuilder().
			MemberType("openid").
			MemberId(userOpenID).
			Perm(perm).
			Build()).
		Build()

	return d.platform.withTransientRetry(ctx, "grant permission", func() error {
		return d.platform.withFreshTenantAccessTokenRetry(ctx, "grant permission", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			resp, err := client.Drive.PermissionMember.Create(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: grant permission api: %w", d.platform.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: grant permission failed code=%d msg=%s", d.platform.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
}

// ApplyPermission requests view or edit permission from the file owner.
// This endpoint requires user_access_token; bot identity is not supported.
func (d *driveClient) ApplyPermission(ctx context.Context, fileType driveFileType, token, perm, remark string) error {
	body := map[string]any{
		"perm": perm,
	}
	if remark != "" {
		body["remark"] = remark
	}
	_, err := d.doDriveRequest(ctx, "POST", fmt.Sprintf("/open-apis/drive/v1/permissions/%s/members/apply?type=%s", token, string(fileType)), body)
	return err
}

// fileURL returns a human-readable URL for a file token and type.
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

// doDriveRequest performs a raw HTTP request against Feishu/Lark APIs.
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

func (d *driveClient) createMindnote(ctx context.Context, title, folderToken string) (string, string, driveFileType, error) {
	token, url, err := d.createDriveFile(ctx, "mindnote", title, folderToken)
	if err != nil {
		return "", "", "", err
	}
	return token, url, driveFileTypeMindnote, nil
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
