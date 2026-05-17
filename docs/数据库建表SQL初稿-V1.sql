-- 数据库建表 SQL 初稿 V1
-- 项目：期末救星
-- 说明：
-- 1. 基于 MySQL 8.x
-- 2. 不创建物理外键约束，便于开发测试
-- 3. 所有关联关系由应用层和事务保证一致性
-- 4. 对象存储字段第一版只保存 URL
-- 5. 所有字段均补充 COMMENT 注释

SET NAMES utf8mb4;

-- 1. 用户主表
CREATE TABLE IF NOT EXISTS `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `email` VARCHAR(128) NOT NULL COMMENT '用户邮箱，第一版仅支持QQ邮箱且全局唯一',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '强哈希加盐后的密码摘要',
  `role` VARCHAR(32) NOT NULL COMMENT '用户角色：ADMIN/USER',
  `status` VARCHAR(32) NOT NULL DEFAULT 'ENABLED' COMMENT '用户状态：ENABLED/DISABLED',
  `registered_at` DATETIME(3) NOT NULL COMMENT '注册时间',
  `last_login_at` DATETIME(3) DEFAULT NULL COMMENT '最后登录时间',
  `disabled_at` DATETIME(3) DEFAULT NULL COMMENT '账号被禁用时间',
  `disabled_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '禁用该账号的管理员用户ID',
  `remark` VARCHAR(255) DEFAULT NULL COMMENT '管理员备注',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_users_email` (`email`),
  KEY `idx_users_role` (`role`),
  KEY `idx_users_status` (`status`),
  KEY `idx_users_registered_at` (`registered_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户主表';

-- 2. 用户登录会话表
CREATE TABLE IF NOT EXISTS `user_sessions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '关联用户ID',
  `session_token` VARCHAR(128) NOT NULL COMMENT '会话令牌或会话唯一标识',
  `status` VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' COMMENT '会话状态：ACTIVE/INVALIDATED/EXPIRED',
  `login_ip` VARCHAR(64) DEFAULT NULL COMMENT '登录IP地址',
  `user_agent` VARCHAR(512) DEFAULT NULL COMMENT '浏览器或设备信息',
  `issued_at` DATETIME(3) NOT NULL COMMENT '会话签发时间',
  `expires_at` DATETIME(3) NOT NULL COMMENT '会话过期时间',
  `invalidated_at` DATETIME(3) DEFAULT NULL COMMENT '会话失效时间',
  `invalid_reason` VARCHAR(64) DEFAULT NULL COMMENT '会话失效原因：LOGOUT/CHANGE_PASSWORD/RESET_PASSWORD/ACCOUNT_DISABLED',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_sessions_token` (`session_token`),
  KEY `idx_user_sessions_user_id` (`user_id`),
  KEY `idx_user_sessions_status_expires` (`status`, `expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户登录会话表';

-- 3. 邀请码表
CREATE TABLE IF NOT EXISTS `invite_codes` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `code` VARCHAR(64) NOT NULL COMMENT '邀请码内容，全局唯一',
  `code_type` VARCHAR(32) NOT NULL COMMENT '邀请码类型：RANDOM/CUSTOM',
  `batch_no` VARCHAR(64) DEFAULT NULL COMMENT '批量生成批次号',
  `total_quota` INT UNSIGNED NOT NULL COMMENT '邀请码总可用次数',
  `remaining_quota` INT UNSIGNED NOT NULL COMMENT '邀请码剩余可用次数',
  `status` VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' COMMENT '邀请码状态：ACTIVE/DISABLED',
  `remark` VARCHAR(255) DEFAULT NULL COMMENT '邀请码备注',
  `created_by` BIGINT UNSIGNED NOT NULL COMMENT '创建该邀请码的管理员用户ID',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_invite_codes_code` (`code`),
  KEY `idx_invite_codes_batch_no` (`batch_no`),
  KEY `idx_invite_codes_status` (`status`),
  KEY `idx_invite_codes_created_by` (`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='邀请码表';

-- 4. 文件分类表
CREATE TABLE IF NOT EXISTS `file_categories` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `name` VARCHAR(64) NOT NULL COMMENT '分类名称，全局唯一',
  `is_builtin` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否为系统内置默认分类：0否1是',
  `status` VARCHAR(32) NOT NULL DEFAULT 'ENABLED' COMMENT '分类状态：ENABLED/DISABLED',
  `sort_no` INT NOT NULL DEFAULT 0 COMMENT '分类排序值，值越小越靠前',
  `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建分类的管理员用户ID',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_file_categories_name` (`name`),
  KEY `idx_file_categories_status_sort` (`status`, `sort_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='文件分类表';

-- 5. 学习资料主表
CREATE TABLE IF NOT EXISTS `learning_files` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `source_file_hash` VARCHAR(128) NOT NULL COMMENT '源文件内容哈希，用于重复文件识别',
  `source_file_name` VARCHAR(255) NOT NULL COMMENT '原始文件名',
  `source_file_type` VARCHAR(64) NOT NULL COMMENT '源文件类型或MIME类型',
  `source_file_size` BIGINT UNSIGNED NOT NULL COMMENT '源文件大小，单位字节',
  `source_object_url` VARCHAR(1024) NOT NULL COMMENT '源文件对象存储URL',
  `category_id` BIGINT UNSIGNED NOT NULL COMMENT '所属文件分类ID',
  `visibility` VARCHAR(32) NOT NULL COMMENT '可见范围：PUBLIC/PRIVATE_ADMIN',
  `upload_user_id` BIGINT UNSIGNED NOT NULL COMMENT '上传该文件的管理员用户ID',
  `upload_time` DATETIME(3) NOT NULL COMMENT '文件上传时间',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_learning_files_hash` (`source_file_hash`),
  KEY `idx_learning_files_category` (`category_id`),
  KEY `idx_learning_files_visibility` (`visibility`),
  KEY `idx_learning_files_upload_user` (`upload_user_id`),
  KEY `idx_learning_files_name` (`source_file_name`),
  KEY `idx_learning_files_upload_time` (`upload_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='学习资料主表，仅存文件主数据';

-- 6. 文件最新预览记录表
CREATE TABLE IF NOT EXISTS `file_preview_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `file_id` BIGINT UNSIGNED NOT NULL COMMENT '关联学习资料ID',
  `preview_mode` VARCHAR(32) NOT NULL COMMENT '预览模式：DIRECT/CONVERT_TO_PDF',
  `preview_status` VARCHAR(32) NOT NULL COMMENT '预览状态：NOT_REQUIRED/PENDING/PROCESSING/SUCCESS/FAIL',
  `preview_object_url` VARCHAR(1024) DEFAULT NULL COMMENT '预览文件对象存储URL，通常为转换后的PDF',
  `last_success_at` DATETIME(3) DEFAULT NULL COMMENT '最近一次预览资源成功生成时间',
  `last_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '最近一次预览失败原因摘要',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_file_preview_records_file_id` (`file_id`),
  KEY `idx_file_preview_records_status` (`preview_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='文件最新预览记录表';

-- 7. 文件最新生成记录表
CREATE TABLE IF NOT EXISTS `file_generate_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `file_id` BIGINT UNSIGNED NOT NULL COMMENT '关联学习资料ID',
  `total_status` VARCHAR(32) NOT NULL COMMENT '总生成状态：PENDING/PROCESSING/PARTIAL_SUCCESS/SUCCESS/FAIL',
  `last_generated_at` DATETIME(3) DEFAULT NULL COMMENT '最近一次生成完成时间',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_file_generate_records_file_id` (`file_id`),
  KEY `idx_file_generate_records_total_status` (`total_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='文件最新生成记录表';

-- 8. 文件最新生成结果项表
CREATE TABLE IF NOT EXISTS `file_generate_record_items` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `generate_record_id` BIGINT UNSIGNED NOT NULL COMMENT '关联文件最新生成记录ID',
  `item_type` VARCHAR(32) NOT NULL COMMENT '结果类型：QUESTION/KNOWLEDGE/EXTENDED',
  `item_status` VARCHAR(32) NOT NULL COMMENT '结果状态：PENDING/PROCESSING/SUCCESS/FAIL',
  `result_object_url` VARCHAR(1024) DEFAULT NULL COMMENT '结果HTML对象存储URL',
  `last_success_at` DATETIME(3) DEFAULT NULL COMMENT '最近一次该类型结果成功生成时间',
  `last_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '最近一次该类型结果失败原因摘要',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_file_generate_record_items_record_type` (`generate_record_id`, `item_type`),
  KEY `idx_file_generate_record_items_status` (`item_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='文件最新生成结果项表';

-- 9. 生成总任务历史表
CREATE TABLE IF NOT EXISTS `generate_tasks` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `task_no` VARCHAR(64) NOT NULL COMMENT '生成总任务编号，全局唯一',
  `file_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联学习资料ID，文件删除后允许为空',
  `upload_user_id` BIGINT UNSIGNED NOT NULL COMMENT '触发该任务的上传管理员用户ID',
  `trigger_type` VARCHAR(32) NOT NULL COMMENT '触发类型：UPLOAD/MANUAL_RETRY',
  `status` VARCHAR(32) NOT NULL COMMENT '总任务状态：PENDING/PROCESSING/PARTIAL_SUCCESS/SUCCESS/FAIL',
  `file_snapshot_name` VARCHAR(255) NOT NULL COMMENT '文件名快照，便于文件删除后保留历史',
  `file_snapshot_hash` VARCHAR(128) NOT NULL COMMENT '文件哈希快照，便于文件删除后保留历史',
  `file_deleted_snapshot` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '文件是否已删除快照：0否1是',
  `started_at` DATETIME(3) DEFAULT NULL COMMENT '任务开始时间',
  `finished_at` DATETIME(3) DEFAULT NULL COMMENT '任务结束时间',
  `last_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '最近一次总任务失败摘要',
  `expires_at` DATETIME(3) NOT NULL COMMENT '任务历史保留截止时间，默认创建后30天',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_generate_tasks_task_no` (`task_no`),
  KEY `idx_generate_tasks_file_id` (`file_id`),
  KEY `idx_generate_tasks_upload_user` (`upload_user_id`, `created_at`),
  KEY `idx_generate_tasks_status` (`status`, `created_at`),
  KEY `idx_generate_tasks_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='生成总任务历史表';

-- 10. 生成子任务历史表
CREATE TABLE IF NOT EXISTS `generate_task_items` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `task_id` BIGINT UNSIGNED NOT NULL COMMENT '关联生成总任务ID',
  `item_type` VARCHAR(32) NOT NULL COMMENT '子任务类型：QUESTION/KNOWLEDGE/EXTENDED',
  `status` VARCHAR(32) NOT NULL COMMENT '子任务状态：PENDING/PROCESSING/SUCCESS/FAIL',
  `auto_retry_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已执行自动重试次数',
  `manual_retry_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已执行手动重试次数',
  `max_auto_retry_count` INT UNSIGNED NOT NULL DEFAULT 3 COMMENT '最大自动重试次数，第一版固定3次',
  `retry_interval_seconds` INT UNSIGNED NOT NULL DEFAULT 5 COMMENT '自动重试间隔秒数，第一版固定5秒',
  `started_at` DATETIME(3) DEFAULT NULL COMMENT '子任务开始时间',
  `finished_at` DATETIME(3) DEFAULT NULL COMMENT '子任务结束时间',
  `next_retry_at` DATETIME(3) DEFAULT NULL COMMENT '下一次自动重试时间',
  `last_error_code` VARCHAR(64) DEFAULT NULL COMMENT '最近一次失败错误码',
  `last_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '最近一次失败错误摘要',
  `result_object_url` VARCHAR(1024) DEFAULT NULL COMMENT '本次子任务生成的结果对象存储URL',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_generate_task_items_task_type` (`task_id`, `item_type`),
  KEY `idx_generate_task_items_status` (`status`, `next_retry_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='生成子任务历史表';

-- 11. 预览转换任务历史表
CREATE TABLE IF NOT EXISTS `preview_conversion_tasks` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `file_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联学习资料ID，文件删除后允许为空',
  `request_user_id` BIGINT UNSIGNED NOT NULL COMMENT '触发转换的管理员用户ID',
  `source_file_type` VARCHAR(64) NOT NULL COMMENT '源文件类型，通常为Word或PPT',
  `status` VARCHAR(32) NOT NULL COMMENT '转换任务状态：PENDING/PROCESSING/SUCCESS/FAIL',
  `auto_retry_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已执行自动重试次数',
  `manual_retry_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已执行手动重试次数',
  `max_auto_retry_count` INT UNSIGNED NOT NULL DEFAULT 3 COMMENT '最大自动重试次数，第一版固定3次',
  `retry_interval_seconds` INT UNSIGNED NOT NULL DEFAULT 5 COMMENT '自动重试间隔秒数，第一版固定5秒',
  `started_at` DATETIME(3) DEFAULT NULL COMMENT '转换任务开始时间',
  `finished_at` DATETIME(3) DEFAULT NULL COMMENT '转换任务结束时间',
  `next_retry_at` DATETIME(3) DEFAULT NULL COMMENT '下一次自动重试时间',
  `last_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '最近一次转换失败原因摘要',
  `preview_object_url` VARCHAR(1024) DEFAULT NULL COMMENT '转换后预览文件对象存储URL',
  `expires_at` DATETIME(3) NOT NULL COMMENT '转换任务历史保留截止时间，默认创建后30天',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_preview_conversion_file` (`file_id`, `created_at`),
  KEY `idx_preview_conversion_status` (`status`, `next_retry_at`),
  KEY `idx_preview_conversion_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='预览转换任务历史表';

-- 12. 任务重试日志表
CREATE TABLE IF NOT EXISTS `task_retry_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `biz_type` VARCHAR(32) NOT NULL COMMENT '业务类型：GENERATE_ITEM/PREVIEW_CONVERT',
  `biz_id` BIGINT UNSIGNED NOT NULL COMMENT '对应的子任务ID或预览转换任务ID',
  `task_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联生成总任务ID，可为空',
  `retry_mode` VARCHAR(32) NOT NULL COMMENT '重试方式：AUTO/MANUAL',
  `retry_no` INT UNSIGNED NOT NULL COMMENT '当前是第几次重试',
  `status_before` VARCHAR(32) NOT NULL COMMENT '重试前状态',
  `status_after` VARCHAR(32) NOT NULL COMMENT '重试后状态',
  `trigger_user_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '手动重试操作人用户ID，自动重试为空',
  `error_message` VARCHAR(1024) DEFAULT NULL COMMENT '本次重试对应的错误摘要',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_retry_logs_biz` (`biz_type`, `biz_id`, `created_at`),
  KEY `idx_retry_logs_task` (`task_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务重试日志表';

-- 13. 站内通知表
CREATE TABLE IF NOT EXISTS `system_notifications` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '接收通知的用户ID',
  `type` VARCHAR(32) NOT NULL COMMENT '通知类型：GEN_SUCCESS/GEN_PARTIAL_SUCCESS/GEN_FAIL/PREVIEW_SUCCESS/PREVIEW_FAIL',
  `title` VARCHAR(255) NOT NULL COMMENT '通知标题',
  `summary` VARCHAR(512) NOT NULL COMMENT '通知摘要，用于通知列表展示',
  `content` TEXT DEFAULT NULL COMMENT '通知详细内容',
  `status` VARCHAR(32) NOT NULL DEFAULT 'UNREAD' COMMENT '通知状态：UNREAD/READ',
  `target_type` VARCHAR(32) NOT NULL COMMENT '跳转目标类型：GENERATE_TASK/PREVIEW_TASK',
  `target_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '跳转目标ID',
  `target_snapshot_name` VARCHAR(255) DEFAULT NULL COMMENT '目标名称快照，便于资源删除后仍展示历史',
  `error_summary` VARCHAR(1024) DEFAULT NULL COMMENT '失败类通知的错误摘要',
  `merged_key` VARCHAR(128) DEFAULT NULL COMMENT '连续失败通知合并键',
  `read_at` DATETIME(3) DEFAULT NULL COMMENT '通知已读时间',
  `expires_at` DATETIME(3) NOT NULL COMMENT '通知保留截止时间，默认创建后30天',
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_notifications_user_status` (`user_id`, `status`, `created_at`),
  KEY `idx_notifications_user_type` (`user_id`, `type`, `created_at`),
  KEY `idx_notifications_expires` (`expires_at`),
  KEY `idx_notifications_merged_key` (`merged_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='站内通知表';
