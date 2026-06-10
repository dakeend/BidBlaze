#!/usr/bin/env bash
set -euo pipefail

sudo mysql <<'SQL'
CREATE DATABASE IF NOT EXISTS auction CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'auction'@'localhost' IDENTIFIED BY 'auction_root';
CREATE USER IF NOT EXISTS 'auction'@'127.0.0.1' IDENTIFIED BY 'auction_root';
CREATE USER IF NOT EXISTS 'auction'@'%' IDENTIFIED BY 'auction_root';
GRANT ALL PRIVILEGES ON auction.* TO 'auction'@'localhost';
GRANT ALL PRIVILEGES ON auction.* TO 'auction'@'127.0.0.1';
GRANT ALL PRIVILEGES ON auction.* TO 'auction'@'%';
FLUSH PRIVILEGES;
SQL

mysql -uauction -pauction_root < /mnt/e/code/ai_zijie/auction-system/docs/schema-v2.sql
mysql -uauction -pauction_root -e "USE auction; SHOW TABLES; SELECT id,nickname,token FROM users ORDER BY id;"
