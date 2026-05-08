# lỗi set chapter thấp hơn chapter hiện tại
khi set chapter 100 xong set chapter 10 thì hệ thông vẫn nhận chapter 10 (phải dùng --force như cli_manual)

# chưa có volume khi dùng progress update
.\mangahub.exe progress update --manga-id naruto --chapter 700 --volume 72 --notes "Great ending!"

# chưa có Update Library Entry
mangahub library update --manga-id <id> --status <new-status>
# Example
mangahub library update --manga-id one-piece --status completed --rating 10

