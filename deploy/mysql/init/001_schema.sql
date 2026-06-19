-- 精小弘 Go bot MySQL schema
-- 说明：
--   1. 运行时不使用 GORM AutoMigrate；表结构以本文件为准。
--   2. docker-compose.yaml 会把本文件挂载到 MySQL 的 /docker-entrypoint-initdb.d/001_schema.sql。
--      MySQL 官方 entrypoint 会在 mysql_data volume 首次初始化时自动执行。
--   3. gorm/gen 也从执行本文件后的 MySQL 表结构生成模型和 query。
--   4. 本文件只建表，不创建 database；docker-compose.yaml 会通过 MYSQL_DATABASE=jxh_bot 自动创建 database。

CREATE TABLE IF NOT EXISTS `knowledge_entries` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source_key` varchar(255) NOT NULL COMMENT '知识条目稳定键；WPS source_id 为空时由 keyword 规范化生成',
  `keyword` varchar(255) NOT NULL COMMENT '关键词；旧 WPS 回复表第一列',
  `entry_type` varchar(32) NOT NULL COMMENT '条目类型：knowledge/menu_node/chitchat',
  `path` varchar(512) DEFAULT NULL COMMENT '由 %编号 菜单树生成的菜单路径',
  `aliases_json` json DEFAULT NULL COMMENT '同义问法 JSON 数组',
  `category` varchar(64) DEFAULT NULL COMMENT '分类',
  `tags_json` json DEFAULT NULL COMMENT '标签 JSON 数组',
  `answer` text NOT NULL COMMENT '标准回答；旧 WPS 回复表第二列',
  `content` mediumtext NOT NULL COMMENT '/ai 检索使用的知识正文',
  `enabled` boolean NOT NULL COMMENT '是否启用',
  `exact_reply` boolean NOT NULL COMMENT '是否参与普通关键词精确回复',
  `ai_enabled` boolean NOT NULL COMMENT '是否参与 /ai 检索',
  `last_import_run_id` bigint unsigned DEFAULT NULL COMMENT '最近一次导入批次 ID',
  `source_updated_at` datetime(3) DEFAULT NULL COMMENT '源表人工维护时间',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_knowledge_entries_source_key` (`source_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `knowledge_import_runs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source` varchar(32) NOT NULL COMMENT '导入来源，如 wps',
  `status` varchar(16) NOT NULL COMMENT '导入状态',
  `total_rows` bigint DEFAULT NULL COMMENT '源表总行数',
  `imported_rows` bigint DEFAULT NULL COMMENT '导入条数',
  `skipped_rows` bigint DEFAULT NULL COMMENT '跳过条数',
  `error_message` text DEFAULT NULL COMMENT '失败原因',
  `started_at` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `finished_at` datetime(3) DEFAULT NULL COMMENT '结束时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `admins` (
  `user_id` bigint NOT NULL COMMENT 'QQ 用户 ID',
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `blacklists` (
  `user_id` bigint NOT NULL COMMENT 'QQ 用户 ID',
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `scheduled_jobs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `type` varchar(16) NOT NULL COMMENT '任务类型：每天/单次',
  `time_hhmm` varchar(5) NOT NULL COMMENT '触发时间，格式 HH:MM',
  `group_id` bigint NOT NULL COMMENT 'QQ群号',
  `message` text NOT NULL COMMENT '定时发送内容',
  `enabled` boolean NOT NULL COMMENT '是否启用',
  `last_run_at` datetime(3) DEFAULT NULL COMMENT '最近执行时间；用于防止同一天重复触发',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `processed_events` (
  `event_key` varchar(128) NOT NULL COMMENT '事件去重键',
  `processed_at` datetime(3) DEFAULT NULL COMMENT '处理时间；清理任务按该字段删除过期记录',
  PRIMARY KEY (`event_key`),
  KEY `idx_processed_events_processed_at` (`processed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
