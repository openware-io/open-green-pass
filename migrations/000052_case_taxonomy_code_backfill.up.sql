-- Backfill taxonomy for generated M3 cases whose module is encoded in the code prefix.
WITH inferred AS (
    SELECT id,
        CASE
            WHEN code ILIKE 'm3-accountcancellation-%' THEN 'accountCancellation'
            WHEN code ILIKE 'm3-clientrelease-%' THEN 'clientRelease'
            WHEN code ILIKE 'm3-miniappservice-%' THEN 'miniappService'
            WHEN code ILIKE 'm3-openplatform-%' THEN 'openPlatform'
            WHEN code ILIKE 'm3-usersticker-%' THEN 'userSticker'
            WHEN code ILIKE 'm3-devicetoken-%' THEN 'deviceToken'
            WHEN code ILIKE 'm3-security-%' THEN 'security'
            WHEN code ILIKE 'm3-config-%' THEN 'config'
            WHEN code ILIKE 'm3-media-%' THEN 'media'
            WHEN code ILIKE 'm3-message-%' THEN 'message'
            WHEN code ILIKE 'm3-monitor-%' THEN 'monitor'
            WHEN code ILIKE 'm3-server-%' THEN 'server'
            WHEN code ILIKE 'm3-stats-%' THEN 'stats'
            WHEN code ILIKE 'm3-auth-%' THEN 'auth'
            WHEN code ILIKE 'm3-user-%' THEN 'user'
            WHEN code ILIKE 'm3-friend-%' THEN 'friend'
            WHEN code ILIKE 'm3-group-%' THEN 'group'
            WHEN code ILIKE 'm3-audit-%' THEN 'audit'
            ELSE ''
        END AS module
    FROM cas_case
    WHERE module = ''
)
UPDATE cas_case c
SET
    module = inferred.module,
    feature = CASE WHEN c.feature = '' THEN inferred.module ELSE c.feature END,
    case_type = CASE WHEN inferred.module IN ('security', 'auth') AND c.case_type IN ('', 'api') THEN 'security' ELSE c.case_type END,
    priority = CASE WHEN inferred.module = 'security' AND c.priority IN ('', 'P2') THEN 'P1' ELSE c.priority END,
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
