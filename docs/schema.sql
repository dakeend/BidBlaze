-- =============================================================
-- 直播竞拍系统 · 数据库 schema v1
-- DB: MySQL 8.0+ / utf8mb4
-- 金额单位: 分 (INT)
-- =============================================================

CREATE DATABASE IF NOT EXISTS auction
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE auction;

-- -------------------------------------------------------------
-- users  用户表 (mock 登录)
-- -------------------------------------------------------------
DROP TABLE IF EXISTS users;
CREATE TABLE users (
  id          BIGINT       NOT NULL AUTO_INCREMENT,
  nickname    VARCHAR(32)  NOT NULL,
  avatar      VARCHAR(255) DEFAULT NULL,
  token       VARCHAR(64)  NOT NULL,
  created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_token    (token),
  UNIQUE KEY uk_nickname (nickname)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -------------------------------------------------------------
-- auctions  拍卖场次
-- status: pending(待开始) -> active(进行中) -> ended(已结束) | cancelled(已取消)
-- 状态由后端 1s ticker 扫表自动推进；商家可主动 cancel
-- -------------------------------------------------------------
DROP TABLE IF EXISTS auctions;
CREATE TABLE auctions (
  id                  BIGINT       NOT NULL AUTO_INCREMENT,
  title               VARCHAR(128) NOT NULL,
  cover_url           VARCHAR(255) DEFAULT NULL,
  start_price         INT          NOT NULL COMMENT '起拍价(分)',
  price_step          INT          NOT NULL COMMENT '最小加价幅度(分)',
  ceiling_price       INT          DEFAULT NULL COMMENT '封顶价(分), NULL=无封顶',
  current_price       INT          NOT NULL COMMENT '当前价(分), 冗余 = max(accepted bids.amount) 或 start_price',
  current_leader_id   BIGINT       DEFAULT NULL COMMENT '当前领先者 user_id',
  start_time          DATETIME     NOT NULL COMMENT '开拍时间, 商家设置, 到点自动 active',
  end_time            DATETIME     NOT NULL COMMENT '结束时间, 延时会改写',
  original_end_time   DATETIME     NOT NULL COMMENT '原始结束时间, 仅用于演示"已延时"',
  extend_seconds      INT          NOT NULL DEFAULT 30 COMMENT '触发延时时延长秒数',
  extend_threshold    INT          NOT NULL DEFAULT 30 COMMENT '剩余<此秒时出价触发延时',
  status              ENUM('pending','active','ended','cancelled') NOT NULL DEFAULT 'pending',
  seller_id           BIGINT       NOT NULL COMMENT '创建者 user_id',
  created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_status_start (status, start_time),
  KEY idx_status_end   (status, end_time),
  KEY idx_seller       (seller_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -------------------------------------------------------------
-- bids  出价流水
-- 成功失败都入库, 通过 status + reject_reason 区分
-- 定时任务定期清理 status='rejected' 且 created_at > 7 天的记录
-- -------------------------------------------------------------
DROP TABLE IF EXISTS bids;
CREATE TABLE bids (
  id            BIGINT       NOT NULL AUTO_INCREMENT,
  auction_id    BIGINT       NOT NULL,
  user_id       BIGINT       NOT NULL,
  amount        INT          NOT NULL COMMENT '出价绝对值(分)',
  status        ENUM('accepted','rejected') NOT NULL,
  reject_reason VARCHAR(32)  DEFAULT NULL COMMENT 'lock_failed|low_price|auction_ended|over_ceiling|auction_not_started',
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_auction_status_amount (auction_id, status, amount DESC),
  KEY idx_auction_created       (auction_id, created_at DESC),
  KEY idx_user                  (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -------------------------------------------------------------
-- orders  成交订单
-- 一场拍卖最多一个订单 (auction_id UNIQUE)
-- -------------------------------------------------------------
DROP TABLE IF EXISTS orders;
CREATE TABLE orders (
  id           BIGINT       NOT NULL AUTO_INCREMENT,
  auction_id   BIGINT       NOT NULL,
  winner_id    BIGINT       NOT NULL,
  seller_id    BIGINT       NOT NULL,
  final_price  INT          NOT NULL COMMENT '成交价(分)',
  status       ENUM('pending_pay','paid','cancelled') NOT NULL DEFAULT 'pending_pay',
  created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  paid_at      DATETIME     DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_auction (auction_id),
  KEY idx_winner_status (winner_id, status),
  KEY idx_seller_status (seller_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -------------------------------------------------------------
-- 演示种子数据 (可选)
-- -------------------------------------------------------------
INSERT INTO users (nickname, avatar, token) VALUES
  ('主播阿明', NULL, 'mock-token-seller-001'),
  ('买家张三', NULL, 'mock-token-user-001'),
  ('买家李四', NULL, 'mock-token-user-002');
