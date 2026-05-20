# Smart Ferry Booking — Yêu cầu hệ thống (Requirements)

> **Nguồn gốc**: `[Ferry]Smart ferrry booking_Requirements.pptx`
> **Vai trò tài liệu**: Đây là tài liệu yêu cầu gốc (Phase 1) của hệ thống đặt vé phà Smart Ferry Reservation, bao gồm overview, list chức năng, luồng nghiệp vụ, yêu cầu chức năng chính và định nghĩa dữ liệu ban đầu.

---

## 1. Tổng quan dự án (Overview)

- **Mục tiêu**: Xây dựng hệ thống quản lý đặt chỗ phà trực tuyến cho 2 công ty khai thác phà khứ hồi tuyến Kagoshima → Okinawa (Marue Ferry & Marix Line).
- **Kiến trúc**: Thiết kế theo mô hình SaaS, multi-tenant theo Organization ID, có thể mở rộng cho các công ty phà khác.
- **Kênh booking**: Web (khách hàng cuối) và Điện thoại (admin nhập tay).
- **Thanh toán**:
  - Web: Chỉ thanh toán bằng thẻ credit card (Stripe).
  - Admin: Credit card, chuyển khoản ngân hàng (trả trước/trả sau), tiền mặt.
- **Xác thực**: Auth0 cho cả Web (member) và Admin (account).
- **Tên service**: スマートフェリー予約 (Smart Ferry Booking)
- **Domain**: `ferry.smaku.app`

### 1.1 Đặc điểm kinh doanh quan trọng

- 2 công ty dùng chung hệ thống, spec thống nhất (ngoại trừ một số ngoại lệ).
- Dữ liệu được quản lý theo từng Organization (multi-tenant).
- Yêu cầu liên kết một phần dữ liệu booking với Salesforce (SF build API, hệ thống này call API sang SF).
- Booking tập thể: trạng thái ban đầu là **đặt tạm (tentative)**, cần admin approve mới thành **booking chính thức (confirmed)**.
- Quản lý nghiêm ngặt cabin trống, tránh booking trùng (race condition).
- Discount và phí bổ sung có logic phức tạp, tách riêng khỏi logic booking chung.

---

## 2. Schedule phát triển ban đầu

| No  | Milestone                             | Phụ trách         | Thời gian    |
| --- | ------------------------------------- | ----------------- | ------------ |
| 1   | Định nghĩa requirement + basic design | 池田 (Ikeda)      | Tháng 6 đầu  |
| 2   | Create design (Figma)                 | 鴻上 (Kōkami)     | Tháng 6      |
| 3   | Check design                          | Khách hàng (船社) | Tháng 6 - 7  |
| 4   | Design detail + implement + test      | AZV               | Tháng 6 - 9  |
| 5   | UAT nội bộ                            | 池田              | Tháng 8      |
| 6   | Build MT PRD                          | AZV               | Tháng 8      |
| 7   | Build liên kết SF                     | AZV               | Tháng 8 - 9  |
| 8   | Khách hàng UAT                        | 船社              | Tháng 9      |
| 9   | Đối ứng bugs / yêu cầu                | 池田 + AZV        | Tháng 9      |
| 10  | Release                               | AZV               | Cuối tháng 9 |

---

## 3. Danh sách màn hình

### 3.1 Màn hình Admin (Management System)

| No  | Tên JP         | Tên EN             | Mô tả                                                         | Agent dùng    |
| --- | -------------- | ------------------ | ------------------------------------------------------------- | ------------- |
| 1   | ログイン       | Login              | Xác thực bằng ID + Password                                   | ○             |
| 2   | ダッシュボード | Dashboard          | TOP page, menu chức năng, booking tạm trong ngày              | ○             |
| 3   | 予約一覧       | Booking List       | Danh sách booking, search, filter, column setting, CSV export | ○             |
| 4   | 予約詳細       | Booking Detail     | Đăng ký/edit thông tin booking                                | ○             |
| 5   | 顧客一覧       | Customer List      | Danh sách khách hàng, search, CSV export                      | ○             |
| 6   | 顧客詳細       | Customer Detail    | Edit thông tin khách hàng                                     | ○             |
| 7   | 運航カレンダー | Sailing Calendar   | Đăng ký lịch chạy tàu theo ngày/tàu/timetable                 | —             |
| 8   | 残数編集       | Capacity Edit      | Số booking theo cabin, chỉnh booking limit                    | —             |
| 9   | 運航ダイヤ     | Sailing Timetable  | Hiển thị bảng giờ tàu chạy                                    | —             |
| 10  | 運航ダイヤ詳細 | Timetable Detail   | Đăng ký/edit timetable                                        | —             |
| 11  | 船一覧         | Ship List          | Danh sách tàu                                                 | —             |
| 12  | 船詳細         | Ship Detail        | Đăng ký/edit thông tin tàu                                    | —             |
| 14  | ターミナル一覧 | Terminal List      | Danh sách terminal                                            | —             |
| 15  | ターミナル詳細 | Terminal Detail    | Đăng ký/edit terminal                                         | —             |
| 16  | 代理店一覧     | Agent List         | Danh sách đại lý                                              | —             |
| 17  | 代理店詳細     | Agent Detail       | Đăng ký/edit đại lý                                           | —             |
| 18  | アカウント一覧 | Account List       | Danh sách tài khoản admin                                     | —             |
| 19  | アカウント詳細 | Account Detail     | Đăng ký/edit tài khoản                                        | △ (edit only) |
| 20  | 運賃一覧       | Fare List          | Danh sách master fare                                         | —             |
| 21  | 運賃詳細       | Fare Detail        | Đăng ký/edit fare                                             | —             |
| 22  | 料金一覧       | Fee List           | Danh sách master fee                                          | —             |
| 23  | 料金詳細       | Fee Detail         | Đăng ký/edit fee                                              | —             |
| 24  | 割引一覧       | Discount List      | Danh sách master discount                                     | —             |
| 25  | 割引詳細       | Discount Detail    | Đăng ký/edit discount                                         | —             |
| 26  | 支払管理       | Payment Management | Quản lý thanh toán (Stripe)                                   | —             |
| 27  | パスワード変更 | Password Change    | Đổi mật khẩu                                                  | —             |

### 3.2 Màn hình Web User

**Booking qua WEB:**

| No  | Tên JP                   | Tên EN                 | Mô tả                         |
| --- | ------------------------ | ---------------------- | ----------------------------- |
| 1   | 乗船検索                 | Boarding Search        | Input điều kiện boarding      |
| 2   | 検索結果一覧             | Search Result List     | Danh sách chuyến phù hợp      |
| 3   | 会員登録・ログイン選択   | Member/Login Selection | Chọn đăng ký hoặc login       |
| 4   | 会員登録                 | Member Registration    | Nhập thông tin đăng ký member |
| 5   | 予約詳細                 | Booking Detail         | Nhập thông tin booking        |
| 6   | 予約詳細確認             | Booking Confirm        | Xác nhận nội dung booking     |
| 7   | クレジットカード情報入力 | Credit Card Input      | Nhập thông tin thẻ            |
| 8   | 予約完了                 | Booking Complete       | Màn hình hoàn tất             |

**My Page:**

| No  | Tên JP           | Tên EN          | Mô tả                         |
| --- | ---------------- | --------------- | ----------------------------- |
| 1   | ログイン         | Login           | Đăng nhập My Page             |
| 2   | マイページトップ | MyPage Top      | Trang chủ sau login           |
| 3   | 予約確認         | Booking Confirm | Xem booking hiện tại          |
| 4   | 予約履歴         | Booking History | Lịch sử booking               |
| 5   | アカウント編集   | Account Edit    | Chỉnh sửa thông tin tài khoản |

---

## 4. Business Flow

### 4.1 Booking qua WEB

```
Khách hàng → Click "Book qua WEB" trên HP công ty tàu
  → Search chỉ định điều kiện (cảng đi/đến, ngày, số người, có/không xe)
  → Hiển thị kết quả search
  → Chọn Login hoặc Đăng ký mới
    ├── [Đăng ký mới] → Input thông tin member → Gửi mail xác thực
    └── [Login] → Đăng nhập bằng ID/PW
  → Input booking detail (hành khách, xe, discount, ghi chú)
  → Confirm nội dung booking
  → Input thông tin credit card → Stripe authorize
    ├── [NG] → Yêu cầu sửa thông tin card
    └── [OK] → Hoàn tất booking
  → Gửi 3 email:
    1. Email hoàn tất booking (cho customer)
    2. Email thông báo booking mới (cho admin)
    3. Email thông báo booking mới (cho agent)
```

### 4.2 Booking qua Admin (điện thoại)

- Admin search chuyến theo điều kiện → Hiển thị số phòng trống theo hạng cabin trong 1 tuần (±3 ngày).
- **Khác biệt so với WEB**: Ngày đi không có phòng trống thì **không hiển thị "Chờ cancel"**.
- Input các trường thông tin booking tương tự WEB nhưng có thêm: phân loại booking (cá nhân/tập thể/quân đội), phương thức thanh toán mở rộng (tiền mặt, chuyển khoản).
- Booking tập thể hoặc xe chở hàng → trạng thái **tentative**, cần admin approve.

---

## 5. Yêu cầu chức năng chính

### 5.1 Search & Booking (WEB)

- **Điều kiện search**: Cảng đi/đến, ngày khởi hành (calendar), số người (người lớn/trẻ em/trẻ nhỏ), có/không kèm xe. **Tất cả bắt buộc**.
- Calendar hiển thị trạng thái: chưa bán ra, hết chỗ, còn trống.
- Kết quả search: ngày không có phòng → "Chờ cancel" (キャンセル待ち).
- **Đăng ký member mới** cần: tên, tên kana, birthday, giới tính, địa chỉ, liên lạc, email, password.
- **Input booking detail**:
  - Hiển thị: chuyến/tuyến + hạng ghế đã chọn, thông tin member (preset), fare + fee.
  - Input: người đồng hành (nhiều người), liên lạc khẩn cấp, có thai (tuần), cứu nạn hàng hải, thẻ đảo xa, ghi chú, discount áp dụng, xe (nhiều chiếc).
  - Xe: phân loại (thường/hàng hóa/xe máy/xe điện/xe đạp), biển số, hàng nguy hiểm, kích thước (dài/rộng/cao).
- **Giới hạn**: Tối đa 14 người/booking trên WEB.

### 5.2 Fare / Fee / Discount

#### Fare (Giá vé)

- **2 loại**: Giá vé chỉ hành khách / Giá vé có xe kèm.
- Khác nhau theo: điểm đi/đến, hạng cabin, loại xe, kích thước xe.
- Có thời hạn hiệu lực → hỗ trợ thay đổi giá trong tương lai.
- **Giá trẻ em**: = 50% giá người lớn (làm tròn).

#### Fee (Phụ phí)

- Phụ phí giường A/B (theo điểm đi/đến)
- BAF (điều chỉnh biến động giá dầu) — thay đổi mỗi 3 tháng, theo rank áp dụng, theo ngày boarding
- Phí exclusive (leasure room) — khi booking chưa đạt tối đa cabin = ½ × số chỗ không dùng × (fare + fee)
- Phí xe vượt kích cỡ — thêm 10% mỗi 25cm vượt quá 2.5m chiều rộng
- Phí hành lý ký gửi, hành lý đặc biệt, phòng thú cưng

#### Công thức tính

```
((Fare hành khách HOẶC Fare xe - Discount) + Fee giường + BAF + Phí quy tắc
 + Phí vượt kích cỡ + Phí hành lý + Phí đặc biệt) × 1.1 (thuế 10%)
```

- Mỗi khoản không bao gồm thuế → tổng × 1.1 mới ra số cuối cùng.

#### Discount (Giảm giá)

12 loại discount chính (xem chi tiết file `04_Discount_Rules.md`):

1. Discount net (booking WEB) — đang xem xét
2. Discount sinh viên — giảm 20% hạng 2
3. Discount nhóm chung — giảm 10% tất cả hạng, ≥15 người
4. Discount tập thể học sinh (THCS-THPT-ĐH) — giảm 30% hạng 2, ≥15 người
5. Discount tập thể tiểu học — giảm 30% người lớn, 10% trẻ em, hạng 2
6. Discount người khuyết tật (hạng 1) — giảm 50% cả fare + fee tất cả hạng
7. Discount người khuyết tật (hạng 2) — giảm 50% fare hạng 2
8. Discount cư dân đảo Amami — theo bảng giá riêng
9. Discount giữa đảo Amami — theo bảng giá riêng
10. Discount giao hữu Amami/Okinawa — theo bảng giá riêng
11. Discount quân đội — giảm 10% tất cả hạng (Kagoshima ↔ Okinawa)
12. Discount đặc biệt — 10%~100% hành khách, △1m~△4m xe

**Quy tắc**:

- Ngoài discount người khuyết tật hạng 1, chỉ giảm cho fare, phụ phí cộng sau.
- Có phân số → làm tròn lên 10 Yen.

### 5.3 Timetable & Calendar Boarding

#### Timetable (Bảng giờ tàu chạy)

- Data master quản lý tuyến đường + thời gian đi/đến.
- 1 lịch trình = 1 lượt khứ hồi 4 ngày:
  - Ngày 1: Khởi hành Kagoshima (18:00)
  - Ngày 2: Đến Naha (19:00)
  - Ngày 3: Khởi hành Naha (07:00)
  - Ngày 4: Đến Kagoshima (08:30-09:00)
- Các cảng dọc đường: Kagoshima → Naze → Kametoku → Wadomari → Yoron → Honbu → Naha
- Marue có 2 timetable (có/không Miyanoura), Marix có 1.

#### Calendar Boarding (Lịch khai thác)

- Tạo bằng: Ngày khởi hành × Tàu × Timetable.
- Booking limit = số tối đa/cabin (nếu không nhập dùng cabin capacity).
- Có thể đăng ký chuyến tạm (mưa bão) → chỉ booking qua admin.
- **Timing thêm số booking**:
  - WEB: Thêm lúc chọn chuyến, trả về nếu không hoàn tất.
  - Admin: Thêm lúc xác nhận booking. Tập thể/hàng hóa → thêm tạm, trả về nếu không hoàn tất.

### 5.4 Thanh toán

| Phương thức              | WEB | Admin | Invoice | Biên lai | Stripe |
| ------------------------ | --- | ----- | ------- | -------- | ------ |
| Credit card              | ○   | ×     | ×       | ○        | ○      |
| Tiền mặt                 | ×   | ○     | ×       | ×        | ×      |
| Chuyển khoản (trả trước) | ×   | ○     | ○       | ○        | ○      |
| Chuyển khoản (trả sau)   | ×   | ○     | ○       | ○        | ○      |

- **Credit card (WEB)**: Với Stripe, bước authorize khi booking → không thanh toán ngay → admin xác nhận phí cuối → nhấn "xác nhận thanh toán" → Stripe charge.
- **Chuyển khoản**: Admin tạo invoice qua Stripe → customer trả → Stripe cập nhật status → booking status đồng bộ.
- **Hoàn tiền**: Khi cancel booking có phí cancel → Stripe charge phí cancel + refund phần còn lại.

### 5.5 Xác thực (Authentication)

- **MyPage (End user)**: Đăng ký member → liên kết customer với công ty tàu. Login bằng email + PW.
- **Admin**: Login account admin bằng email + PW. Có quyền hạn: admin (full) vs agent (hạn chế menu).

---

## 6. Định nghĩa dữ liệu ban đầu (Data Items)

### 6.1 Master Organization (組織マスタ)

| No  | JP     | EN                | Type  | Max | Required | Unique |
| --- | ------ | ----------------- | ----- | --- | -------- | ------ |
| 1   | 組織ID | organization_id   | Số    | —   | ○        | —      |
| 2   | 組織名 | organization_name | Chuỗi | 100 | ○        | ○      |

### 6.2 Master Ship (船マスタ)

| No   | JP                              | EN                       | Type  | Max   | Required | Giá trị                                                                                                                           |
| ---- | ------------------------------- | ------------------------ | ----- | ----- | -------- | --------------------------------------------------------------------------------------------------------------------------------- |
| 1    | 船ID                            | ship_id                  | Số    | —     | ○        | —                                                                                                                                 |
| 2    | 組織ID                          | organization_id          | Số    | —     | ○        | —                                                                                                                                 |
| 3    | 船名                            | ship_name                | Chuỗi | 40    | ○        | Unique                                                                                                                            |
| 4    | 船画像                          | ship_image               | Chuỗi | —     | ○        | ≤3 files, png/jpg/jpeg, ≤5MB                                                                                                      |
| 5    | 船内案内図                      | ship_map_image           | Chuỗi | —     | —        | 1 file                                                                                                                            |
| 6    | 紹介文                          | ship_introduction        | Chuỗi | 200   | ○        | —                                                                                                                                 |
| 7    | 特徴                            | feature                  | Số    | —     | —        | Đa chọn: 1:キッズルーム 2:コインロッカー 3:シャワールーム 4:ペット可 5:レストラン 6:売店 7:多目的トイレ 8:ゲームコーナー 9:大浴場 |
| 8-13 | 全長/全幅/トン数/定員/速力/積載 | length/width/tonnage/... | Chuỗi | 10-50 | —        | —                                                                                                                                 |
| 14   | ステータス                      | status                   | Số    | —     | ○        | 1:有効 0:無効                                                                                                                     |

### 6.3 Master Cabin (客室マスタ)

| No  | JP         | EN             | Type  | Max | Giá trị                |
| --- | ---------- | -------------- | ----- | --- | ---------------------- |
| 1   | 客室ID     | cabin_id       | Số    | —   | —                      |
| 2   | 船ID       | ship_id        | Số    | —   | —                      |
| 3   | 客室名     | cabin_name     | Chuỗi | 20  | Unique/ship            |
| 4   | 等級ID     | cabin_class_id | Số    | —   | —                      |
| 5   | 様式       | cabin_style    | Số    | —   | 1:洋室 2:和洋室 3:和室 |
| 6   | 定員数     | cabin_capacity | Số    | —   | —                      |
| 7   | 相部屋可否 | room_share     | Số    | —   | 1:可 0:不可            |
| 8   | 予約上限数 | booking_limit  | Số    | —   | Optional               |

### 6.4 Master Terminal (ターミナルマスタ)

| No    | JP                   | EN                  | Type  | Max |
| ----- | -------------------- | ------------------- | ----- | --- |
| 1     | ターミナルID         | terminal_id         | Số    | —   |
| 2     | 組織ID               | organization_id     | Số    | —   |
| 3     | ターミナル名         | terminal_name       | Chuỗi | 30  |
| 4-6   | 郵便番号/住所/紹介文 | zip/address/intro   | —     | —   |
| 7-9   | 営業時間/電話/メール | hours/tel/email     | —     | —   |
| 10-13 | 担当者/委託業者      | staff/subcontractor | —     | —   |

### 6.5 Master Route (航路マスタ)

| No  | JP               | EN                    | Type      | Ghi chú                            |
| --- | ---------------- | --------------------- | --------- | ---------------------------------- |
| 1   | 航路ID           | route_id              | Số        | —                                  |
| 2   | 組織ID           | organization_id       | Số        | —                                  |
| 3   | 航路名           | route_name            | Chuỗi(30) | Unique/ORG                         |
| 4   | 出発ターミナルID | departure_terminal_id | Số        | —                                  |
| 5   | 到着ターミナルID | arrival_terminal_id   | Số        | —                                  |
| 6   | 距離             | distance              | Số        | Dùng cho discount theo khoảng cách |

### 6.6 Booking cơ bản (予約基本情報)

| No   | JP               | EN                       | Type  | Max | Giá trị                                                 |
| ---- | ---------------- | ------------------------ | ----- | --- | ------------------------------------------------------- |
| 1    | 組織ID           | organization_id          | Số    | —   | —                                                       |
| 2    | 予約ID           | booking_id               | Số    | —   | Auto-gen                                                |
| —    | 予約ステータス   | booking_status           | Số    | —   | 0:キャンセル 1:仮予約 2:本予約 9:完了                   |
| 3    | 予約区分         | booking_type             | Số    | —   | 1:一般旅客のみ 2:一般車両あり 3:団体旅客 4:団体車両あり |
| 4    | 運航カレンダーID | sailing_calendar_id      | Số    | —   | —                                                       |
| 5    | 客室ID           | cabin_id                 | Số    | —   | —                                                       |
| 6    | 貸切利用         | exclusive_use            | Số    | —   | 1:非貸切 2:貸切                                         |
| 7    | 顧客ID           | customer_id              | Số    | —   | Required if type=1,2                                    |
| 8-10 | 大人/小人/乳幼児 | adult/child/infant_count | Số    | —   | —                                                       |
| 11   | 同伴者           | companions               | JSON  | —   | —                                                       |
| 12   | 緊急連絡先       | emergency_contact        | Số    | —   | —                                                       |
| 13   | メール           | booking_mail_address     | Chuỗi | 256 | —                                                       |
| 14   | 支払方法         | payment_method           | Số    | —   | 1:Credit 2:現地 3:振込前払 4:振込後払                   |
| 17   | 支払状況         | payment_status           | Số    | —   | 1:未入金 2:入金済                                       |
| 29   | 運賃             | fare                     | Số    | —   | —                                                       |
| 32   | 合計金額         | total_amount             | Số    | —   | —                                                       |

### 6.7 Account Admin (管理者アカウント)

| No  | JP           | EN                   | Type  | Max | Giá trị                      |
| --- | ------------ | -------------------- | ----- | --- | ---------------------------- |
| 1   | 組織ID       | organization_id      | Số    | —   | —                            |
| 2   | アカウントID | account_id           | Số    | —   | —                            |
| 3   | メール       | account_mail_address | Chuỗi | 256 | Unique                       |
| 4   | 権限区分     | authority_type       | Số    | —   | 1:管理者 2:海運代理店        |
| 5   | 氏名         | account_name         | Chuỗi | 30  | —                            |
| 6   | 代理店ID     | agent_id             | Số    | —   | Required if authority_type=2 |

### 6.8 Member (会員情報)

| No  | JP            | EN                     | Type       | Giá trị           |
| --- | ------------- | ---------------------- | ---------- | ----------------- |
| 1-2 | 組織ID/会員ID | org_id/member_id       | Số         | —                 |
| 3-4 | 姓/名         | family/first_name      | Chuỗi(30)  | —                 |
| 5-6 | 姓カナ/名カナ | family/first_name_kana | Chuỗi(30)  | —                 |
| 7   | 性別          | sex                    | Số         | 1:男 2:女         |
| 8   | 生年月日      | birthday               | Ngày       | —                 |
| 9   | 郵便番号      | zip_code               | Số(7)      | —                 |
| 10  | 住所          | address                | Chuỗi(50)  | —                 |
| 11  | 電話番号      | telephone              | Số(11)     | —                 |
| 12  | メール        | mail_address           | Chuỗi(256) | Unique            |
| 13  | ステータス    | status                 | Số         | 0:仮登録 1:本登録 |

### 6.9 Customer (顧客情報)

| No    | JP          | EN              | Type      | Giá trị                         |
| ----- | ----------- | --------------- | --------- | ------------------------------- |
| 1     | 組織ID      | organization_id | Số        | —                               |
| 2     | 顧客ID      | customer_id     | Số        | —                               |
| 3     | 顧客区分    | customer_type   | Số        | 1:個人 2:法人                   |
| 4     | 会員ID      | member_id       | Số        | Required khi member booking WEB |
| 5-8   | 姓/名/カナ  | names           | Chuỗi     | Required khi type=1             |
| 9     | 性別        | sex             | Số        | 1:男 2:女 (khi type=1)          |
| 10    | 生年月日    | birthday        | Ngày      | Required khi type=1             |
| 11    | 法人名      | corporate_name  | Chuỗi(50) | Required khi type=2             |
| 12-16 | 担当者情報  | staff info      | —         | Required khi type=2             |
| 17-21 | 住所/連絡先 | address/contact | —         | —                               |

**Đặc điểm**:

- Quản lý customer theo từng ORG.
- Tạo customer từ member khi member booking qua WEB.
- Phân biệt cá nhân (type=1) và pháp nhân (type=2).

---

## 7. Sơ đồ quan hệ bảng (ER Overview)

```
Organization (1) ──→ (n) Ship ──→ (n) Cabin ──→ Cabin Class
                 ──→ (n) Terminal
                 ──→ (n) Route (departure_terminal ↔ arrival_terminal)
                 ──→ (n) Timetable ──→ (n) Schedule/Calendar
                 ──→ (n) Account Admin ──→ Agent
                 ──→ (n) Customer ←── Member
                 ──→ (n) Fare/Fee/Discount

Schedule (1) ──→ (1) Capacity Detail (JSON per cabin)

Booking (n) ←── Customer (1)
Booking (1) ──→ (1) Booking Detail (補足情報)
Booking (1) ──→ (1) Invoice
```

---

## 8. Design chung (Common UI/UX)

- Component design chung tham khảo Figma「デザインガイドライン」.
- **Snack bar**: Hiển thị khi hoàn tất CRUD: `〇〇を□□しました。`
- **Popup xóa**: `Title: 本当に削除しますか？ Text: 削除を戻すことはできません...`
- **Popup discard**: `Title: このページから移動しますか？ Text: 保存前にページを移動すると編集内容が破棄されます...`
- **Error popup**: `Error message: {nội dung lỗi từ BE}` + `もう一度お試しください。`

---

## 9. Email Templates

### 9.1 Cấu trúc email

Email gồm 3 phần: **Header** + **Body** + **Footer**.

- Header / Footer dùng chung cho nhiều email.
- FROM: `noreply@ferry.smaku.app`

### 9.2 Danh sách email

| Email               | Trigger                 | Mô tả                              |
| ------------------- | ----------------------- | ---------------------------------- |
| 仮登録完了          | Đăng ký tạm member      | Gửi mã OTP xác thực                |
| 本登録完了          | Xác thực OTP thành công | Thông báo đăng ký chính thức       |
| パスワード再設定    | Yêu cầu reset PW        | Gửi URL reset (hết hạn 30 phút)    |
| メールアドレス変更  | Thay đổi email          | Gửi URL verify email mới (30 phút) |
| 予約完了 (customer) | Hoàn tất booking        | Chi tiết booking cho khách         |
| 予約通知 (admin)    | Có booking mới          | Thông báo cho admin                |
| 予約通知 (agent)    | Có booking mới          | Thông báo cho đại lý               |

---

## 10. Chức năng hoãn lại (Deferred)

- ~~List cabin: Search + Sort~~
- Chức năng thông báo quan trọng cho member (bảo trì hệ thống).
- Chức năng đăng ký thông tin phương tiện trên MyPage.
- Chức năng gửi lại email đăng ký tạm.
