-- 0004_tool_comp_and_file_encoding.sql
--
-- 两个改动，都是现场提出来后补的。
--
-- 一、刀具补偿量（独立字段）
--   现场要记录的「刀具补偿量」是一个独立数值，不等于刀具直径，也不等于刀尖圆弧。
--   单位 mm，沿用全库约定以「实际值 × 1000」的整数落库。
--
-- 二、程序文件编码
--   现场很多 NC 程序的注释是中文，而且是用老编辑器存成的 GBK(ANSI)。
--   如果软件一律按 UTF-8 读，中文注释会变成乱码；
--   用户在软件里改完再存回去，就会把整个程序文件的编码毁掉——机床可能直接不认。
--   所以文件内容是什么编码必须记下来，读的时候按它解码，写的时候按它编码。
--   取值：'utf-8'（含纯 ASCII）或 'gbk'。

ALTER TABLE program_tool ADD COLUMN comp_amount_milli INTEGER NOT NULL DEFAULT 0;

ALTER TABLE nc_file ADD COLUMN file_encoding TEXT NOT NULL DEFAULT 'utf-8';
