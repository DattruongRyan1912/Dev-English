# Wave 2 integration review R7

## Trạng thái

- Verdict Sol Ultra: **BLOCKED**.
- Reviewer: Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`).
- Review turn: `01a03d80-e5df-7342-bbae-fc82dc25a9ef`.
- Integration branch: `integration/wave-2`.
- Integration worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-2-integration`.
- Base commit: `d39019abb93b333654b14e068a1ae0d476b1cae2`.

## Phạm vi đã tích hợp

Đúng 32 file được stage trong sáu path cone đã được phê duyệt:

- `backend/internal/knowledge/`
- `infra/migrations/005_knowledge.sql`
- `backend/internal/work/`
- `infra/migrations/006_work.sql`
- `backend/internal/connectors/`
- `backend/internal/mcp/`

Các cone trong integration worktree được kiểm tra byte-for-byte với bốn frozen worker worktree đã approved. Integration worktree không có file unstaged hoặc untracked ngoài các file đã stage.

## Blocker

`005_knowledge.sql` khai báo dependency tới `003_platform_foundation.sql` và tạo foreign key tới `workspaces`. `006_work.sql` cũng khai báo dependency đó và tham chiếu `workspaces`. Trong integration branch chỉ có các migration `000`, `001`, `002`, `005`, `006`; không có migration tạo bảng `workspaces`.

Migration runner đọc và thực thi toàn bộ file SQL theo thứ tự tên file. Vì vậy fresh migration chain của branch này không thể tạo schema hợp lệ cho 005/006. Sol xác định đây là blocker dependency/authorization, chưa phải lỗi runtime PostgreSQL đã quan sát.

Canonical checkout hiện có các file nền tảng `003_platform_foundation.sql` và `004_platform_safety.sql` dưới dạng WIP chưa được stage. Chúng nằm ngoài bốn cone Wave 2 đã được cấp quyền tích hợp, nên không tự ý copy hoặc rebase chúng vào branch này.

## Gate thực tế

- Focused race test bốn package: exit `0`; `304` pass, `2` PostgreSQL test skip.
- Coverage độc lập theo package: Knowledge `96.3%`, Work `93.3%`, Connectors `91.5%`, MCP `95.3%`.
- Repository test: exit `0`; `351` pass, `3` skip.
- Focused/repository vet: exit `0`.
- `gofmt -d`: exit `0`, không output.
- Staged diff check: exit `0`.
- So sánh với frozen worker cones: exit `0`.
- Protocol verifier: exit `0` với `PASS protocol`, `PASS entrypoints`, `PASS manifests`.
- PostgreSQL migration runtime: **chưa kiểm chứng**; `DEVENGLISH_TEST_DATABASE_URL` chưa được đặt, `COMPOSE_FILE` chưa được đặt và không dùng shared development database.
- Live provider và end-to-end runtime: **chưa kiểm chứng**.

## R8 tiền kiểm sau khi được cấp quyền

Người dùng đã cấp quyền đưa hai migration nền tảng vào integration target. Hai file được copy nguyên trạng từ canonical WIP và stage thêm; so sánh `cmp` với source exit `0`.

- Migration test bằng Compose project/volume/port cô lập: exit `0`.
- First-run áp dụng thành công toàn bộ `000` tới `006`.
- Second-run xác nhận idempotency cho toàn bộ migration.
- Failing migration xác nhận rollback và không ghi version giả.
- PostgreSQL-focused tests: exit `0`; `90` pass trong Knowledge và Store.
- Focused race tests có PostgreSQL URL: exit `0`; `307` pass trong 5 package.
- Repository test: exit `0`; `351` pass trong 15 package.
- Repository vet, gofmt và staged diff check: exit `0`.
- Protocol verifier: `PASS protocol`, `PASS entrypoints`, `PASS manifests`.

Branch đã đủ điều kiện để Sol Ultra thực hiện review R8 độc lập. Commit, push, merge và deploy vẫn chưa được cấp quyền.
