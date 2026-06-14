# Feishu/Lark 云文档命令变更说明

> 本文档汇总了 cc-connect 中 Feishu/Lark 平台云文档相关命令的最新设计与用法。
> 设计对齐官方 Lark CLI 的命名空间：`docs`、`sheets`、`base`。

## 一、变更概览

- 删除了原有的 `/drive` 统一入口命令。
- 拆分为三个独立的资源命令：
  - `/doc`：飞书文档（docx）
  - `/sheet`：电子表格
  - `/base`：多维表格（bitable）
- Sheet 和 Base 操作现在需要显式指定工作表/数据表 ID。
- Sheet 支持 worksheet（工作表）的创建、删除、列出。
- Base 支持 table（数据表）的创建、删除、列出，以及 record（记录）增删改查。
- 创建命令完成后，会自动给当前消息发送者授予 `full_access`（可管理）权限。
- 每个命令空间都支持 `grant`（授权给他人）和 `apply`（向所有者申请权限）。

## 二、命令用法

### `/doc` — 文档

```
/doc create <title>
/doc fetch <docx URL/token>
/doc update <docx URL/token> <content>
/doc delete <docx URL/token>
/doc grant <docx URL/token> <user_open_id> <view|edit|full_access>
/doc apply <docx URL/token> <view|edit> [remark]
```

### `/sheet` — 电子表格

```
/sheet create <title>
/sheet workbook-info <URL/token>
/sheet worksheet-create <URL/token> <title>
/sheet worksheet-delete <URL/token> <sheet_id>
/sheet cells-get <URL/token> <sheet_id> <range>
/sheet cells-set <URL/token> <sheet_id> <range> <JSON values>
/sheet delete <URL/token>
/sheet grant <URL/token> <user_open_id> <view|edit|full_access>
/sheet apply <URL/token> <view|edit> [remark]
```

说明：
- `workbook-info` 列出 spreadsheet 中的所有 worksheet（含 sheet_id、标题、索引）。
- `cells-get` / `cells-set` 的 `range` 使用 A1 格式，例如 `A1:D10`。
- `cells-set` 的 JSON values 为二维数组，例如 `[["A","B"],[1,2]]`。

### `/base` — 多维表格

```
/base create <name>
/base table-list <URL/token>
/base table-create <URL/token> <name>
/base table-delete <URL/token> <table_id>
/base record-list <URL/token> <table_id>
/base record-create <URL/token> <table_id> <JSON fields>
/base record-update <URL/token> <table_id> <record_id> <JSON fields>
/base record-delete <URL/token> <table_id> <record_id>
/base delete <URL/token>
/base grant <URL/token> <user_open_id> <view|edit|full_access>
/base apply <URL/token> <view|edit> [remark]
```

说明：
- `table-list` 列出 base 中的所有数据表。
- record 的 JSON fields 为字段名到字段值的映射，例如 `{"Title":"Task 1","Status":"Done"}`。

## 三、权限行为

### 创建时自动授权

`/doc create`、`/sheet create`、`/base create` 成功后，会尝试自动给当前消息发送者授予 `full_access`（可管理权限）。

- 授权失败不会阻塞文件创建，仅作为提示返回。
- 要求机器人拥有 `docs:permission.member:create` 权限。

### 主动授权给他人

```
/<doc|sheet|base> grant <URL/token> <user_open_id> <view|edit|full_access>
```

- `view`：可查看
- `edit`：可编辑
- `full_access`：可管理（含再授权、删除）

### 向所有者申请权限

```
/<doc|sheet|base> apply <URL/token> <view|edit> [remark]
```

- 用于非所有者向文件所有者申请权限。
- 该接口底层使用 `user_access_token`，但当前实现统一使用 tenant token 调用。
- 如果机器人身份不支持，会返回具体错误。

## 四、URL/token 解析

所有接受 `<URL/token>` 的命令都支持：
- 直接输入文件 token
- 输入飞书/Lark 文件 URL，自动从以下路径提取 token：
  - `/docx/`
  - `/sheets/`
  - `/base/`
  - `/mindnote/`
  - `/file/`

## 五、文件结构

| 文件 | 职责 |
|------|------|
| `platform/feishu/drive.go` | 通用 `driveClient`、文件创建/删除、权限操作、URL/token 解析、原始 HTTP 请求 |
| `platform/feishu/doc.go` | `/doc` 命令实现 |
| `platform/feishu/sheet.go` | `/sheet` 命令实现，含 worksheet 和 cells 操作 |
| `platform/feishu/base.go` | `/base` 命令实现，含 table 和 record 操作 |
| `platform/feishu/feishu.go` | 消息路由，分发 `/doc`、 `/sheet`、 `/base` |

## 六、向后兼容性

- `/drive` 命令已删除。
- `/doc` 命令不再作为 `/drive` 的别名，现在是独立命名空间。
- 原 `/drive sheet read/write` 改为 `/sheet cells-get/cells-set`，且必须指定 `sheet_id`。
- 原 `/drive bitable list/add/update/delete` 改为 `/base record-list/create/update/delete`，且必须指定 `table_id`。

## 七、相关提交

- `cd3215a1` — feat(feishu): align permission handling with Lark CLI
- `ecfd82d6` — refactor(feishu): split /drive into /doc /sheet /base aligned with Lark CLI
- `90f410b5` — feat(feishu): add /drive command suite with sheet and bitable CRUD

## 八、配置要求

机器人需要以下权限范围：

- `docx:document:create`
- `docx:document:readonly`
- `docx:document:write_only`
- `sheets:spreadsheet:create`
- `sheets:spreadsheet:read`
- `sheets:spreadsheet:write_only`
- `base:app:create`
- `base:table:read`
- `base:table:create`
- `base:table:delete`
- `base:record:read`
- `base:record:create`
- `base:record:update`
- `base:record:delete`
- `docs:permission.member:create`
- `docs:permission.member:apply`

## 九、使用示例

创建一个表格并查看工作表：

```
/sheet create 项目进度
/sheet workbook-info <上一步返回的 token>
/sheet worksheet-create <token> Sheet2
/sheet cells-set <token> <sheet_id> A1:B2 [["任务","状态"],["设计","进行中"]]
```

创建一个多维表格并添加记录：

```
/base create 客户管理
/base table-list <token>
/base record-create <token> <table_id> {"客户":"A公司","阶段":"意向"}
```

给同事授权：

```
/doc grant <docx URL> ou_xxxxxxxx full_access
```

向文件所有者申请编辑权限：

```
/sheet apply <URL> edit 需要更新数据
```
