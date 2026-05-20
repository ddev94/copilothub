# Smart Ferry Booking — Định nghĩa dữ liệu (Data Item Definition)

> **Nguồn gốc**: `スマートフェリー予約_データ項目定義.xlsx`
> **Vai trò tài liệu**: Đây là tài liệu định nghĩa chi tiết cấu trúc dữ liệu (database schema) cho toàn bộ hệ thống Smart Ferry Reservation, bao gồm 39+ bảng từ master data đến transaction data.

---

## Mục lục bảng dữ liệu

| No  | Tên bảng (JP)        | Tên bảng (EN)           | Phân loại      |
| --- | -------------------- | ----------------------- | -------------- |
| 1   | 組織マスタ           | organization            | Master         |
| 2   | 船マスタ             | ship                    | Master         |
| 3   | 船内施設マスタ       | ship_facility           | Master         |
| 4   | 客室マスタ           | cabin                   | Master         |
| 5   | 車両枠マスタ         | ship_car                | Master         |
| 6   | 客室設備マスタ       | cabin_equipment         | Master         |
| 7   | ターミナルマスタ     | terminal                | Master         |
| 8   | 航路マスタ           | route                   | Master         |
| 9   | 代理店マスタ         | agent                   | Master         |
| 10  | 管理者アカウント     | admin_account           | Master         |
| 11  | 会員情報             | member                  | User           |
| 12  | 顧客情報             | customer                | User           |
| 13  | 運航スケジュール     | sailing_schedule        | Operation      |
| 14  | 料金マスタ           | charge                  | Pricing        |
| 15  | 割引マスタ           | discount                | Pricing        |
| 16  | BAFマスタ            | baf                     | Pricing        |
| 17  | 予約                 | booking                 | Transaction    |
| 18  | 予約履歴             | booking_history         | History        |
| 19  | 予約変更依頼         | change_booking_request  | Transaction    |
| 20  | 予約車両             | booking_car             | Transaction    |
| 21  | 予約車両履歴         | booking_car_history     | History        |
| 22  | 予約団体             | booking_group           | Transaction    |
| 23  | 予約団体履歴         | booking_group_history   | History        |
| 24  | 乗船券               | boarding_ticket         | Ticketing      |
| 25  | 乗船券履歴           | boarding_ticket_history | History        |
| 26  | 引換券               | exchange_ticket         | Ticketing      |
| 27  | 引換券履歴           | exchange_ticket_history | History        |
| 28  | 荷物                 | baggage                 | Transaction    |
| 29  | クーポン             | coupon                  | Pricing        |
| 30  | 発券手続き           | ticket_procedure        | Ticketing      |
| 31  | 表示項目定義         | display_item_definition | Settings       |
| 32  | クレジットトランザク | credit_transaction      | Payment        |
| 33  | 操作ログ             | operation_log           | Audit          |
| 34  | 車両残数             | vehicle_capacity        | Operation      |
| 35  | 添付ファイル         | attachment              | Transaction    |
| 36  | 領収書発行記録       | receipt_issuance_record | Payment        |
| 37  | アカウント設定       | account_config          | Settings       |
| 38  | 乗船申込手続きQR     | application_form_qr     | Ticketing (V3) |
| 39  | 乗船申込             | application_form        | Ticketing (V3) |
| 39b | 不乗船証明書         | refund_certificate      | Ticketing (V3) |

---

## 1. 組織マスタ (Organization)

Quản lý thông tin công ty phà (multi-tenant).

| No  | Tên JP             | Tên EN                      | Kiểu   | Max | Bắt buộc | Unique | Giá trị / Ghi chú                     |
| --- | ------------------ | --------------------------- | ------ | --- | -------- | ------ | ------------------------------------- |
| 1   | ID                 | id                          | 数値   | —   | ○        | ○      | —                                     |
| 2   | 組織名             | name                        | 文字列 | 14  | ○        | ○      | —                                     |
| 3   | 組織名（英語）     | name_en                     | 文字列 | 28  | ○        | ○      | —                                     |
| 4   | メールアドレス     | mail_address                | 文字列 | 256 | —        | ○      | —                                     |
| 5   | インボイス登録番号 | invoice_registration_number | 文字列 | 14  | ○        | ○      | Số đăng ký hóa đơn                    |
| 6   | 営業時間           | opening_hours               | 文字列 | 20  | —        | —      | —                                     |
| 7   | 電話番号           | telephone_number            | 数値   | 11  | ○        | ○      | —                                     |
| 8   | FAX番号            | fax_number                  | 数値   | 11  | —        | ○      | —                                     |
| 9   | 部署名             | department_name             | 文字列 | 20  | —        | —      | —                                     |
| 10  | 郵便番号           | zip_code                    | 数値   | 7   | ○        | —      | —                                     |
| 11  | 都道府県           | prefectures                 | 文字列 | 4   | ○        | —      | —                                     |
| 12  | 住所               | address                     | 文字列 | 50  | ○        | —      | —                                     |
| 13  | 銀行名             | bank_name                   | 文字列 | 20  | ○        | —      | Tên ngân hàng                         |
| 14  | 支店名             | branch_name                 | 文字列 | 20  | ○        | —      | —                                     |
| 15  | 口座種別           | account_type                | 文字列 | 2   | ○        | —      | —                                     |
| 16  | 口座番号           | account_number              | 数値   | 7   | ○        | —      | —                                     |
| 17  | 口座名義人         | account_name                | 文字列 | 20  | ○        | —      | —                                     |
| 18  | 特等名             | special_class_name          | 文字列 | 20  | ○        | —      | Tên hạng đặc biệt (tùy biến theo ORG) |
| 19  | 特等名（英語）     | special_class_name_en       | 文字列 | 40  | ○        | —      | —                                     |
| 20  | 1等名              | first_class_name            | 文字列 | 20  | ○        | —      | —                                     |
| 21  | 1等名（英語）      | first_class_name_en         | 文字列 | 40  | ○        | —      | —                                     |
| 22  | 2等名              | second_class_name           | 文字列 | 20  | ○        | —      | —                                     |
| 23  | 2等名（英語）      | second_class_name_en        | 文字列 | 40  | ○        | —      | —                                     |
| 24  | 登録日時           | created_datetime            | 日時   | —   | ○        | —      | —                                     |
| 25  | 更新日時           | updated_datetime            | 日時   | —   | ○        | —      | —                                     |

**Đặc điểm kiến trúc**: Mỗi ORG có thể tùy biến tên hạng cabin (special/first/second class) bằng 2 ngôn ngữ (JP/EN). Thông tin ngân hàng dùng cho Stripe invoice.

---

## 2. 船マスタ (Ship)

| No    | Tên JP                                             | Tên EN                                                                     | Kiểu   | Max    | Bắt buộc | Giá trị / Ghi chú                                                                                                                                                                           |
| ----- | -------------------------------------------------- | -------------------------------------------------------------------------- | ------ | ------ | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | ID                                                 | id                                                                         | 数値   | —      | ○        | —                                                                                                                                                                                           |
| 2     | 組織ID                                             | organization_id                                                            | 数値   | —      | ○        | FK → organization                                                                                                                                                                           |
| 3     | 名前                                               | name                                                                       | 文字列 | 40     | ○        | Unique/ORG                                                                                                                                                                                  |
| 4     | 名前（英語）                                       | name_en                                                                    | 文字列 | 80     | ○        | Unique/ORG                                                                                                                                                                                  |
| 5     | 画像                                               | images                                                                     | JSON   | —      | ○        | ≤3 files, png/jpg/jpeg, ≤5MB                                                                                                                                                                |
| 6     | 船内案内図                                         | map_image                                                                  | 文字列 | 50     | —        | 1 file, png/jpg/jpeg, ≤5MB                                                                                                                                                                  |
| 7     | 紹介文                                             | introduction                                                               | 文字列 | 200    | ○        | —                                                                                                                                                                                           |
| 8     | 紹介文（英語）                                     | introduction_en                                                            | 文字列 | 400    | ○        | —                                                                                                                                                                                           |
| 9     | 特徴                                               | feature                                                                    | JSON   | —      | —        | Đa chọn: 1:キッズルーム 2:コインロッカー 3:シャワールーム 4:ペットルーム 5:レストラン 6:売店 7:多目的トイレ 8:ゲームコーナー 9:大浴場 10:授乳室 11:エレベーター 12:エスカレーター 13:喫煙所 |
| 10-16 | 全長/全幅/総トン数/旅客定員/航海速力/積載台数(/EN) | length/width/gross_tonnage/passenger_capacity/speed/vehicles_capacity(/en) | 文字列 | 10-100 | —        | Thông tin kỹ thuật tàu                                                                                                                                                                      |
| 17    | ステータス                                         | status                                                                     | 数値   | —      | ○        | 1:有効 0:無効                                                                                                                                                                               |
| 18    | お知らせ                                           | announcement                                                               | 文字列 | 1000   | —        | Thông báo hiển thị trên booking page                                                                                                                                                        |
| 19    | お知らせ（英語）                                   | announcement_en                                                            | 文字列 | 2000   | —        | —                                                                                                                                                                                           |
| 20-21 | 登録/更新日時                                      | created/updated_datetime                                                   | 日時   | —      | ○        | —                                                                                                                                                                                           |

---

## 3. 船内施設マスタ (Ship Facility)

| No  | Tên JP           | Tên EN                   | Kiểu   | Max | Bắt buộc | Ghi chú                    |
| --- | ---------------- | ------------------------ | ------ | --- | -------- | -------------------------- |
| 1   | ID               | id                       | 数値   | —   | ○        | —                          |
| 2   | 船ID             | ship_id                  | 数値   | —   | ○        | FK → ship                  |
| 3   | タイトル         | title                    | 文字列 | 40  | ○        | △ Unique/Ship              |
| 4   | タイトル（英語） | title_en                 | 文字列 | 80  | ○        | △ Unique/Ship              |
| 5   | 画像             | image                    | 文字列 | 50  | ○        | 1 file, png/jpg/jpeg, ≤5MB |
| 6   | 案内文           | guide_content            | 文字列 | 200 | ○        | —                          |
| 7   | 案内文（英語）   | guide_content_en         | 文字列 | 400 | ○        | —                          |
| 8-9 | 登録/更新日時    | created/updated_datetime | 日時   | —   | ○        | —                          |

---

## 4. 客室マスタ (Cabin) — **Bảng quan trọng**

Quản lý cabin + booking limit phân chia Kagoshima/離島間 × WEB/Admin/Whole.

| No    | Tên JP                      | Tên EN                   | Kiểu        | Bắt buộc | Giá trị / Ghi chú                    |
| ----- | --------------------------- | ------------------------ | ----------- | -------- | ------------------------------------ |
| 1     | ID                          | id                       | 数値        | ○        | —                                    |
| 2     | 船ID                        | ship_id                  | 数値        | ○        | FK → ship                            |
| 3     | 客室名                      | name                     | 文字列(20)  | ○        | △ Unique/Ship                        |
| 4     | 客室名（英語）              | name_en                  | 文字列(40)  | ○        | △ Unique/Ship                        |
| 5     | 等級ID                      | cabin_class_id           | 数値        | ○        | Thuộc hạng nào                       |
| 6     | 様式                        | style                    | 数値        | ○        | 1:洋室 2:和洋室 3:和室               |
| 7     | 定員数                      | capacity                 | 数値        | ○        | Sức chứa tối đa                      |
| 8     | 相部屋可否                  | room_share               | 数値        | ○        | 0:不可 1:可                          |
| 9     | 予約上限数(鹿児島発着)/全体 | kagoshima_limit_whole    | 数値        | —        | Booking limit route Kagoshima        |
| 10    | 予約上限数(鹿児島発着)/WEB  | kagoshima_limit_web      | 数値        | —        | —                                    |
| 11    | 予約上限数(鹿児島発着)/管理 | kagoshima_limit_admin    | 数値        | —        | —                                    |
| 12    | アラート残数(鹿児島発着)    | kagoshima_alert_capacity | 数値        | —        | Ngưỡng cảnh báo sắp hết              |
| 13    | 予約上限数(離島間)/全体     | others_limit_whole       | 数値        | —        | Booking limit route khác             |
| 14    | 予約上限数(離島間)/WEB      | others_limit_web         | 数値        | —        | —                                    |
| 15    | 予約上限数(離島間)/管理     | others_limit_admin       | 数値        | —        | —                                    |
| 16    | アラート残数(離島間)        | others_alert_capacity    | 数値        | —        | —                                    |
| 17    | 客室区分                    | cabin_type               | 数値        | ○        | 0:無 1:寝台A 2:寝台B 3:寝台室 4:洋室 |
| 18    | 清掃区分                    | cleaning_type            | 数値        | ○        | 0:不要 1:必要                        |
| 19    | 客室画像                    | cabin_image              | JSON        | —        | ≤3 files                             |
| 20    | 紹介文                      | introduction             | 文字列(200) | —        | —                                    |
| 21    | 紹介文（英語）              | introduction_en          | 文字列(400) | —        | —                                    |
| 22    | 貸切最低人数                | minimum_charter          | 数値        | △        | Bắt buộc khi room_share=0            |
| 23    | 女性専用                    | woman_only               | 数値        | —        | —                                    |
| 24    | コード                      | code                     | 文字列(3)   | △        | Unique/Ship                          |
| 25-26 | 登録/更新日時               | created/updated_datetime | 日時        | ○        | —                                    |

**Kiến trúc booking limit**: Chia thành 2 nhóm route (Kagoshima↔đảo vs Giữa các đảo) × 3 kênh (Whole/WEB/Admin). Nếu WEB hết slot nhưng Admin còn → admin vẫn booking được.

---

## 5. 車両枠マスタ (Ship Car — Vehicle Frame)

Quản lý capacity xe theo tàu, phân chia Large/Small × Kagoshima/Others.

| No    | Tên JP               | Tên EN                    | Kiểu | Bắt buộc | Ghi chú                                                      |
| ----- | -------------------- | ------------------------- | ---- | -------- | ------------------------------------------------------------ |
| 1     | ID                   | id                        | 数値 | ○        | —                                                            |
| 2     | 船ID                 | ship_id                   | 数値 | ○        | FK → ship                                                    |
| 3     | 車長基準             | limit_car_length          | 数値 | ○        | Ngưỡng phân loại xe lớn/nhỏ                                  |
| 4     | 車高基準             | limit_car_height          | 数値 | ○        | —                                                            |
| 5     | 鹿児島発着大型枠上限 | large_car_kagoshima_limit | JSON | ○        | `{whole, admin, web, receptionStatus, alertCapacityMetrers}` |
| 11    | 鹿児島発着小型枠上限 | small_car_kagoshima_limit | JSON | ○        | Cấu trúc tương tự                                            |
| 12    | 離島間大型枠上限     | large_car_others_limit    | JSON | ○        | —                                                            |
| 13    | 離島間小型枠上限     | small_car_others_limit    | JSON | ○        | —                                                            |
| 14-15 | 登録/更新日時        | created/updated_datetime  | 日時 | ○        | —                                                            |

**JSON structure**: `receptionStatus` → trạng thái tiếp nhận (bookable/tentative/not bookable). `alertCapacityMetrers` → mét xe còn lại để cảnh báo.

---

## 6. 客室設備マスタ (Cabin Equipment)

| No  | Tên EN                   | Kiểu       | Ghi chú        |
| --- | ------------------------ | ---------- | -------------- |
| 1   | id                       | 数値       | —              |
| 2   | cabin_id                 | 数値       | FK → cabin     |
| 3   | name                     | 文字列(20) | △ Unique/Cabin |
| 4   | name_en                  | 文字列(40) | △ Unique/Cabin |
| 5   | quantity                 | 数値       | Số lượng       |
| 6-7 | created/updated_datetime | 日時       | —              |

---

## 7. ターミナルマスタ (Terminal)

| No    | Tên JP               | Tên EN                   | Kiểu   | Max | Bắt buộc | Ghi chú                                  |
| ----- | -------------------- | ------------------------ | ------ | --- | -------- | ---------------------------------------- |
| 1     | ID                   | id                       | 数値   | —   | ○        | —                                        |
| 2     | 組織ID               | organization_id          | 数値   | —   | ○        | FK → organization                        |
| 3     | ターミナル名         | name                     | 文字列 | 30  | ○        | △ Unique/ORG                             |
| 4     | ターミナル名（英語） | name_en                  | 文字列 | 60  | ○        | △ Unique/ORG                             |
| 5     | 郵便番号             | zip_code                 | 数値   | 7   | ○        | —                                        |
| 6     | 住所                 | address                  | 文字列 | 50  | ○        | —                                        |
| 7     | 紹介文               | introduction             | 文字列 | 200 | —        | —                                        |
| 8     | 営業時間             | opening_hours            | 文字列 | 20  | —        | —                                        |
| 9     | 電話番号             | telephone_number         | 数値   | 11  | —        | —                                        |
| 10    | メールアドレス       | mail_address             | 文字列 | 256 | —        | —                                        |
| 11-14 | 担当者/委託業者      | staff/subcontractor      | —      | —   | —        | Thông tin người liên hệ + đơn vị ủy thác |
| 15-16 | 登録/更新日時        | created/updated_datetime | 日時   | —   | ○        | —                                        |
| 17    | コード               | code                     | 数値   | —   | ○        | —                                        |

---

## 8. 航路マスタ (Route)

Quản lý tuyến đường kèm chi tiết các cảng dừng.

| No    | Tên JP                | Tên EN                   | Kiểu       | Bắt buộc | Giá trị / Ghi chú             |
| ----- | --------------------- | ------------------------ | ---------- | -------- | ----------------------------- |
| 1     | ID                    | id                       | 数値       | ○        | —                             |
| 2     | 組織ID                | organization_id          | 数値       | ○        | FK → organization             |
| 3     | 航路名                | route_name               | 文字列(30) | ○        | △ Unique/ORG                  |
| 4     | 航路名略称            | route_name_abbreviation  | 文字列(6)  | ○        | —                             |
| 5     | ターミナル詳細        | terminal_detail          | JSON       | ○        | Mảng các cảng dừng            |
| —     | └コード               | └code                    | 数値       | ○        | —                             |
| —     | └順番                 | └order                   | 数値       | ○        | Thứ tự dừng                   |
| —     | └ターミナルID         | └terminalId              | 数値       | ○        | FK → terminal                 |
| —     | └ターミナル名         | └terminalName            | 文字列     | ○        | —                             |
| —     | └ターミナル名（英語） | └terminalNameEn          | 文字列     | ○        | —                             |
| —     | └翌日着               | └isNextDayArrival        | 数値       | ○        | 0:OFF 1:ON (đến ngày hôm sau) |
| —     | └到着時間             | └arrivalTime             | 時刻       | —        | —                             |
| —     | └出発時間             | └departureTime           | 時刻       | —        | —                             |
| —     | └手仕舞い日           | └bookingDeadlineDay      | 数値       | ○        | 0~9 ngày trước khởi hành      |
| —     | └手仕舞い時間         | └bookingDeadlineTime     | 時刻       | ○        | —                             |
| 16-17 | 登録/更新日時         | created/updated_datetime | 日時       | ○        | —                             |
| 18    | 航行進路              | route_direction          | 数値       | ○        | Hướng đi (down/up)            |

---

## 9. 代理店マスタ (Agent)

| No  | Tên JP                    | Tên EN                 | Kiểu | Max | Bắt buộc | Giá trị / Ghi chú            |
| --- | ------------------------- | ---------------------- | ---- | --- | -------- | ---------------------------- |
| 1   | ID                        | id                     | 数値 | —   | ○        | —                            |
| 2   | 組織ID                    | organization_id        | 数値 | —   | ○        | FK → organization            |
| 3   | 区分                      | type                   | 数値 | —   | ○        | 1:海運代理店 2:旅行代理店    |
| 4   | 名前                      | name                   | 数値 | 40  | ○        | △ Unique/ORG                 |
| 5-8 | 郵便番号/住所/電話/メール | zip/address/tel/mail   | —    | —   | —        | —                            |
| 9   | 海運代理店ID              | shipping_agency_id     | 数値 | —   | △        | Bắt buộc + Unique khi type=1 |
| 10  | 管轄港ID                  | controlled_terminal_id | 数値 | —   | △        | Bắt buộc khi type=1          |

---

## 10. 管理者アカウント (Admin Account)

| No  | Tên JP         | Tên EN               | Kiểu   | Max | Bắt buộc | Giá trị                                         |
| --- | -------------- | -------------------- | ------ | --- | -------- | ----------------------------------------------- |
| 1   | 組織ID         | organization_id      | 数値   | —   | ○        | —                                               |
| 2   | アカウントID   | account_id           | 数値   | —   | ○        | —                                               |
| 3   | メールアドレス | account_mail_address | 文字列 | 256 | ○        | Unique                                          |
| 4   | 権限区分       | authority_type       | 数値   | —   | ○        | 1:管理者 2:海運代理店 3:旅行代理店 4:船内乗務員 |
| 5   | 氏名           | account_name         | 文字列 | 30  | ○        | —                                               |
| 6   | 代理店ID       | agent_id             | 数値   | —   | —        | Bắt buộc khi authority_type=2,3                 |

---

## 11. 会員情報 (Member)

| No  | Tên JP            | Tên EN                 | Kiểu   | Max | Bắt buộc | Giá trị                          |
| --- | ----------------- | ---------------------- | ------ | --- | -------- | -------------------------------- |
| 1   | ID                | id                     | 数値   | —   | ○        | —                                |
| 2-3 | 姓/名             | family/first_name      | 文字列 | 30  | —        | —                                |
| 4-5 | 姓(カナ)/名(カナ) | family/first_name_kana | 文字列 | 30  | ○        | —                                |
| 6   | 性別              | sex                    | 数値   | —   | ○        | 1:男 2:女                        |
| 7   | 生年月日          | birthday               | 日付   | —   | ○        | —                                |
| 8   | 郵便番号          | zip_code               | 数値   | 7   | —        | —                                |
| 9   | 都道府県          | prefectures            | 数値   | —   | —        | —                                |
| 10  | 住所              | address                | 文字列 | 50  | —        | —                                |
| 11  | 電話番号          | telephone_number       | 数値   | 11  | ○        | —                                |
| 12  | メールアドレス    | mail_address           | 文字列 | 256 | ○        | Unique                           |
| 13  | ステータス        | status                 | 数値   | —   | ○        | 0:仮登録 1:本登録                |
| 14  | 認証ID            | auth_id                | 文字列 | —   | ○        | Auth0 ID                         |
| 15  | 認証コード        | verification_code      | 数値   | —   | ○        | OTP code                         |
| 16  | 認証コード期限    | code_expired_datetime  | 日時   | —   | ○        | —                                |
| 17  | 国籍              | nationality            | 文字列 | 100 | ○        | —                                |
| 18  | 旅券番号          | passport_number        | 数値   | —   | △        | Bắt buộc khi nationality ≠ Japan |

---

## 12. 顧客情報 (Customer)

| No    | Tên JP                             | Tên EN                        | Kiểu   | Max | Bắt buộc | Giá trị / Ghi chú                          |
| ----- | ---------------------------------- | ----------------------------- | ------ | --- | -------- | ------------------------------------------ |
| 1     | 組織ID                             | organization_id               | 数値   | —   | ○        | —                                          |
| 2     | 顧客ID                             | customer_id                   | 数値   | —   | ○        | —                                          |
| 3     | 顧客区分                           | customer_type                 | 数値   | —   | ○        | 1:個人 2:法人                              |
| 4     | 会員ID                             | member_id                     | 数値   | —   | △        | Bắt buộc khi WEB booking (member→customer) |
| 5-8   | 姓/名/カナ                         | names                         | 文字列 | 30  | △        | Chỉ type=1                                 |
| 9     | 性別                               | sex                           | 数値   | —   | △        | 1:男 2:女 (chỉ type=1)                     |
| 10    | 生年月日                           | birthday                      | 日付   | —   | —        | Chỉ type=1                                 |
| 11    | 年齢                               | age                           | 数値   | —   | △        | Bắt buộc khi type=1                        |
| 12    | 法人名                             | corporate_name                | 文字列 | 50  | △        | Chỉ type=2, bắt buộc                       |
| 13-17 | 部署名/担当者                      | department/staff              | 文字列 | —   | △        | Chỉ type=2                                 |
| 18-22 | 郵便番号/都道府県/住所/電話/メール | zip/pref/addr/tel/mail        | —      | —   | —        | —                                          |
| 23    | 離島航空カード番号                 | remote_island_air_card_number | 数値   | 8   | —        | Thẻ ưu đãi đảo xa                          |
| 24    | 支払方法                           | payment_method                | 数値   | —   | —        | 1:Credit 2:現地 3:振込前 4:振込後          |
| 25    | 請求締め日                         | invoice_deadline              | 文字列 | 10  | —        | —                                          |
| 26    | paynowID                           | paynow_id                     | 文字列 | —   | —        | △ Unique/ORG                               |
| 27    | 登録アカウントID                   | registration_account_id       | 数値   | —   | —        | —                                          |
| 28    | 国籍                               | nationality                   | 文字列 | 100 | —        | —                                          |
| 29    | 旅券番号                           | passport_number               | 数値   | —   | △        | Bắt buộc khi nationality ≠ Japan           |

---

## 13. 運航スケジュール (Sailing Schedule) — **Bảng trung tâm**

Quản lý từng chuyến tàu cụ thể (ngày × tàu × route).

| No    | Tên JP                      | Tên EN                                            | Kiểu         | Bắt buộc | Giá trị / Ghi chú                     |
| ----- | --------------------------- | ------------------------------------------------- | ------------ | -------- | ------------------------------------- |
| 1     | スケジュールID              | schedule_id                                       | 数値         | ○        | —                                     |
| 2     | 組織ID                      | organization_id                                   | 数値         | ○        | —                                     |
| 3     | 臨時便フラグ                | isTemporary                                       | Boolean      | ○        | False:定期便 True:臨時便              |
| 4     | 航路                        | route                                             | JSON         | ○        | Snapshot route info                   |
| —     | └航路ID                     | └route_id                                         | 数値         | ○        | —                                     |
| —     | └航路名                     | └route_name                                       | 文字列       | ○        | —                                     |
| —     | └ターミナル詳細             | └terminal_detail                                  | JSON         | ○        | Copy từ route, không thay đổi         |
| —     | └└手仕舞い日/時間           | └└bookingDeadlineDay/Time                         | —            | ○        | Có thể thay đổi sau copy              |
| 16    | 船ID                        | ship_id                                           | 数値         | ○        | —                                     |
| 17    | 出発日時                    | departure_time                                    | 日時         | ○        | —                                     |
| 18    | 到着日時                    | arrival_time                                      | 日時         | ○        | —                                     |
| 19    | 予約上限数                  | booking_limit                                     | JSON         | ○        | Mảng per-cabin booking limits         |
| —     | └客室ID                     | └cabin_id                                         | 数値         | ○        | —                                     |
| —     | └Kagoshima limits           | └kagoshima_limit_whole/web/admin                  | 数値         | ○        | —                                     |
| —     | └Others limits              | └others_limit_whole/web/admin                     | 数値         | ○        | —                                     |
| —     | └アラート残数               | └alert_capacity                                   | 数値         | —        | —                                     |
| —     | └メモ                       | └remark                                           | 文字列(1000) | —        | —                                     |
| —     | └複数予約可否               | └multiple_reservations_allowed                    | Boolean      | ○        | —                                     |
| —     | └乗船申込可否               | └allow_application_form                           | Boolean      | ○        | —                                     |
| 33    | 運航ステータス              | sailing_status                                    | 数値         | ○        | 1:通常運航 2:条件付き/経路変更 3:欠航 |
| 34-37 | 条件付き/経路変更ターミナル | conditional/skip_terminal                         | JSON         | △        | Khi status=2                          |
| 38    | 手仕舞い日                  | booking_deadline_day                              | 数値         | ○        | 1:当日 2:前日                         |
| 39    | 手仕舞い時間                | booking_deadline_time                             | 時刻         | ○        | —                                     |
| 40-56 | 車両枠情報                  | car frame limits (large/small × kagoshima/others) | JSON         | —        | Tương tự ship_car nhưng per-schedule  |
| 57-58 | 大型/小型車両メモ           | vehicle_remark                                    | 文字列(1000) | —        | —                                     |
| 59    | コード                      | code                                              | 文字列(3)    | ○        | Unique                                |
| 60    | 年度                        | fiscal_year                                       | 数値(4)      | —        | —                                     |
| 61    | 次航                        | jikou                                             | 数値(2)      | —        | —                                     |
| 62    | お知らせ                    | announcement                                      | 文字列(1000) | —        | —                                     |
| 63    | お知らせ（英語）            | announcement_en                                   | 文字列(2000) | —        | —                                     |

---

## 14. 料金マスタ (Charge — Fare Table)

| No  | Tên JP             | Tên EN                       | Kiểu | Bắt buộc | Ghi chú          |
| --- | ------------------ | ---------------------------- | ---- | -------- | ---------------- |
| 1   | 料金ID             | charge_id                    | 数値 | ○        | —                |
| 2   | 組織ID             | organization_id              | 数値 | ○        | —                |
| 3   | 出発ターミナルID   | departure_terminal_id        | 数値 | ○        | —                |
| 4   | 到着ターミナルID   | arrival_terminal_id          | 数値 | ○        | —                |
| 5   | 特等               | special_class                | 金額 | —        | —                |
| 6   | 1等                | first_class                  | 金額 | —        | —                |
| 7   | 2等                | second_class                 | 金額 | —        | —                |
| 8   | 学割               | student_discount             | 金額 | —        | Giá sinh viên    |
| 9   | 寝台A              | bed_A                        | 金額 | —        | Phụ phí giường A |
| 10  | 寝台B              | bed_B                        | 金額 | —        | Phụ phí giường B |
| 11  | 寝台室             | bed_room                     | 金額 | —        | —                |
| 12  | 洋室               | western_room                 | 金額 | —        | —                |
| 13  | 乗用車運賃(3M未満) | passenger_vehicles_fare_base | 金額 | —        | Giá xe ≤3m       |
| 14  | 乗用車運賃(1M増毎) | passenger_vehicles_fare_add  | 金額 | —        | +mỗi 1m          |
| 15  | 貨物車運賃(3M未満) | truck_fare_base              | 金額 | —        | —                |
| 16  | 貨物車運賃(1M増毎) | truck_fare_base_add          | 金額 | —        | —                |
| 17  | 自転車             | bicycle                      | 金額 | —        | —                |
| 18  | 原動機付自転車     | motorized_bicycle            | 金額 | —        | —                |
| 19  | 自動二輪車         | motorbike                    | 金額 | —        | —                |
| 20  | ペット室利用       | pet_cage_use                 | 金額 | —        | —                |
| 21  | 受託手荷物         | checked_baggage              | 金額 | —        | —                |
| 22  | 有効期間開始日     | effective_period_start       | 日付 | ○        | —                |
| 23  | 有効期間終了日     | effective_period_end         | 日付 | —        | —                |

---

## 15. 割引マスタ (Discount) — **Bảng phức tạp**

| No    | Tên JP           | Tên EN                         | Kiểu         | Bắt buộc | Giá trị / Ghi chú                                                                  |
| ----- | ---------------- | ------------------------------ | ------------ | -------- | ---------------------------------------------------------------------------------- |
| 1     | 割引ID           | discount_id                    | 数値         | ○        | —                                                                                  |
| 2     | 組織ID           | organization_id                | 数値         | ○        | —                                                                                  |
| 3     | 割引名           | discount_name                  | 文字列(20)   | ○        | △ Unique/ORG + thời hạn                                                            |
| 4     | 割引名（英語）   | discount_name_en               | 文字列(100)  | △        | Bắt buộc khi WEB hiển thị                                                          |
| 5     | 適用条件         | discount_condition             | 文字列(500)  | ○        | Mô tả điều kiện áp dụng                                                            |
| 6     | 適用条件（英語） | discount_condition_en          | 文字列(1000) | △        | —                                                                                  |
| 7     | 略称             | abbreviation                   | String(50)   | —        | —                                                                                  |
| 8     | WEB予約表示      | web_booking_available          | Boolean      | —        | Hiển thị trên WEB hay không                                                        |
| 9     | 適用方法         | method                         | JSON         | —        | —                                                                                  |
| —     | └WEB             | └web                           | 数値         | —        | 1:manual 2:auto 3:not apply                                                        |
| —     | └管理            | └admin                         | 数値         | —        | 1:manual 2:auto 3:not apply                                                        |
| 12    | 選択権限         | admin_permissions              | JSON         | —        | —                                                                                  |
| —     | └管理者          | └admin                         | Boolean      | —        | —                                                                                  |
| —     | └海運代理店      | └agent_shipping                | Boolean      | —        | —                                                                                  |
| —     | └旅行代理店      | └agent_travel                  | Boolean      | —        | —                                                                                  |
| 16    | 仮予約           | booking_temp_with_agent        | Boolean      | —        | =1 khi áp dụng discount → booking temp                                             |
| 17    | 併用対象         | colectively_with               | JSON         | —        | List discount IDs có thể dùng chung. Nguyên tắc: 1 individual + 1 auto per booking |
| 18    | 適用金額         | apply_charge                   | JSON         | —        | Loại phí áp dụng discount                                                          |
| —     | └特等/1等/2等    | └special/first/second_class    | Boolean      | —        | —                                                                                  |
| —     | └等級差額        | └cabin_class_difference        | Boolean      | —        | —                                                                                  |
| —     | └寝台差額        | └cabin_type_difference         | Boolean      | —        | —                                                                                  |
| —     | └BAF             | └baf                           | Boolean      | —        | —                                                                                  |
| —     | └車両運賃        | └car                           | Boolean      | —        | —                                                                                  |
| 26    | 割引区分         | value_type                     | 数値         | —        | 1:%(割合) 2:¥(金額) 3:m(メートル)                                                  |
| 27    | 割引値           | value                          | 数値         | —        | —                                                                                  |
| 28    | 割引値（小人）   | value_children                 | 数値         | —        | Chỉ khi value_type=2                                                               |
| 29    | 割引最大値       | value_max                      | 数値         | —        | Chỉ khi value_type=1 (% giảm max)                                                  |
| 30    | 直接入力可否     | value_manual                   | Boolean      | —        | Cho phép nhập trực tiếp                                                            |
| 31    | 適用区間         | apply_route                    | JSON         | —        | —                                                                                  |
| —     | └全区間          | └is_all                        | Boolean      | —        | Áp dụng tất cả tuyến                                                               |
| —     | └対象区間        | └accepted_list                 | Array        | —        | Khi is_all=0                                                                       |
| —     | └対象外区間      | └rejected_list                 | Array        | —        | Khi is_all=1                                                                       |
| 35    | 割引内容         | discount_detail                | 文字列(50)   | ○        | —                                                                                  |
| 36    | 適用往復区分     | round-trip_type_apply          | 数値         | ○        | 0:全て 1:片道のみ 2:復路のみ                                                       |
| 37    | 適用範囲         | scope                          | 数値         | ○        | 1:個人 2:全体 3:団体                                                               |
| 38    | 適用予約区分     | apply_for_booking_type         | JSON         | ○        | 1:一般旅客 2:一般車両 3:団体旅客 4:団体車両                                        |
| 39    | 関係者適用       | apply_to_concerned_individuals | Boolean      | —        | Áp dụng cho người đi kèm (người chăm sóc)                                          |
| 40    | 関係性           | relationships                  | 数値         | △        | 1:介護人 2:付添人                                                                  |
| 41    | 適用属性         | apply_attribute                | JSON         | —        | null:全属性 1:大人 2:小人 3:幼児 4:乳児                                            |
| 42    | 運転手適用       | apply_driver                   | Boolean      | —        | —                                                                                  |
| 43-44 | 有効期間         | effective_period_start/end     | 日時         | ○/—      | —                                                                                  |

---

## 16. BAFマスタ (Bunker Adjustment Factor)

| No   | Tên JP         | Tên EN                     | Kiểu | Bắt buộc | Giá trị                 |
| ---- | -------------- | -------------------------- | ---- | -------- | ----------------------- |
| 1    | BAF ID         | baf_id                     | 数値 | ○        | —                       |
| 2    | 組織ID         | organization_id            | 数値 | ○        | —                       |
| 3    | 航路タイプ     | route_type                 | 数値 | ○        | 1:鹿児島発着 2:それ以外 |
| 4    | ランク         | rank                       | 数値 | ○        | 1~15                    |
| 5    | 旅客           | passenger                  | 金額 | ○        | —                       |
| 6    | 乗用車         | passenger_vehicle          | 金額 | ○        | —                       |
| 7    | 貨物車(6m未満) | truck_under_6m             | 金額 | ○        | —                       |
| 8    | 貨物車(6m以上) | truck_over_6m              | 金額 | ○        | —                       |
| 9-10 | 有効期間       | effective_period_start/end | 日付 | ○        | Thay đổi mỗi 3 tháng    |

---

## 17. 予約 (Booking) — **Bảng lớn nhất, 134 fields**

| No      | Tên JP                            | Tên EN                                           | Kiểu        | Giá trị / Ghi chú                                                                                        |
| ------- | --------------------------------- | ------------------------------------------------ | ----------- | -------------------------------------------------------------------------------------------------------- |
| 1       | 組織ID                            | organization_id                                  | 数値        | —                                                                                                        |
| 2       | ID                                | id                                               | 数値        | Unique                                                                                                   |
| 3       | 予約経路                          | booking_channel                                  | 数値        | 1:WEB 2:管理                                                                                             |
| 4       | 予約ステータス                    | status                                           | 数値        | **-1:一時予約 0:キャンセル 1:仮予約 2:本予約 4:引換券登録済 5:乗船券登録済 8:キャンセル待ち 9:乗船済み** |
| 5       | 予約区分                          | type                                             | 数値        | 1:一般旅客 2:一般車両 3:団体旅客 4:団体車両 9:無人車                                                     |
| 6       | 予約番号                          | booking_number                                   | 文字列(20)  | △ Unique/Schedule                                                                                        |
| 7       | 運航スケジュールID                | sailing_schedule_id                              | 数値        | FK → sailing_schedule                                                                                    |
| 8-9     | 出発/到着日時                     | departure/arrival_datetime                       | 日付        | —                                                                                                        |
| 10-11   | 出発/到着ターミナルID             | departure/arrival_terminal_id                    | 数値        | —                                                                                                        |
| 12      | 区間コード                        | stage_code                                       | 数値        | —                                                                                                        |
| 13      | 客室ID                            | cabin_id                                         | 数値        | △ (不要 khi type=9 無人車)                                                                               |
| 14-19   | 客室詳細                          | cabin_detail (JSON)                              | —           | cabin_name, cabin_class, room_share, cabin_type, cleaning_type                                           |
| 20      | 顧客ID                            | customer_id                                      | 数値        | ○                                                                                                        |
| 21      | 電話番号                          | telephone_number                                 | 数値        | ○                                                                                                        |
| 22-25   | 大人/小人/幼児/乳児人数           | adult/child/toddler/infant_count                 | 数値        | △                                                                                                        |
| 26      | 乗船者                            | passengers                                       | JSON        | △ Mảng thông tin hành khách                                                                              |
| —       | └乗船者ID                         | └passenger_id                                    | 数値        | —                                                                                                        |
| —       | └代表者                           | └is_representative                               | 数値        | 0:無 1:有                                                                                                |
| —       | └姓名(カナ)/性別/年齢/属性        | └name_kana/sex/age/attribute                     | —           | attribute: 1:大人 2:小人 3:幼児 4:乳児                                                                   |
| —       | └都道府県/住所/国籍/旅券番号      | └prefectures/address/nationality/passport        | —           | —                                                                                                        |
| —       | └基本運賃                         | └fare                                            | 金額        | —                                                                                                        |
| —       | └等級差額/客室区分料金/BAF/幅割増 | └class_diff/cabin_type/baf/width_charge          | 金額        | —                                                                                                        |
| —       | └自動割引ID/額                    | └auto_discount_id/amount                         | —           | —                                                                                                        |
| —       | └個人割引ID/額                    | └personal_discount_id/amount                     | —           | —                                                                                                        |
| —       | └旅客/車両割引率                  | └passenger/vehicle_discount_percentage           | 数値        | —                                                                                                        |
| —       | └割引m数                          | └discount_meters                                 | 数値        | —                                                                                                        |
| —       | └合計金額                         | └total_amount                                    | 金額        | —                                                                                                        |
| —       | └関係者                           | └concerned_individual                            | 数値        | 介護/付添の乗船者ID                                                                                      |
| —       | └会社名/会社電話                  | └company_name/phone                              | String      | Phase2追加                                                                                               |
| —       | └領収書宛名                       | └receiver_name                                   | String      | Phase2追加                                                                                               |
| 59      | 料金ID                            | charge_id                                        | 数値        | ○ FK → charge                                                                                            |
| 60      | BAF ID                            | baf_id                                           | 数値        | ○ FK → baf                                                                                               |
| 61-63   | 妊婦/妊娠週数/海難救護            | pregnant/gestational_weeks/sea_rescue            | 数値        | —                                                                                                        |
| 64      | 緊急連絡先                        | emergency_contact                                | 数値        | △                                                                                                        |
| 65      | メールアドレス                    | mail_address                                     | 文字列(256) | △                                                                                                        |
| 66-67   | 代理店ID/名                       | agent_id/name                                    | —           | —                                                                                                        |
| 68      | ペット室                          | pet_cage                                         | 数値        | —                                                                                                        |
| 69-70   | クーポン名/詳細                   | coupon_name/detail                               | 文字列      | —                                                                                                        |
| 71-73   | 全体/団体/overall割引ID           | general/group/overall_discount_id                | 数値        | —                                                                                                        |
| 74      | 支払方法                          | payment_method                                   | 数値        | 1:SF 2:現地 3:前振込 4:後請求 5:船車券 6:船車券/手書き 7:バウチャー 8:クーポン                           |
| 75      | 支払状況                          | payment_status                                   | 数値        | 0:未入金 1:与信済 2:入金済                                                                               |
| 76-78   | 請求関連                          | invoice_deadline/payment_due_date/invoice_remark | —           | —                                                                                                        |
| 79      | 料金詳細                          | fareDetail                                       | JSON        | exclusive_charge, breakfast/lunch/dinner_charge, discount合計各種                                        |
| 89      | 合計金額                          | total_amount                                     | 金額        | ○                                                                                                        |
| 90      | ペット室利用料金                  | pet_cage_charge                                  | 金額        | —                                                                                                        |
| 91      | 備考                              | remark                                           | 文字列(500) | —                                                                                                        |
| 92-98   | キャンセル関連                    | cancel_fee_type/fee/partial_detail/notification  | —           | cancel_fee_type: 1:0円 2:200円 3:10% 4:30% 5:100% 6:その他                                               |
| 101-105 | 登録/更新者ID/日時                | registrant/updater/created/updated               | —           | —                                                                                                        |
| 106     | 関連予約ID                        | related_booking_id                               | 数値        | Booking liên quan (往復 round-trip)                                                                      |
| 107     | 仮予約理由                        | temporary_booking_reason                         | JSON        | 1:車両枠 2:特殊仕様 3:代理店団体 4:代理店割引 5:便変更 6:往復割引過剰 7:クレジット中断 8:危険物          |
| 108     | 一時予約有効期限                  | temporary_booking_expiration                     | 日時        | —                                                                                                        |
| 109-111 | 特記事項                          | special_note/list/post_deadline_change_list      | —           | —                                                                                                        |
| 112     | 元予約ID                          | original_booking_id                              | 数値        | △ 変更フロー用                                                                                           |
| 113-120 | 引換券/乗船券詳細                 | exchange/boarding_ticket                         | JSON        | ticket_procedure_id + passenger_id                                                                       |
| 121-127 | アラート                          | alerts                                           | JSON        | type: 1:往復割引過剰 2:クレジット返金エラー 3:クレジット決済スキップ                                     |
| 128     | グループ予約紐づけID              | group_booking_link_id                            | 数値        | —                                                                                                        |
| 129     | 決済完了フラグ                    | payment_completed                                | 数値        | 登録/変更ステップ完了                                                                                    |
| 130     | 強制変更フラグ                    | is_force_updated_credit_fail                     | boolean     | —                                                                                                        |
| 131     | キャンセル日時                    | cancellation_datetime                            | 日時        | —                                                                                                        |
| 132-133 | 請求先                            | billing_recipient_id/name                        | —           | Phase2追加                                                                                               |
| 134     | 予約締め切りリマインド送信有無    | is_booking_deadline_reminder_sent                | boolean     | —                                                                                                        |

---

## 18-23. Bảng lịch sử & phụ trợ

### 予約履歴 (Booking History)

- Cấu trúc tương tự bảng 予約, thêm history_id riêng.

### 予約変更依頼 (Change Booking Request)

- Dùng cho flow agent yêu cầu thay đổi booking → admin xác nhận.
- Fields: organization_id, booking_id, agent_id, change_status (1:管理者確認中 2:代理店確認中 9:変更完了), messages (JSON array với owner_id, content, created_datetime).

### 予約車両 (Booking Car)

- Quản lý danh sách xe trong booking.
- car_detail (JSON): passenger_id, type (1:自動車 2:特殊手荷物), carType (1:乗用車 2:貨物車), length/height/width, roof_box, dangerous_goods, driver_telephone, license plate (registered_place/class_number/hiragana/vehicle_number), special_spec, wheelchair, specialBaggageType (1:自転車 2:原付~124cc 3:自動二輪125~749cc 4:自動二輪750cc~).
- remark, payment_method (1:元払い 2:着払い 3:前振込 4:後請求).

### 予約団体 (Booking Group)

- group_name, group_detail (JSON: attribute + count), meal (0:無 1:有), remark.
- passengerCountInfo (JSON): Thống kê theo giới tính × thuộc tính (adultFemale/Male, childFemale/Male, infant/toddler/driver).
- voucherBookingGroupInputInfo (JSON): Giá trị voucher cho xe + hành khách nhóm.

---

## 24. 乗船券 (Boarding Ticket) — **137 fields**

Bảng lớn nhất, quản lý vé lên tàu với mọi thông tin fare chi tiết per-passenger.

**Status flow**: -1:ダミー → 1:登録済み → 2:発券済み → 3:乗船済み → 4:下船済み. 0:キャンセル, 9:VOID

**Cấu trúc chính**:

- Thông tin chuyến: sailing_schedule_id, departure/arrival datetime/terminal
- Thông tin cabin: cabin_detail (JSON)
- Passengers (JSON array): mỗi passenger có fare breakdown đầy đủ (fare, baf_charge, class_price_difference, cabin_type_charge, width_additional_charge, auto/personal_discount, boardingStatus: 1:乗船済み 2:下船済み)
- Fare detail (JSON): exclusive_charge, breakfast/lunch/dinner_charge, discount totals
- Payment: payment_method (0:SF 1:現金 2:現地カード 3:後請求 4:前振込 5:船車券 6:船車券/手書き 7:バウチャー 8:クーポン 9:返金), payment_status
- Cancel: cancel_fee_type, cancel_fee, refund_amount
- Phase2 additions: 追加精算/船内精算 (method, amount, reason), ticket-level cancel fields
- Issuance log (JSON): issuance_count, issuance_datetime
- round_trip_type: 0:片道 1:往路 2:復路

---

## 26. 引換券 (Exchange Ticket)

Vé đổi — dùng khi khách mua trước, đổi lấy boarding ticket tại quầy.

**Status**: 0:キャンセル → 1:登録済み → 2:発券済み → 3:不乗船 → 4:一部引換済み → 5:全額引換済み. 9:VOID

Cấu trúc gần giống boarding_ticket, nhưng thêm: expiration_date, unused_amount, transfer_boarding_ticket_number.

---

## 28. 荷物 (Baggage) — 49 fields

| Nhóm              | Fields                                           | Ghi chú                                                          |
| ----------------- | ------------------------------------------------ | ---------------------------------------------------------------- |
| Pet               | pet_cage_count, pet_cage_charge                  | Phí phòng thú cưng                                               |
| Checked baggage   | checked_baggage_count/charge                     | Hành lý ký gửi                                                   |
| Bicycle           | bicycle_count/charge                             | —                                                                |
| Motorized bicycle | motorized_bicycle_count/charge                   | ≤124cc                                                           |
| Motorbike         | motorbike_count/charge (125~749cc)               | —                                                                |
| Large motorbike   | large_displacement_count/charge (≥750cc)         | —                                                                |
| Tag               | baggage_tag_number (JSON)                        | Số nhãn hành lý                                                  |
| Special           | is_special_baggage_paid, is_special_baggage_free | —                                                                |
| Free flags        | is_checked_baggage_free, is_pet_cage_free        | Phase2                                                           |
| WEB paid special  | special_baggage_web_paid (JSON)                  | BaggageType, PaymentAmount                                       |
| Payment detail    | payment_detail (JSON)                            | PaymentMethod, BaggageType, Count, Amount, Route (0:窓口 1:船内) |
| Alerts            | alerts (JSON)                                    | type: 1:要クレジット一部返金                                     |

---

## 29-36. Bảng hỗ trợ & thanh toán

### クーポン (Coupon)

- code (unique), name (unique), agent_id, isInvalid (0:有効 1:無効).

### 発券手続き (Ticket Procedure)

- procedure_type (1:当日売り 2:前売り), roundtrip_type (1:片道 2:往復), preset_type (0:無し 1:引換券 2:予約).

### 表示項目定義 (Display Item Definition)

- Cho phép admin tùy biến columns hiển thị trong booking list/ticket list.
- share_type (0:不可 1:可能), function_type (1:予約一覧 2:発券一覧).

### クレジットトランザクション (Credit Transaction)

- action: 1:AUTHORIZE 2:AUTHORIZE_CAPTURE 3:CAPTURE 4:CANCEL 5:REAUTHORIZE 6:REAUTHORIZE_CAPTURE
- session_type: 8 loại session (Web Register Auth, Web Register Auth+Capture, Web Update Auth+Refund, Web Update Auth+Capture+Refund, Web&Admin Update Capture, Admin Update Reauth+Refund, Admin Update Reauth+Capture+Refund, Web&Admin Cancel Entire Refund)
- booking_action: 1:予約登録/WEB ~ 7:予約キャンセル/管理

### 操作ログ (Operation Log)

- system_type (1:WEB 2:管理), operator_type (1:ユーザ 2:管理者 3:海運代理店 4:旅行代理店).
- operation: 1:予約CSVダウンロード 2:領収書ダウンロード 3:発券CSVダウンロード.

### 車両残数 (Vehicle Capacity)

- car_type (0:small 1:large), capacity_type (0:web 1:admin 2:whole).

### 添付ファイル (Attachment)

- booking_id, file_name, file_path, soft delete support.

### 領収書発行記録 (Receipt Issuance Record)

- booking_id, payment_date, amount, pdf_path, issuer_id (= customer_id).

---

## 37. アカウント設定 (Account Config)

| No  | Tên EN          | Kiểu     | Giá trị                                        |
| --- | --------------- | -------- | ---------------------------------------------- |
| 1   | id              | int      | Auto increment                                 |
| 2   | organization_id | int      | FK → organization                              |
| 3   | account_id      | int      | FK → account                                   |
| 4   | type            | tiny_int | 1:Booking list bookmark 2:Ticket list bookmark |
| 5   | config          | JSON     | Cấu hình tùy biến                              |

---

## 38. 乗船申込手続きQR (Application Form QR) — Phase 3

| No  | Tên JP                  | Tên EN                             | Kiểu     | Bắt buộc | Ghi chú                       |
| --- | ----------------------- | ---------------------------------- | -------- | -------- | ----------------------------- |
| 1   | ID                      | id                                 | Number   | ○        | —                             |
| 2   | 組織ID                  | organization_id                    | Number   | ○        | Xác định QR thuộc công ty nào |
| 3   | 出発港                  | departure_terminal_id              | Number   | ○        | —                             |
| 4   | バリデーション用コード  | validation_code                    | String   | ○        | —                             |
| 5   | QR有効期限              | expired                            | Datetime | ○        | —                             |
| 6-9 | 登録者/日時/更新者/日時 | registrant/created/updater/updated | —        | —        | —                             |

---

## 39. 乗船申込 (Application Form) — Phase 3

| No    | Tên JP                     | Tên EN                                             | Kiểu   | Bắt buộc | Giá trị / Ghi chú                                                                                                                                                    |
| ----- | -------------------------- | -------------------------------------------------- | ------ | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | ID                         | id                                                 | Number | ○        | —                                                                                                                                                                    |
| 2     | 組織ID                     | organization_id                                    | Number | ○        | Filter organization                                                                                                                                                  |
| 3     | ステータス                 | status                                             | Number | ○        | 1:予約データ未反映 9:予約データ反映済み                                                                                                                              |
| 4     | タイプ                     | type                                               | Number | ○        | 1:Web booking 2:Admin booking 3:No booking                                                                                                                           |
| 5     | 関連予約ID                 | booking_id                                         | Number | —        | —                                                                                                                                                                    |
| 6-8   | Schedule/Terminal IDs      | sailing_schedule_id, departure/arrival_terminal_id | Number | —        | —                                                                                                                                                                    |
| 9     | 客室ID                     | cabin_id                                           | Number | —        | —                                                                                                                                                                    |
| 10-13 | 人数 (大人/小人/幼児/乳児) | adult/child/toddler/infant_count                   | Number | —        | —                                                                                                                                                                    |
| 14    | 乗船者                     | passengers                                         | JSON   | —        | Array: passenger_id, is_representative, name_kana, sex, age, attribute, prefectures, address, nationality, passport_number, discount IDs, company_name, phone_number |
| 37    | 車両詳細                   | car_detail                                         | JSON   | —        | carType, name, shape, length/height/width, roof_box, dangerous_goods, license plate, special_spec, wheelchair                                                        |
| 54    | 特殊手荷物詳細             | special_baggage_detail                             | JSON   | —        | 1:自転車 2:原付 3:自動二輪125~749 4:自動二輪750~                                                                                                                     |
| 57-58 | 妊娠週数/海難救護          | gestational_weeks/sea_rescue                       | Number | —        | —                                                                                                                                                                    |

---

## 39b. 不乗船証明書 (Refund Certificate) — Phase 3

| No  | Tên JP         | Tên EN                | Kiểu     | Bắt buộc | Giá trị                                   |
| --- | -------------- | --------------------- | -------- | -------- | ----------------------------------------- |
| 1   | id             | id                    | number   | ○        | —                                         |
| 2   | 組織ID         | organization_id       | number   | ○        | —                                         |
| 3   | ステータス     | status                | number   | ○        | 0:登録済み 1:発券済み                     |
| 4   | 乗船券ID       | boarding_ticket_id    | number   | ○        | —                                         |
| 5   | 発券手続ID     | ticket_procedure_id   | number   | ○        | —                                         |
| 6   | 引換券発行店   | issuing_counter       | String   | ○        | —                                         |
| 7   | 引換券No       | issuing_number        | String   | ○        | —                                         |
| 8   | お客様名       | passenger_name        | String   | ○        | —                                         |
| 9   | 不乗船理由     | non_boarding_reason   | JSON     | ○        | {type, content}                           |
| 12  | 券面金額       | total_exchange_amount | number   | ○        | —                                         |
| 13  | 不乗船金額     | unused_amount         | number   | ○        | —                                         |
| 14  | 払戻区分       | cancel_fee_type       | number   | —        | 1:0円 2:200円 3:10% 4:30% 5:100% 6:その他 |
| 15  | 払戻手数料     | cancel_fee            | number   | ○        | —                                         |
| 16  | 払戻金額       | refund_amount         | number   | ○        | —                                         |
| 17  | 発行日         | issued_datetime       | Datetime | —        | —                                         |
| 18  | 不乗船証明書No | certificate_no        | String   | ○        | —                                         |
