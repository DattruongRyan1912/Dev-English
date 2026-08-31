# U0 — Screenshot audit

## Scope

Đối chiếu theo các screenshot mobile đã ghi nhận trong quá trình phát triển và
desktop browser smoke tại 390x844, 430x932 và 1440x900. Screenshot chỉ là
evidence về presentation; hành vi phải được xác nhận bằng test/runtime.

## Findings

| Surface | Hiện trạng quan sát được | Quyết định rebuild |
| --- | --- | --- |
| Shell | Hai cảm giác sản phẩm cùng tồn tại: Learning legacy (Home/Practice/Review/Progress) và workspace mới | Một IA duy nhất: Today / Work / Knowledge / Learning; legacy chỉ nằm trong compatibility entry |
| Today | Copy kiểu template, nhiều khối card, chưa làm nổi bật next safe action | Một briefing có thứ bậc: workspace state → next action → assistant timeline → quick capture |
| Work | Project/Task/Decision hiển thị được nhưng thiếu edit, conflict, trash/restore và detail context | Tách list/detail; mutation luôn hiển thị version và kết quả |
| Knowledge | Evidence có mặt nhưng chưa có revision/freshness/sync state rõ ràng | Search-first; evidence excerpt + locator + freshness + sync run |
| Conversation | Text-first đã có nhưng chưa thể hiện rõ grounding grounded/inferred/unknown và action boundary | Mỗi trả lời có grounding badge, citations, unknowns và suggested actions không tự chạy |
| Learning | Các tính năng cũ hoạt động như một app học riêng, chưa lấy work làm nguồn mission | Learning là overlay; mission bắt nguồn từ task/decision/conversation |
| Mobile | Một số CTA/card dễ kéo dài theo chiều dọc; thao tác phụ khó thấy | Một primary action/section, bottom navigation cố định, action menu có hit area tối thiểu 44px |
| Desktop | Nội dung nằm giữa nhưng chưa có workspace context và split detail | Sidebar + main column + optional detail/context pane |

## Visual rules

- Không dùng slogan chung chung hoặc dữ liệu demo âm thầm trong canonical mode.
- Dùng line, spacing và typography trước khi thêm border/card.
- Preview, Canonical, Degraded, Stale, Unknown phải là trạng thái có nghĩa và
  có copy giải thích.
- Không che excerpt/locator sau nút Sources chung chung.
- Mọi mutation phải có pending, success, conflict và failure state.

## Required screenshot matrix

| Viewport | Required states |
| --- | --- |
| 390x844 | Today loaded/empty/error; Work list/form/conflict; Knowledge search/stale; Conversation unknown/citation; Learning mission/transcript |
| 430x932 | Same states with long titles, keyboard visible and safe-area padding |
| 1440x900 | Sidebar, Today split context, Work detail, Knowledge detail, Conversation action preview |
