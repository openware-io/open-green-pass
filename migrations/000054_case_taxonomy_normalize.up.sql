-- Normalize partially classified rows after earlier backfills.
UPDATE cas_case
SET feature = module
WHERE feature = '' AND module <> '';

UPDATE cas_case
SET business_domain = CASE
    WHEN module IN ('auth', 'user', 'security', 'accountCancellation', 'deviceToken') THEN '账号与安全'
    WHEN module IN ('message', 'group', 'friend') THEN '消息与社交'
    WHEN module IN ('media', 'userSticker') THEN '内容与媒体'
    WHEN module IN ('clientRelease', 'miniappService') THEN '客户端与小程序'
    WHEN module = 'openPlatform' THEN '开放平台'
    WHEN module IN ('config', 'stats', 'monitor', 'server', 'audit') THEN '平台运营'
    ELSE business_domain
END
WHERE business_domain = '' AND module <> '';
