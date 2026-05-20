# Sửa label ở màn hình đăng ký member

## Requirement

Update label お名前（カナ）thành "xxx" ở màn hình đăng ký user

---

## US-1: Cập nhật nhãn お名前（カナ） thành 'xxx' trên màn hình đăng ký user

> Là một người quản trị, tôi muốn nhãn 'お名前（カナ）' trên màn hình đăng ký user được cập nhật thành 'xxx', để đảm bảo giao diện phù hợp với yêu cầu mới.

### Acceptance Criteria

1. Given tôi đang ở màn hình đăng ký user, When tôi nhìn thấy trường nhập tên bằng Katakana, Then nhãn của trường này phải hiển thị là 'xxx' thay vì 'お名前（カナ）'.
2. Given tôi kiểm tra toàn bộ màn hình đăng ký user, When tôi tìm kiếm nhãn 'お名前（カナ）', Then không còn xuất hiện nhãn này ở bất kỳ vị trí nào.

### Test Cases

**TC-1: Kiểm tra nhãn trường nhập Katakana đã đổi thành 'xxx'**
- Steps: 1. Truy cập màn hình đăng ký user.
2. Xác định trường nhập tên bằng Katakana.
3. Quan sát nhãn của trường này.
- Expected: Nhãn trường nhập Katakana hiển thị là 'xxx'.

**TC-2: Đảm bảo không còn nhãn 'お名前（カナ）' trên màn hình**
- Steps: 1. Truy cập màn hình đăng ký user.
2. Kiểm tra toàn bộ các nhãn trên màn hình.
- Expected: Không còn nhãn nào hiển thị là 'お名前（カナ）'.

---

## US-2: Cập nhật label 'お名前（カナ）' thành 'xxx' trên tất cả các màn hình

> Là một quản trị viên, tôi muốn label 'お名前（カナ）' được cập nhật thành 'xxx' trên tất cả các màn hình có sử dụng label này, để đảm bảo tính nhất quán và dễ hiểu cho người dùng.

### Acceptance Criteria

1. Given tôi đang ở bất kỳ màn hình nào có label 'お名前（カナ）', When tôi truy cập màn hình đó, Then label phải hiển thị là 'xxx' thay vì 'お名前（カナ）'.
2. Given tôi kiểm tra màn hình đăng ký user, When tôi xem trường nhập tên, Then label phải là 'xxx'.
3. Given tôi kiểm tra các màn hình khác có sử dụng label này (ví dụ: chỉnh sửa thông tin user, xem chi tiết user), When tôi truy cập các màn hình đó, Then label cũng phải là 'xxx'.

### Test Cases

**TC-1: Kiểm tra label trên màn hình đăng ký user**
- Steps: 1. Truy cập màn hình đăng ký user
2. Quan sát trường nhập tên (trước đây là 'お名前（カナ）')
- Expected: Label hiển thị là 'xxx'

**TC-2: Kiểm tra label trên màn hình chỉnh sửa thông tin user**
- Steps: 1. Truy cập màn hình chỉnh sửa thông tin user
2. Quan sát trường nhập tên (trước đây là 'お名前（カナ）')
- Expected: Label hiển thị là 'xxx'

**TC-3: Kiểm tra label trên màn hình xem chi tiết user**
- Steps: 1. Truy cập màn hình xem chi tiết user
2. Quan sát thông tin tên (trước đây là 'お名前（カナ）')
- Expected: Label hiển thị là 'xxx'

**TC-4: Kiểm tra label trên các màn hình khác có sử dụng label này**
- Steps: 1. Truy cập từng màn hình có sử dụng label 'お名前（カナ）'
2. Quan sát label tại các vị trí liên quan
- Expected: Tất cả các label đều hiển thị là 'xxx'

---

