-- Backfill taxonomy for generated cases that have no coverage_cell_ids yet.
-- Title-based inference is intentionally conservative: unknown cases stay unclassified.
WITH inferred AS (
    SELECT id,
        CASE
            WHEN title ILIKE '%敏感词%' OR title ILIKE '%举报%' OR title ILIKE '%违规%' THEN 'security'
            WHEN title ILIKE '%客户端发布%' OR title ILIKE '%发布平台%' THEN 'clientRelease'
            WHEN title ILIKE '%小程序%' THEN 'miniappService'
            WHEN title ILIKE '%开放平台%' OR title ILIKE '%OAuth%' OR title ILIKE '%OpenID%' THEN 'openPlatform'
            WHEN title ILIKE '%群组%' OR title ILIKE '%成员%' OR title ILIKE '%禁言%' THEN 'group'
            WHEN title ILIKE '%好友%' THEN 'friend'
            WHEN title ILIKE '%消息%' THEN 'message'
            WHEN title ILIKE '%媒体%' OR title ILIKE '%图片%' OR title ILIKE '%头像%' THEN 'media'
            WHEN title ILIKE '%表情%' THEN 'userSticker'
            WHEN title ILIKE '%审计%' OR title ILIKE '%日志%' THEN 'audit'
            WHEN title ILIKE '%在线%' THEN 'monitor'
            WHEN title ILIKE '%服务器%' OR title ILIKE '%监控%' THEN 'server'
            WHEN title ILIKE '%统计%' OR title ILIKE '%趋势%' THEN 'stats'
            WHEN title ILIKE '%配置%' OR title ILIKE '%系统%' THEN 'config'
            WHEN title ILIKE '%注销%' THEN 'accountCancellation'
            WHEN title ILIKE '%设备令牌%' OR title ILIKE '%设备%' THEN 'deviceToken'
            WHEN title ILIKE '%登录%' OR title ILIKE '%认证%' OR title ILIKE '%token%' THEN 'auth'
            WHEN title ILIKE '%用户%' OR title ILIKE '%账号%' THEN 'user'
            ELSE ''
        END AS module
    FROM cas_case
    WHERE module = ''
)
UPDATE cas_case c
SET
    module = inferred.module,
    feature = CASE WHEN c.feature = '' THEN inferred.module ELSE c.feature END,
    case_type = CASE WHEN inferred.module = 'security' AND (c.case_type = '' OR c.case_type = 'api') THEN 'security' ELSE c.case_type END,
    priority = CASE WHEN inferred.module = 'security' AND (c.priority = '' OR c.priority = 'P2') THEN 'P1' ELSE c.priority END,
    business_domain = CASE
        WHEN inferred.module IN ('auth', 'user', 'security', 'accountCancellation', 'deviceToken') THEN '账号与安全'
        WHEN inferred.module IN ('message', 'group', 'friend') THEN '消息与社交'
        WHEN inferred.module IN ('media', 'userSticker') THEN '内容与媒体'
        WHEN inferred.module IN ('clientRelease', 'miniappService') THEN '客户端与小程序'
        WHEN inferred.module = 'openPlatform' THEN '开放平台'
        WHEN inferred.module IN ('config', 'stats', 'monitor', 'server', 'audit') THEN '平台运营'
        ELSE c.business_domain
    END
FROM inferred
WHERE c.id = inferred.id AND inferred.module <> '';
