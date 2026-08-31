# DevEnglish product MCP — client handoff

Trạng thái: transport, auth, scope, tool/resource discovery và REST parity đã
có bằng chứng local. Việc cấu hình và chạy thử từ Codex/Claude trên máy của
operator vẫn là một acceptance gate riêng.

## 1. Phân biệt hai MCP server

Repo hiện có hai bề mặt không được trộn lẫn:

1. **CyberOS MCP**: các file `.mcp.json`, `.codex/config.toml` và
   `.agents/mcp_config.json` đang trỏ tới server stdio
   `.cyberos/mcp/cyberos-mcp.mjs`. Server này phục vụ governance của repo.
2. **DevEnglish product MCP**: backend application expose Streamable HTTP tại
   `<APP_ORIGIN>/mcp`. Đây là server để Codex/Claude đọc Work/Knowledge và gọi
   application services. Nó dùng bearer token sản phẩm riêng.

Không ghi đè cấu hình CyberOS để trỏ sang product MCP. Hãy thêm một entry
riêng ở cấu hình client cá nhân hoặc client profile được tin cậy.

## 2. Endpoint, scope và capability

Ví dụ local trên cùng máy:

    http://127.0.0.1:8080/mcp

Ví dụ qua HTTPS/reverse proxy:

    https://english.example.com/mcp

Token nên cấp ít quyền nhất:

| Scope | Capability |
| --- | --- |
| `assistant:use` | `assistant_ask` |
| `knowledge:read` | `knowledge_search`, `knowledge_get` |
| `work:read` | `project_list`, `task_list`, `decision_get` và các resource Work |
| `work:write` | `project_create`, `task_upsert`, `decision_record`, `knowledge_import_manual`, `entity_trash` |
| `github:write` | `github_issue_create`, `github_issue_comment`, `github_issue_label` sau challenge |

Các tên có dấu gạch dưới là V1 contract ổn định cho Codex/Claude. Một số tên
dotted còn được giữ như compatibility alias cho client cũ; client mới nên
dùng tên gạch dưới.

Resource hiện tại:

- `devenglish://work/projects`
- `devenglish://work/tasks`
- `devenglish://work/decisions`

V1 không cấp Drive write, GitHub push/merge/PR mutation hoặc autonomous action.

## 3. Cấp token an toàn

Token được cấp bởi `POST /api/v2/mcp/tokens`. Response có `id`, `token`,
`scopes` và `expiresAt`; giá trị `token` chỉ được reveal một lần. Backend lưu
digest, không lưu plaintext. TTL mặc định là 90 ngày.

### Local development non-strict

Khi backend đang chạy non-strict và dùng single-user development workspace,
có thể cấp read-only token như sau. File tạm được tạo với permission riêng và
không được in ra terminal:

~~~bash
export DEVENGLISH_URL=http://127.0.0.1:8080
umask 077
TOKEN_FILE="$(mktemp)"
curl -fsS -X POST "$DEVENGLISH_URL/api/v2/mcp/tokens" \
  -H 'Content-Type: application/json' \
  --data '{"scopes":["assistant:use","knowledge:read","work:read"]}' \
  >"$TOKEN_FILE"
export DEVENGLISH_MCP_TOKEN="$(jq -er .token "$TOKEN_FILE")"
export DEVENGLISH_MCP_TOKEN_ID="$(jq -er .id "$TOKEN_FILE")"
rm -f "$TOKEN_FILE"
~~~

Nếu local server đã bật strict auth, dùng một web session đã xác thực hoặc
flow production bên dưới. Không dùng login secret trong frontend.

### Production/strict auth qua HTTPS

Cookie login của production là `Secure`, vì vậy flow này phải chạy qua HTTPS:

~~~bash
export DEVENGLISH_URL=https://english.example.com
umask 077
COOKIE_JAR="$(mktemp)"
TOKEN_FILE="$(mktemp)"
trap 'rm -f "$COOKIE_JAR" "$TOKEN_FILE"' EXIT

curl -fsS -c "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -X POST "$DEVENGLISH_URL/api/v1/auth/login" \
  --data "{\"secret\":\"$DEVENGLISH_LOGIN_SECRET\"}" \
  >/dev/null

curl -fsS -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -X POST "$DEVENGLISH_URL/api/v2/mcp/tokens" \
  --data '{"scopes":["assistant:use","knowledge:read","work:read"]}' \
  >"$TOKEN_FILE"

export DEVENGLISH_MCP_TOKEN="$(jq -er .token "$TOKEN_FILE")"
export DEVENGLISH_MCP_TOKEN_ID="$(jq -er .id "$TOKEN_FILE")"
~~~

`DEVENGLISH_LOGIN_SECRET` chỉ nằm trong shell secret manager hoặc môi trường
được bảo vệ. Không commit token, cookie jar, response file hay secret vào repo.

## 4. Kiểm tra transport trước khi gắn client

Mọi MCP POST hiện tại phải gửi cả hai media type trong `Accept`, vì server có
thể trả JSON hoặc SSE theo negotiation:

~~~bash
curl -fsS -X POST "$DEVENGLISH_URL/mcp" \
  -H "Authorization: Bearer $DEVENGLISH_MCP_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'MCP-Protocol-Version: 2025-06-18' \
  --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"operator-smoke","version":"1"}}}' \
  | jq '{jsonrpc,id,result:{protocolVersion,serverInfo,capabilities}}'
~~~

Kết quả cần có HTTP `200`, JSON-RPC `2.0`, `result.protocolVersion`,
`serverInfo` và `capabilities`. Header `MCP-Protocol-Version` là optional,
nhưng nếu gửi thì phải là version server hỗ trợ.

Liệt kê tools/resources mà không in bearer token:

~~~bash
curl -fsS -X POST "$DEVENGLISH_URL/mcp" \
  -H "Authorization: Bearer $DEVENGLISH_MCP_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | jq -r '.result.tools[].name'

curl -fsS -X POST "$DEVENGLISH_URL/mcp" \
  -H "Authorization: Bearer $DEVENGLISH_MCP_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}' \
  | jq -r '.result.resources[].uri'
~~~

Read-only smoke cho assistant:

~~~bash
curl -fsS -X POST "$DEVENGLISH_URL/mcp" \
  -H "Authorization: Bearer $DEVENGLISH_MCP_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"assistant_ask","arguments":{"message":"What is currently known about my active work?"}}}' \
  | jq '{jsonrpc,id,error,result}'
~~~

Assistant phải giữ evidence/unknown/stale semantics. Nếu không có evidence,
response phải nói rõ unknown hoặc inferred; không coi câu trả lời tự sinh là
canonical Work/Knowledge.

## 5. Cấu hình Codex

Theo [Codex MCP documentation](https://learn.chatgpt.com/docs/extend/mcp?surface=cli),
client Codex local dùng Streamable HTTP trực tiếp. Theo [Codex configuration
reference](https://learn.chatgpt.com/docs/config-file/config-reference),
bearer token nên lấy từ environment thay vì ghi plaintext vào TOML.

Thêm entry sau vào user-level `~/.codex/config.toml`, hoặc trusted project
config riêng cho product client. Không thay thế table `mcp_servers.cyberos`
đang có trong repo:

Mẫu copy-only đã được lưu tại
[`docs/project/mcp/codex-product-mcp.toml.example`](mcp/codex-product-mcp.toml.example).
Mẫu này chỉ bật read-only tools, trỏ local vào `127.0.0.1`, và đọc bearer token
từ environment; không phải active config và không chứa token. Với checkout này,
hãy merge mẫu vào user-level config thay vì sửa `.codex/config.toml` trong repo,
vì file đó đang dành cho CyberOS governance.

~~~toml
[mcp_servers.devenglish]
url = "https://english.example.com/mcp"
bearer_token_env_var = "DEVENGLISH_MCP_TOKEN"
enabled_tools = [
  "assistant_ask",
  "knowledge_search",
  "knowledge_get",
  "project_list",
  "task_list",
  "decision_get",
]
default_tools_approval_mode = "prompt"
startup_timeout_sec = 10
tool_timeout_sec = 60
~~~

Trước khi mở/restart Codex, export `DEVENGLISH_MCP_TOKEN` trong môi trường mà
client desktop/CLI nhận được. Sau đó kiểm tra:

~~~bash
codex mcp list
~~~

Trong TUI có thể mở `/mcp` để xem server, tools và lỗi khởi động. Nếu chỉ
muốn đọc dữ liệu, giữ `enabled_tools` như trên. Chỉ thêm các write tools sau
khi đã test challenge/receipt ở mục 7.

## 6. Cấu hình Claude

Trong Claude client, chọn **remote/custom Streamable HTTP MCP server** (tên
menu có thể khác theo bản client), nhập URL product `/mcp`, và cấu hình header:

    Authorization: Bearer <DEVENGLISH_MCP_TOKEN từ secret manager>

Không dán token vào repo, prompt, screenshot hoặc file `.mcp.json` đã tracked.
Không trỏ Claude vào `.cyberos/mcp/cyberos-mcp.mjs` nếu mục tiêu là đọc dữ liệu
DevEnglish; đó là governance server stdio. Nếu client Claude yêu cầu file
config thay vì UI, chỉ dùng transport shape tương đương gồm URL HTTP và
Authorization bearer header, rồi kiểm tra lại bằng `initialize`/`tools/list`
ở mục 4 theo đúng version client đang cài.

## 7. Quy tắc khi gọi write tools

- Bắt đầu bằng token chỉ có `assistant:use`, `knowledge:read`,
  `work:read`.
- `work:write` yêu cầu `Idempotency-Key`; update yêu cầu
  `expectedVersion`, conflict trả `409`/MCP conflict và không tự ghi đè.
- Mutation MCP cần nonce/replay metadata theo protocol. Dùng nonce mới cho
  mỗi request, không retry mù một mutation.
- `entity_trash` cần challenge đã preview, đúng target và expected version.
- GitHub issue/comment/label cần `github:write` và challenge TTL 5 phút; retry
  hợp lệ trả receipt cũ, không tạo mutation thứ hai.
- Drive vẫn read-only. Không có công cụ push code, merge PR hoặc thay credential.

Không cấp tất cả scope vào cùng một token chỉ để tiện thử. Muốn tăng quyền,
issue token mới với scope cụ thể và revoke token cũ nếu không còn dùng.

## 8. Revoke và replay check

Revoke bằng web session của đúng user/workspace, không dùng MCP bearer token
để quản lý token:

~~~bash
curl -fsS -X DELETE \
  "$DEVENGLISH_URL/api/v2/mcp/tokens/$DEVENGLISH_MCP_TOKEN_ID" \
  -b "$COOKIE_JAR"
~~~

Kết quả mong đợi là HTTP `204`. Sau đó request MCP với token cũ phải trả
HTTP `401`:

~~~bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  -X POST "$DEVENGLISH_URL/mcp" \
  -H "Authorization: Bearer $DEVENGLISH_MCP_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":5,"method":"ping"}'
~~~

Không xóa token cũ khỏi biến môi trường trước bước này; sau khi kiểm tra xong
hãy `unset DEVENGLISH_MCP_TOKEN DEVENGLISH_MCP_TOKEN_ID`.

## 9. Network boundary

- Codex chạy cùng Mac với backend: dùng `127.0.0.1`, không dùng địa chỉ LAN.
- Thiết bị hoặc máy khác mạng: dùng hostname HTTPS qua Tailscale hoặc reverse
  proxy có firewall. `localhost` trên điện thoại là chính điện thoại, không
  phải Mac.
- Không expose cổng backend thô ra Internet chỉ vì đã bật CORS. CORS không thay
  thế bearer auth, TLS, firewall và token revocation.
- Khi đổi từ local sang host thật, cập nhật `DEVENGLISH_ALLOWED_ORIGINS` cho
  web origin và chạy lại initialize, tool/resource discovery, revoke/401.

## 10. Operator acceptance checklist

Ghi lại ngày, client/version, URL (không ghi token) và kết quả vào
`docs/project/ux/OPERATOR_VERDICT.md`:

- [ ] Issue read-only token thành công; response không bị cache/log plaintext.
- [ ] `initialize` trả `200`; `tools/list` và `resources/list` đúng scope.
- [ ] `assistant_ask` trả evidence/unknown/stale đúng contract.
- [ ] Một request thiếu scope bị từ chối; không có tool vượt scope.
- [ ] Không bật write tools trước khi challenge/idempotency được kiểm tra.
- [ ] Revoke trả `204`; token cũ trả `401`.
- [ ] Codex kết nối được product MCP mà không làm mất CyberOS MCP.
- [ ] Claude kết nối được product MCP bằng URL/header đúng.

Evidence local hiện tại gồm official MCP Go SDK conformance `62/62`,
production-shaped Docker smoke với MCP issue/initialize/revoke/revoked
`201/200/204/401`, và các test scope/replay/isolation. Các checkbox phía trên
vẫn cần operator chạy từ client thực tế; không suy ra acceptance chỉ từ
server-side test.
