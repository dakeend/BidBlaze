-- =============================================================
-- 直播竞拍系统 · 数据库 schema v2
-- DB: MySQL 8.0+ / utf8mb4
-- 金额单位: 分 (BIGINT)
-- Source contract: docs/contract-v2.md
-- =============================================================

CREATE DATABASE IF NOT EXISTS auction
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE auction;

SET FOREIGN_KEY_CHECKS = 0;
DROP TABLE IF EXISTS event_outbox;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS bids;
DROP TABLE IF EXISTS auctions;
DROP TABLE IF EXISTS users;
SET FOREIGN_KEY_CHECKS = 1;

-- -------------------------------------------------------------
-- users
-- -------------------------------------------------------------
CREATE TABLE users (
  id          BIGINT       NOT NULL AUTO_INCREMENT,
  nickname    VARCHAR(32)  NOT NULL,
  avatar      VARCHAR(255) DEFAULT NULL,
  token       VARCHAR(128) NOT NULL,
  created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_token (token),
  UNIQUE KEY uk_users_nickname (nickname)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -------------------------------------------------------------
-- auctions
-- -------------------------------------------------------------
CREATE TABLE auctions (
  id                  BIGINT       NOT NULL AUTO_INCREMENT,
  title               VARCHAR(128) NOT NULL,
  description         TEXT         DEFAULT NULL,
  cover_url           VARCHAR(512) DEFAULT NULL,
  images              JSON         DEFAULT NULL COMMENT '商品图片 URL 数组',
  stream_url          VARCHAR(512) DEFAULT NULL COMMENT '直播流地址, 为空时前端使用占位视频',
  start_price         BIGINT       NOT NULL COMMENT '起拍价(分)',
  price_step          BIGINT       NOT NULL COMMENT '最小加价幅度(分)',
  ceiling_price       BIGINT       DEFAULT NULL COMMENT '封顶价(分), NULL=无封顶',
  current_price       BIGINT       NOT NULL COMMENT '当前价(分)',
  current_leader_id   BIGINT       DEFAULT NULL COMMENT '当前领先者 user_id',
  start_time          DATETIME(3)  NOT NULL,
  end_time            DATETIME(3)  NOT NULL,
  original_end_time   DATETIME(3)  NOT NULL,
  extend_seconds      INT          NOT NULL DEFAULT 30,
  extend_threshold    INT          NOT NULL DEFAULT 30,
  status              ENUM('pending','active','ended','cancelled') NOT NULL DEFAULT 'pending',
  version             BIGINT       NOT NULL DEFAULT 0 COMMENT '同一拍卖内事件和状态版本',
  viewer_count        INT          NOT NULL DEFAULT 0 COMMENT '房间在线连接数估算',
  bid_count           BIGINT       NOT NULL DEFAULT 0 COMMENT 'accepted 出价累计数',
  seller_id           BIGINT       NOT NULL,
  created_at          DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at          DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_auctions_status_start_time (status, start_time),
  KEY idx_auctions_status_end_time (status, end_time),
  KEY idx_auctions_seller_status (seller_id, status),
  KEY idx_auctions_current_leader (current_leader_id),
  CONSTRAINT fk_auctions_seller FOREIGN KEY (seller_id) REFERENCES users(id),
  CONSTRAINT fk_auctions_current_leader FOREIGN KEY (current_leader_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -------------------------------------------------------------
-- bids
-- -------------------------------------------------------------
CREATE TABLE bids (
  id               BIGINT       NOT NULL AUTO_INCREMENT,
  auction_id       BIGINT       NOT NULL,
  user_id          BIGINT       NOT NULL,
  amount           BIGINT       NOT NULL COMMENT '出价绝对值(分)',
  status           ENUM('accepted','rejected') NOT NULL,
  reject_reason    VARCHAR(32)  DEFAULT NULL COMMENT 'lock_failed|low_price|auction_ended|over_ceiling|auction_not_started|conflict',
  idempotency_key  VARCHAR(128) DEFAULT NULL,
  created_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_bids_auction_status_amount (auction_id, status, amount DESC),
  KEY idx_bids_auction_created (auction_id, created_at DESC),
  KEY idx_bids_user_created (user_id, created_at DESC),
  UNIQUE KEY uk_bids_idempotency (auction_id, user_id, idempotency_key),
  CONSTRAINT fk_bids_auction FOREIGN KEY (auction_id) REFERENCES auctions(id),
  CONSTRAINT fk_bids_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -------------------------------------------------------------
-- orders
-- -------------------------------------------------------------
CREATE TABLE orders (
  id           BIGINT       NOT NULL AUTO_INCREMENT,
  auction_id   BIGINT       NOT NULL,
  winner_id    BIGINT       NOT NULL,
  seller_id    BIGINT       NOT NULL,
  final_price  BIGINT       NOT NULL COMMENT '成交价(分)',
  status       ENUM('pending_pay','paid','closed') NOT NULL DEFAULT 'pending_pay',
  created_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  paid_at      DATETIME(3)  DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_orders_auction (auction_id),
  KEY idx_orders_winner_status (winner_id, status),
  KEY idx_orders_seller_status (seller_id, status),
  CONSTRAINT fk_orders_auction FOREIGN KEY (auction_id) REFERENCES auctions(id),
  CONSTRAINT fk_orders_winner FOREIGN KEY (winner_id) REFERENCES users(id),
  CONSTRAINT fk_orders_seller FOREIGN KEY (seller_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -------------------------------------------------------------
-- idempotency_keys
-- -------------------------------------------------------------
CREATE TABLE idempotency_keys (
  id                BIGINT       NOT NULL AUTO_INCREMENT,
  user_id           BIGINT       NOT NULL,
  scope             VARCHAR(32)  NOT NULL COMMENT 'bid|pay',
  idempotency_key   VARCHAR(128) NOT NULL,
  request_hash      CHAR(64)     NOT NULL,
  response_json     JSON         DEFAULT NULL,
  status            ENUM('processing','succeeded','failed') NOT NULL,
  created_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_idempotency_user_scope_key (user_id, scope, idempotency_key),
  KEY idx_idempotency_created (created_at),
  CONSTRAINT fk_idempotency_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -------------------------------------------------------------
-- event_outbox
-- -------------------------------------------------------------
CREATE TABLE event_outbox (
  id              BIGINT       NOT NULL AUTO_INCREMENT,
  aggregate_type  VARCHAR(32)  NOT NULL COMMENT 'auction',
  aggregate_id    BIGINT       NOT NULL COMMENT 'auction_id',
  event_type      VARCHAR(64)  NOT NULL COMMENT 'BidAccepted|AuctionExtended|AuctionStarted|AuctionEnded|AuctionCancelled',
  event_seq       BIGINT       NOT NULL,
  payload         JSON         NOT NULL,
  status          ENUM('pending','published','failed') NOT NULL DEFAULT 'pending',
  retry_count     INT          NOT NULL DEFAULT 0,
  last_error      VARCHAR(512) DEFAULT NULL,
  created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  published_at    DATETIME(3)  DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_outbox_auction_seq (aggregate_type, aggregate_id, event_seq),
  KEY idx_outbox_status_id (status, id),
  KEY idx_outbox_aggregate (aggregate_type, aggregate_id, event_seq)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -------------------------------------------------------------
-- seed data
-- -------------------------------------------------------------
INSERT INTO users (nickname, avatar, token) VALUES
  ('主播阿明', NULL, 'mock-token-seller-001'),
  ('买家张三', NULL, 'mock-token-user-001'),
  ('买家李四', NULL, 'mock-token-user-002');
