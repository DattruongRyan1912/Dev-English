# DevEnglish Product Reset — End-to-End Delivery and UI Rebuild

**Revision:** 2.0 — source-verified rewrite
**Status:** IMPLEMENTATION EVIDENCE COMPLETE LOCALLY — acceptance/release gates open; chưa phải task lifecycle state
**Baseline được kiểm tra:** remote `main@104304f12f411dd534aa4b385e964b3b8c11ae43`, ngày 2026-08-27
**Phạm vi:** hợp nhất backend/client, thay application shell và toàn bộ trải nghiệm người dùng
**Cách thực thi:** Luna Max ở luồng chính triển khai; tối đa một Sol Ultra read-only reviewer tại mỗi review gate

**Trạng thái thực thi hiện tại (2026-08-29):** implementation evidence cho S1–S7
đã được ghi nhận trong `docs/project/STATUS.md` và
`docs/project/agent-runs/2026-08-29-acceptance-audit.md`. Các mục còn mở là
acceptance/release boundary, không được suy diễn là đã hoàn tất từ local build.

> Đây là product reset, không phải Wave 6. Mục tiêu không phải tạo thêm package hoặc màn preview; mỗi delivery slice phải tạo ra một luồng người dùng chạy thật từ PostgreSQL → application service → REST/UI, có kiểm thử và runtime evidence.

## 1. Kết quả sản phẩm cần đạt

DevEnglish trở thành **AI work assistant cho developer, có English learning overlay thích ứng**.

English là lớp hỗ trợ được rút ra từ công việc thật. Người dùng có thể làm việc chủ yếu bằng tiếng Việt, nhìn thấy và thực hành English đúng ngữ cảnh mà không bị ép chuyển sang một “khóa học” riêng.

Luồng giá trị cốt lõi:

```text
Capture hoặc import công việc
        ↓
Project → Task → Decision
        ↓
Knowledge có source → revision → evidence
        ↓
Assistant 1:1 trả lời có grounding và citation
        ↓
Action được preview → confirm → receipt
        ↓
English overlay rút ra từ chính workflow vừa xảy ra
```

### 1.1. Những việc app phải giải quyết

- Tóm tắt hôm nay cần làm gì, việc nào ưu tiên và dữ liệu nào hỗ trợ quyết định đó.
- Quản lý Project, Task và Decision theo workspace.
- Thu thập, đồng bộ và tìm lại knowledge từ manual input, Drive và GitHub.
- Hỏi AI 1:1 bằng tiếng Việt hoặc tiếng Anh, luôn giữ giao diện chữ và lịch sử hội thoại.
- Phân biệt rõ fact, inference, unknown và stale source.
- Đề xuất action nhưng không tự ý thay đổi dữ liệu hoặc hệ thống bên ngoài.
- Học cách diễn đạt bằng tiếng Anh từ task, decision, stand-up, bug report và technical discussion thật.
- Dùng chung dữ liệu và semantics qua Flutter, REST và MCP cho Codex/Claude.

### 1.2. V1 không làm

- Dynamic plugin marketplace, WASM hoặc Go runtime plugin.
- Full SaaS/RBAC nhiều tổ chức.
- Drive write.
- GitHub push, merge, code mutation hoặc thay đổi PR state.
- Autonomous external action không confirmation.
- Realtime full-duplex voice.
- OCR/scanned-document ingestion.
- Xóa compatibility endpoints trước khi migration và rollback đã được chứng minh.

## 2. Baseline đã kiểm chứng

Plan cũ đã trộn ba trạng thái khác nhau. Revision này tách chúng rõ ràng.

| Nguồn | Trạng thái thật | Ý nghĩa |
|---|---|---|
| Remote `main@104304f` | Đã chứa migrations `003`–`006` và packages `work`, `knowledge`, `connectors`, `mcp`, `assistant` | Foundation đã được merge; không xây lại từ đầu |
| Current checkout `chore/rc1-timeout-browser-smoke@d39019a` | Cũ hơn remote main và đang có nhiều tracked/untracked WIP | Không được reset, copy mù hoặc coi là integration base |
| `integration/wave-2@ea3bd05` | Nguồn lịch sử của Work/Knowledge/Connectors/MCP foundation | Chỉ dùng để truy vết evidence; remote main là nguồn hợp nhất mới hơn |
| `integration/wave-3@228d356` | Có assistant grounding, MCP application adapter, workspace preview và tests | Phần lớn đã vào remote main qua squash; không tiếp tục phát triển độc lập |

### 2.1. Những gì đã có trên remote main

- Work service/repository/types và optimistic-concurrency foundation.
- Knowledge service/repository/types, source/revision/chunk/claim/evidence foundation.
- Connector contracts, read sync và safe-write primitives.
- MCP auth/scope/replay/registry và read-only application adapter.
- Assistant grounding boundary: grounded/inferred/unknown, verified citation và suggested-action boundary.
- Workspace models, demo controller, Today/Work/Knowledge/Learning preview UI.
- Migrations `003_platform_foundation.sql`, `004_platform_safety.sql`, `005_knowledge.sql`, `006_work.sql`.
- Auth/session, provider probes, speech, learning, privacy và production-hardening logic từ RC1.

### 2.2. Khoảng trống sản phẩm tại baseline R0

Các điểm dưới đây là khoảng trống được ghi nhận khi lập product-reset plan;
chúng là lý do của các slice S1–S7, không phải trạng thái hiện tại. Trạng thái
đã triển khai và các giới hạn còn mở được theo dõi trong
`docs/project/STATUS.md` và các acceptance records.

- `backend/internal/httpapi/server.go` chưa mount Work/Knowledge/Assistant/MCP routes.
- Chưa có một application composition root nối repository, services, provider router và transports.
- Assistant service hiện mới chốt grounding contract; persistence và provider routing vẫn được để “injected later”.
- MCP application hiện read-only và tự ghi rõ không mount HTTP hoặc mutation tools.
- Authenticated production trong `lib/main.dart` vẫn trả về legacy Home/Practice/Review/Progress shell.
- `WorkspaceController` vẫn dùng `WorkspaceDemo` và `DemoAssistantGateway`.
- `workspace_screen.dart` là một file lớn, dùng Material defaults và card wall; chưa phải UI sản phẩm được thiết kế.
- CyberOS backlog chưa có product task cho đợt cải tổ này; các WAVE manifest cũ không phải task-state authority.

Kết luận: **foundation đã tồn tại nhưng runtime composition và product UX chưa tồn tại**. Công việc tiếp theo là tích hợp, mở rộng và thay UI — không reimplement các package đã được kiểm chứng.

## 3. Chiến lược reuse, integrate và rebuild

| Nhóm | Xử lý | Điều kiện |
|---|---|---|
| Auth/session/privacy/secrets/provider probes | Giữ và regression-test | Không thay security boundary khi không cần |
| Writing/SRS/vocabulary/mistake/roleplay/speaking | Giữ logic, chuyển surface vào Learning | Learning data không được sửa canonical Work |
| Work/Knowledge/Connectors/MCP/Assistant packages trên remote main | Integrate và mở rộng | Không copy từ worktree cũ nếu remote main đã có |
| Migrations 003–006 | Giữ làm migration chain chuẩn | Chạy upgrade test từ legacy snapshot |
| REST composition và app bootstrap | Xây mới | Dùng application services chung với MCP |
| Flutter production data layer | Xây mới | Không cho production fallback sang demo |
| Application shell, design system và feature screens | Rebuild | Không tiếp tục mở rộng `workspace_screen.dart` nguyên khối |
| Legacy top-level navigation | Compatibility tạm thời | Chỉ deprecate sau data/UI parity và rollback test |

## 4. Data integrity và kiến trúc đích

Giữ **compiled modular monolith**: một Go binary, PostgreSQL/pgvector, embedding sidecar tùy chọn. Không tải code bên thứ ba lúc runtime.

### 4.1. Module ownership

```text
platform      workspace, migrations, jobs, audit, health
work          project, task, decision, history, trash, version conflict
knowledge     source, revision, chunk, topic, claim, evidence, retrieval
connectors    Drive/GitHub adapters, sync cursor, idempotency, safe write
assistant     conversation, retrieval, grounding, provider routing, usage
actions       challenge, confirmation, idempotency, receipt
mcp           HTTP transport, token scope, resource/tool adapters
learning      SRS, vocabulary, mistakes, roleplay, speaking, English overlay
httpapi       REST adapters only; không chứa business rules
application   composition root và use cases dùng chung cho REST/MCP
```

### 4.2. Nguồn sự thật

| Dữ liệu | Canonical owner | Quy tắc |
|---|---|---|
| Project, Task, Decision | App/PostgreSQL | Versioned, audited, workspace isolated |
| Drive/GitHub content | External source | Read-only V1, mỗi thay đổi tạo revision mới |
| Knowledge claim | App + evidence links | Không có evidence thì không được coi là fact |
| Assistant output | Conversation history | Không tự trở thành canonical Work/Knowledge |
| Learning observation | Learning module | Không mutate Work/Knowledge |
| External mutation result | Action receipt | Model không được tự tạo receipt |

### 4.3. Grounded response contract

```json
{
  "answer": "...",
  "grounding": "grounded | inferred | unknown",
  "evidence": [],
  "unknowns": [],
  "staleSources": [],
  "suggestedActions": [],
  "actionReceipts": []
}
```

Quy tắc bắt buộc:

- Không có verified evidence: trả `unknown`; không trả raw model answer như fact.
- Citation chỉ tham chiếu evidence ID mà retriever đã cấp.
- Stale source phải xuất hiện trong response/UI.
- Retrieved content là untrusted data, không thay system policy hoặc tự gọi tool.
- Usage lấy provider-reported usage khi có; nếu không có thì ghi `unavailable`, không đoán.

### 4.4. API boundary đề xuất

Giữ `/api/v1/*` cho compatibility; tính năng mới dùng `/api/v2/*`.

| Surface | Vai trò |
|---|---|
| `GET /api/v2/bootstrap` | Current workspace, Today summary và capability state |
| `/api/v2/projects`, `/tasks`, `/decisions` | Work CRUD/version/trash |
| `/api/v2/knowledge/sources`, `/search` | Import, revision, retrieval và evidence |
| `/api/v2/assistant/conversations`, `/messages` | Persistent text-first assistant |
| `/api/v2/actions/challenges`, `/confirm` | Safe mutation boundary |
| `/mcp` | Streamable HTTP adapter dùng cùng application services |

Tên route được khóa trong task contract trước implementation; REST và MCP không được có hai bộ business semantics.

### 4.5. Provider routing theo capability

Không hard-code business logic vào tên model. Dùng capability:

- `chat_fast`: mặc định DeepSeek fast.
- `reasoning`: mặc định DeepSeek smart/pro.
- `stt`: Groq.
- `tts` và `pronunciation`: Azure Speech.

Provider/model là cấu hình có probe và budget guard. Deterministic provider chỉ bật bằng explicit demo/test mode; production fail closed.

## 5. UI rebuild — thiết kế trước, code sau

### 5.1. Hướng thị giác

Định hướng đề xuất: **editorial command center** — giống một workbench có thứ bậc rõ, không giống dashboard template.

- Nền trung tính, typography mạnh, một accent chính và semantic colors có nghĩa.
- Dùng line, spacing và grouping trước khi dùng card.
- Evidence/unknown/stale có visual language riêng.
- Mật độ thông tin vừa phải; mobile không kéo dài bằng chuỗi card khổng lồ.
- Copy ngắn, cụ thể theo dữ liệu; không dùng slogan hoặc filler AI chung chung.
- English và Vietnamese có vai trò rõ: work content giữ nguyên ngôn ngữ người dùng; learning helper giải thích song ngữ khi cần.

### 5.2. Anti-pattern bị cấm

- Dùng trực tiếp Material `Card`, `FilledButton`, `NavigationBar` rải khắp feature screen.
- Một file screen trên hàng trăm dòng chứa cả shell, navigation và mọi feature.
- Demo data âm thầm xuất hiện trong production.
- Primary action full-width lặp lại ở mọi section.
- Card-in-card, border quanh mọi khối hoặc quá nhiều icon không mang thông tin.
- Citation ẩn sau generic “Sources” mà không có excerpt/locator.
- Chỉ thiết kế happy path; bỏ loading, empty, error, offline, stale và permission state.

### 5.3. Information architecture

Mobile primary navigation:

```text
Today | Work | Knowledge | Learning
```

Settings, workspace switcher và capability status nằm ở app bar/profile menu. Desktop dùng navigation rail/sidebar tương ứng.

Assistant không là một tab tách biệt:

- Today có composer chính và conversation timeline.
- Work/Knowledge có “Ask with this context”.
- Conversation mở thành full-screen mobile hoặc split pane desktop.

### 5.4. Màn hình đích

#### Today

- Workspace + sync/capability state.
- Một `Next best action` có lý do và nguồn.
- Persistent assistant composer.
- Recent conversation và evidence.
- Quick capture: task, decision, manual source.
- Một English micro-prompt tùy chọn lấy từ task hiện tại.

#### Work

- Project list/search/filter.
- Project detail: tasks, decisions, linked knowledge, activity.
- Task detail: status, priority, due date, version, context và conversation.
- Decision detail: context, alternatives, outcome, evidence và author/time.
- Conflict UI hiển thị expected/current version; không silently overwrite.

#### Knowledge

- Search-first.
- Source list có type, sync status, revision và freshness.
- Source detail có excerpt/locator và revision timeline.
- Claim ↔ evidence relationship rõ ràng.
- Import preview trước khi ghi.

#### Learning

- Mission được tạo từ project/task/decision/conversation thật.
- Writing, roleplay, speaking, vocabulary, SRS, mistakes và insights.
- “Help me answer” cho Vietnamese explanation + English starter + follow-up.
- Transcript luôn hiển thị/chỉnh sửa được; TTS chỉ chạy khi bấm.

### 5.5. Flutter structure mục tiêu

```text
lib/src/app/                 shell, navigation, bootstrap, route guards
lib/src/design_system/       tokens, typography, primitives, states
lib/src/features/today/
lib/src/features/work/
lib/src/features/knowledge/
lib/src/features/assistant/
lib/src/features/learning/
lib/src/data/                REST DTOs, API clients, repositories
```

Legacy screens giữ ở compatibility area cho tới cutover; không tiếp tục trộn vào shell mới.

### 5.6. UX Gate U0 — bắt buộc trước Flutter implementation

Deliverables:

1. Audit screenshot UI hiện tại ở mobile 390×844 và desktop 1440×900.
2. User-flow map cho: create task, import source, ask assistant, inspect citation, confirm action, start learning.
3. Low-fidelity wireframes cho Today, Work, Knowledge, Conversation và Learning.
4. High-fidelity mobile screens cho năm surface trên, cùng một desktop responsive composition.
5. Token sheet và component state sheet.
6. Prototype cho navigation, assistant/citation và action confirmation.
7. Operator verdict: `approved` hoặc `changes_requested`.

Không bắt đầu rebuild screen trước khi U0 được duyệt. Đây là gate ngăn UI tiếp tục mang cảm giác AI-generated mặc định.

## 6. Delivery plan theo vertical slice

Các task ID dưới đây là **đề xuất**. Chỉ tạo task files/frontmatter/backlog sau khi operator duyệt plan; task state phải tuân CyberOS.

### Gate R0 / TASK-PRODUCT-001 — Rebaseline và UX lock

**Mục tiêu:** có một integration base, task graph và UI direction duy nhất.

**Thực hiện:**

- Reconcile remote `main@104304f`, local stale refs và dirty WIP; không reset WIP.
- Lập reuse ledger cho từng file/package; không copy lại output đã có trên remote main.
- Tạo integration worktree sạch từ remote main sau human approval.
- Chạy baseline thật: Go format/vet/test/race/build, migrations, Flutter analyze/test/web build/Android debug build, Docker health/ready.
- Chạy UX Gate U0.
- Tạo TASK-PRODUCT-002…008 với dependency, file cone, acceptance và evidence path.
- Chốt migration policy: một default workspace/user; giữ learning tables; legacy `work_context` chỉ map thành source/context có provenance.

**Exit:** operator chấp thuận baseline + U0; một clean integration worktree; CyberOS task graph không drift.

### Slice S1 / TASK-PRODUCT-002 — Product walking skeleton

**Mục tiêu demo duy nhất:**

```text
Login
→ Today shell mới
→ tạo Project + Task thật
→ nhập một manual Knowledge source
→ hỏi Assistant về source/task
→ nhận câu trả lời có citation
→ refresh/restart vẫn thấy dữ liệu và conversation
```

**Scope tối thiểu:**

- Application composition root.
- Current workspace resolver.
- REST v2 tối thiểu cho project/task, manual source/search và assistant ask.
- Conversation persistence.
- Production Flutter repositories/controllers.
- UI mới cho Today, Work, Knowledge và Conversation theo U0.
- Demo gateway chỉ tồn tại trong explicit test/demo configuration.

**Phạm vi S1 được cố ý deferred (đã giao cho các slice sau):** Drive/GitHub
sync, vector search, external action, voice và MCP live transport. Các phần
này hiện được đối chiếu ở S3–S6 và trạng thái thực tế nằm trong
`docs/project/STATUS.md`.

**Acceptance:**

- Không route nào trong demo phụ thuộc `WorkspaceDemo`.
- PostgreSQL reload và process restart không mất dữ liệu.
- Citation trỏ đúng revision/excerpt.
- Không evidence trả unknown.
- Cross-user/workspace isolation pass.
- Mobile screenshot/golden đúng U0.

S1 là gate quan trọng nhất: sau S1 app phải “thành hình”, dù feature depth còn ít.

### Slice S2 / TASK-PRODUCT-003 — Work depth và internal actions

- Full Project/Task/Decision CRUD.
- History, optimistic concurrency, soft-delete/restore và audit.
- Internal action preview, idempotency key và receipt.
- Today priority/next-action dùng dữ liệu Work thật.
- Conflict, trash, empty/error UI.

**Exit:** create/update/conflict/trash/restore/replay chạy qua UI và API; không ghi đè âm thầm.

### Slice S3 / TASK-PRODUCT-004 — Knowledge sync và hybrid retrieval

- Source/item/revision/chunk/claim/evidence completion.
- Drive read-only và GitHub read-only incremental sync.
- PostgreSQL lexical/FTS search trước.
- `multilingual-e5-small` 384-dimension sidecar và Reciprocal Rank Fusion sau khi FTS ổn định.
- Sidecar lỗi phải fallback FTS và báo degraded.
- Knowledge import/sync/revision/stale UI.

**Exit:** sync idempotent, revision bất biến, citation đúng source, permission/rate-limit không phá dữ liệu đã có.

### Slice S4 / TASK-PRODUCT-005 — Production assistant và safe external actions

- Capability-based provider router.
- Persistent multi-turn conversation và bounded context.
- Usage/budget guard, retry chỉ cho network/429/5xx.
- Suggested action không executable.
- Challenge TTL năm phút, canonical action hash, one-time confirmation và receipt.
- GitHub V1 chỉ issue/comment/label.
- Action confirmation/receipt UI.

**Exit:** fabricated citation golden corpus = 0; replay trả receipt cũ; thiếu/expired confirmation không chạy action.

### Slice S5 / TASK-PRODUCT-006 — Workflow-native Learning và voice

- Chuyển writing, roleplay, speaking, vocabulary, SRS và mistake memory vào Learning shell.
- Tạo mission từ task/decision/conversation.
- Beginner-safe Vietnamese guidance.
- Push-to-talk → editable transcript; TTS on demand.
- Learning observation bất đồng bộ, không mutate Work/Knowledge.

**Exit:** Task → Learning session → feedback → observation chạy thật; voice failure vẫn giữ text.

### Slice S6 / TASK-PRODUCT-007 — MCP parity

- Mount `/mcp` Streamable HTTP.
- Bearer token riêng, digest storage, one-time reveal, default expiry 90 ngày, revoke.
- Scopes: `knowledge:read`, `work:read`, `work:write`, `assistant:use`, `github:write`.
- Read tools/resources trước; write/confirmed tools dùng action service chung.
- Protocol, auth, scope, replay và conformance tests.

**Exit:** cùng một use case qua REST và MCP có cùng validation, conflict, unknown, receipt và isolation semantics.

### Slice S7 / TASK-PRODUCT-008 — Cutover, migration và release hardening

- Authenticated production mặc định vào Today shell mới.
- Legacy learning routes chuyển thành compatibility surfaces.
- Backfill default workspace và chạy legacy-upgrade preservation harness.
- Rollback flag quay lại legacy shell mà không mất dữ liệu.
- Docker backend/PostgreSQL/embedding sidecar E2E.
- Responsive, accessibility, performance, offline/provider failure và secret-redaction tests.
- Cập nhật README, STATUS, ROADMAP, DECISIONS, TEST_LOG, changelog và release report theo evidence thật.

**Exit:** không còn production demo fallback; người dùng cũ không mất learning data; release candidate có rollback evidence.

## 7. Quality gates

Mỗi task phải có exact command, exit code và evidence path. Green process hoặc manifest không đủ.

### Backend

- `gofmt`, `go vet`, unit/integration, `go test -race`, build.
- PostgreSQL migration từ clean DB và legacy snapshot.
- Cross-user/workspace isolation.
- Concurrency, idempotency, replay và secret redaction.
- Touched/new packages đạt tối thiểu **90% statement coverage** theo CyberOS; tổng coverage không thấp hơn R0 baseline.
- Với profile Go PostgreSQL-enabled, chạy `node
  scripts/backend_coverage_gate.mjs --profile <coverprofile> --min 90`; gate
  so sánh raw statement counts bằng số nguyên và phải liệt kê đầy đủ các
  package được yêu cầu. Truyền thêm
  `--baseline-manifest docs/project/coverage/2026-08-29-r0-backend-coverage-baseline.json`
  để enforce tổng coverage không thấp hơn R0. Không dùng phần trăm đã làm
  tròn làm bằng chứng.

### Flutter

- `flutter analyze`, unit/widget tests, web build và Android debug build.
- Golden screenshots ở 390×844, 430×932 và desktop 1440×900.
- Loading, empty, error, offline, stale, unknown, permission và conflict states.
- Tap target ≥44px, semantics, contrast và keyboard navigation web.

### Runtime

- Docker health/ready và login/session.
- PostgreSQL restart persistence.
- Provider contract tests dùng fake/local server; test mặc định không gọi live provider.
- Browser/mobile-sized smoke cho acceptance flow của task.
- 100.000-chunk load/index/pagination chỉ chạy ở S7 capacity gate, không chặn mọi task nhỏ.

## 8. CyberOS và reviewer protocol

CyberOS task frontmatter và `docs/tasks/BACKLOG.md` là trạng thái duy nhất. WAVE JSON chỉ là historical evidence.

Quy trình mỗi task:

1. Operator duyệt plan/task để chuyển `draft → ready_to_implement`.
2. Luna Max ở luồng chính triển khai task đầu tiên đủ dependency.
3. Luna tạo completion packet và chuyển `implementing → ready_to_review` theo workflow.
4. Chỉ khi output frozen mới gọi **một** Sol Ultra read-only reviewer.
5. Main controller phải đọc nội dung thật của Sol; timeout không phải “không có kết quả”.
6. Nếu Sol còn active, dùng bounded `wait_threads`; không spawn reviewer thay thế hoặc task trùng.
7. Sol trả `approved`, `changes_requested` hoặc `blocked`; Sol không sửa code.
8. Human acceptance bắt buộc tại `reviewing → ready_to_test` và `testing → done`.
9. Commit, push, merge và deploy luôn cần operator authorization riêng.

Completion packet:

```text
Task / slice:
Base commit and worktree:
Scope and file cone:
Files changed:
Migration/dependency impact:
Tests with exact exit codes:
Runtime evidence:
Known limitations:
Security/data-integrity notes:
Reviewer handoff:
```

## 9. Migration và rollback

- Tạo một default workspace cho mỗi existing user bằng deterministic mapping.
- Không đổi hoặc xóa learning tables trong cùng migration với workspace backfill.
- Legacy `work_context` trở thành Knowledge source/context có provenance; không tự suy diễn thành Task/Decision.
- Migration chạy idempotent và có dry-run/report.
- Shell cutover có feature flag; rollback chỉ đổi routing, không rollback data writes.
- Purge vĩnh viễn là action riêng; soft-delete mặc định có restore window.

## 10. Definition of Done cấp sản phẩm

DevEnglish Product Reset chỉ hoàn tất khi:

- Today/Work/Knowledge/Learning dùng một shell và production data layer.
- Walking skeleton và mọi slice tiếp theo chạy qua PostgreSQL thật.
- Project/Task/Decision có conflict protection, audit và trash/restore.
- Knowledge có source/revision/evidence và sync idempotent.
- Assistant luôn phân biệt grounded/inferred/unknown/stale.
- Conversation text được persist; voice chỉ là optional input/output.
- External action có confirmation và receipt.
- MCP/REST dùng cùng application services.
- English overlay đến từ workflow và không làm bẩn canonical data.
- Production không dùng demo data hoặc deterministic fallback ngầm.
- UI đã qua operator-approved U0, golden/responsive/accessibility checks.
- Migration, rollback, Docker E2E và release evidence đều có command/output thật.

## 11. Hành động lịch sử sau khi plan được duyệt

Các bước dưới đây là trình tự khởi tạo được ghi trong Revision 2.0. Ở
checkpoint hiện tại, chúng đã được thực hiện ở mức implementation/evidence;
không dùng danh sách lịch sử này để suy ra release acceptance. Xem
`docs/project/STATUS.md` và `docs/project/agent-runs/2026-08-29-acceptance-audit.md`
để biết trạng thái hiện tại.

1. Tạo `TASK-PRODUCT-001-rebaseline-ux-lock` ở trạng thái `draft`.
2. Reconcile remote main, local WIP và integration history; không checkout/reset current WIP.
3. Tạo UX U0 artifacts và xin operator verdict.
4. Sau U0, tạo task graph 002–008 với file cone/dependency cụ thể.
5. Bắt đầu S1 walking skeleton; dừng tại review gate với đúng một Sol reviewer.

Tài liệu này không tự cấp quyền commit, push, merge, deploy hoặc thay đổi CyberOS task state.
