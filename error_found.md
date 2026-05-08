## đã sửa

# lỗi set chapter thấp hơn chapter hiện tại
khi set chapter 100 xong set chapter 10 thì hệ thông vẫn nhận chapter 10 (phải dùng --force như cli_manual)

# chưa có Update Library Entry
mangahub library update --manga-id <id> --status <new-status>
# Example
mangahub library update --manga-id one-piece --status completed --rating 10

# thêm update notes cho .\mangahub progress update --manga-id one-piece --chapter 100 --notes "Great ending!"

## chưa sửa
##############################
# sửa validate email trong auth register

# sửa thêm login bằng email

# thêm .\mangahub server status

# thêm chi tiết cho .\mangahub manga info <id>

# thêm page và limit cho .\mangahub manga list --page 2 --limit 20 

# thêm số lượng khi list (đọc trong cli_manual: mangahub library list)

# thêm advance search option cho .\mangahub manga search --genre shounen --status completed --page 1 --limit 20 --sort-by title --order asc

# thêm rating cho .\mangahub library add --manga-id one-piece --status reading --rating 10

# thêm sort cho .\mangahub library list --sort-by title --order asc và mangahub library list --sort-by last-updated --order desc

# thêm volume khi dùng progress update # With additional info
mangahub progress update --manga-id <id> --chapter <number> --volume <number>

# thêm view progress history manga (xem progress hiện tại của manga)
mangahub progress history --manga-id <id>

# View all progress updates
mangahub progress history