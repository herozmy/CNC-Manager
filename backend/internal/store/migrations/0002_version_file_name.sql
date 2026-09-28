-- 0002_version_file_name.sql —— 让每个版本记住自己的文件名
--
-- 背景（实测发现的问题）：
--   nc_file 是按内容 sha256 全局共享的，一行 nc_file 会被多个程序、多个版本引用。
--   原先文件名只存在 nc_file.original_name 上，于是出现这样的现象：
--   程序员给程序 B 上传了一个叫 O5678.nc 的文件，只要它和程序 A 已有的某个文件
--   内容完全相同，版本列表里显示的就是 A 的文件名 O1234_v1.nc ——
--   会让人以为传错文件了。
--
-- 修正方式：物理文件继续共享（省空间、可秒传），但文件名落在 nc_version 上。
-- 每个版本记录"用户当时用的那个文件名"，这才是使用者真正关心的信息。

ALTER TABLE nc_version ADD COLUMN file_name TEXT NOT NULL DEFAULT '';

-- 回填历史数据：老版本的版本文件名沿用其物理文件第一次上传时的名字
UPDATE nc_version
SET file_name = COALESCE((SELECT f.original_name FROM nc_file f WHERE f.id = nc_version.file_id), '')
WHERE file_name = '';
