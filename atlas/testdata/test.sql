CREATE TABLE `op_logs` (
    `id` bigint NOT NULL AUTO_INCREMENT,
    `user_id` bigint NOT NULL,
    `module` varchar(256) NOT NULL,
    `action` varchar(1024) NOT NULL,
    `log_id` varchar(125) NOT NULL,
    `content` text NOT NULL,
    `status` bigint NOT NULL DEFAULT '0',
    `created_at` bigint NOT NULL DEFAULT '0',
    PRIMARY KEY (`id`)
) ENGINE = InnoDB COMMENT = '操作历史';

