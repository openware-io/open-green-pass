-- Best-effort backfill for cases generated before taxonomy existed.
-- Only fills empty module/case_type/priority/business_domain from coverage cells.
-- Feature and operation stay empty (UI shows "未分类") instead of inventing labels.
WITH classified AS (
    SELECT
        c.id,
        MIN(cell.module) AS module,
        CASE WHEN BOOL_OR(cell.dimension IN ('authz', 'security')) THEN 'security' ELSE 'functional' END AS case_type,
        CASE WHEN BOOL_OR(cell.dimension IN ('authz', 'security')) THEN 'P1' ELSE 'P2' END AS priority
    FROM cas_case c
    CROSS JOIN LATERAL jsonb_array_elements_text(c.coverage_cell_ids) AS ref(cell_id)
    JOIN cov_cell cell ON cell.id = ref.cell_id::bigint
    WHERE c.module = ''
    GROUP BY c.id
)
UPDATE cas_case c
SET
    module = classified.module,
    case_type = CASE WHEN c.case_type = '' OR c.case_type = 'api' THEN classified.case_type ELSE c.case_type END,
    priority = CASE WHEN c.priority = '' OR c.priority = 'P2' THEN classified.priority ELSE c.priority END,
    business_domain = CASE
        WHEN c.business_domain <> '' THEN c.business_domain
        WHEN classified.module IN ('auth', 'user', 'security', 'accountCancellation', 'deviceToken') THEN '账号与安全'
        WHEN classified.module IN ('message', 'group', 'friend') THEN '消息与社交'
        WHEN classified.module IN ('media', 'userSticker') THEN '内容与媒体'
        WHEN classified.module IN ('clientRelease', 'miniappService') THEN '客户端与小程序'
        WHEN classified.module = 'openPlatform' THEN '开放平台'
        WHEN classified.module IN ('config', 'stats', 'monitor', 'server', 'audit') THEN '平台运营'
        ELSE '未分类'
    END
FROM classified
WHERE c.id = classified.id;
