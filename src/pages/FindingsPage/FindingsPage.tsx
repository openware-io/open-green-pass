import { useEffect, useMemo, useState } from 'react';
import { PageHeader } from '@/components/shared';
import { configuredGreenPassClient } from '@/api/runtime';
import type { Finding as ApiFinding } from '@/api/client';
import { toast } from 'sonner';
import { AlertTriangle, Bug, Camera, CheckCircle2, ChevronRight, Clipboard, FileWarning, GitBranch, Image as ImageIcon, Maximize2, Play, Search, ShieldAlert, ShieldCheck, Upload, UserRound, Video, X } from 'lucide-react';

type Severity = 'P0' | 'P1' | 'P2' | 'P3';
type FindingStatus = '待确认' | '已确认' | '修复中' | '待回归' | '已关闭' | '误报';

type Finding = {
  id: string;
  title: string;
  severity: Severity;
  status: FindingStatus;
  module: string;
  endpoint: string;
  owner: string;
  updated: string;
  run: string;
  caseId: string;
  confidence: '高' | '中' | '低';
  source: string;
  expected: string;
  actual: string;
  rootCause: string;
  suggestion: string;
  evidence: { request: string; response: string; log: string; trace: string };
  repo: { name: string; url: string; branch: string; commit: string };
  media: { screenshot: string; video?: { poster: string; duration: string; caption: string } };
  coverage: string[];
  timeline: Array<{ time: string; text: string; actor: string }>;
};

const REPO = {
  name: 'open-chat-admin',
  url: 'https://github.com/openware-io/open-chat-admin',
  branch: '2.2.1',
  commit: 'a44876435f69f78723e23c04d2ff1d59e1d05f96',
};

function screenshot(label: string, detail: string, tone: 'red' | 'amber' = 'red') {
  const color = tone === 'red' ? '#f87171' : '#fbbf24';
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="960" height="540">
    <rect width="960" height="540" fill="#0f172a"/>
    <rect x="30" y="24" width="900" height="54" rx="12" fill="#1e293b"/>
    <circle cx="56" cy="51" r="7" fill="#f87171"/><circle cx="78" cy="51" r="7" fill="#fbbf24"/><circle cx="100" cy="51" r="7" fill="#34d399"/>
    <rect x="126" y="40" width="500" height="22" rx="6" fill="#334155"/>
    <text x="56" y="126" fill="#94a3b8" font-family="Arial" font-size="18">GreenPass · 测试执行证据</text>
    <rect x="56" y="150" width="230" height="42" rx="10" fill="${color}" opacity="0.16"/>
    <text x="74" y="178" fill="${color}" font-family="Arial" font-size="22" font-weight="700">${label}</text>
    <text x="56" y="248" fill="#e2e8f0" font-family="Arial" font-size="28" font-weight="700">${detail}</text>
    <rect x="56" y="286" width="848" height="1" fill="#334155"/>
    <text x="56" y="330" fill="#94a3b8" font-family="Arial" font-size="20">请求：见证据链中的 HTTP 请求</text>
    <text x="56" y="366" fill="#94a3b8" font-family="Arial" font-size="20">实际：与契约期望不一致，详见响应与日志</text>
    <rect x="56" y="410" width="848" height="82" rx="10" fill="#1e293b"/>
    <text x="76" y="444" fill="#fca5a5" font-family="monospace" font-size="18">ERROR assertion.failed expected=2xx actual=5xx</text>
    <text x="76" y="472" fill="#cbd5e1" font-family="monospace" font-size="18">INFO  evidence captured at frozen commit ${REPO.commit.slice(0, 8)}</text>
    <text x="824" y="520" fill="#475569" font-family="Arial" font-size="14">EVIDENCE · READ ONLY</text>
  </svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

const DEFAULT_MEDIA = { screenshot: screenshot('截图证据', '断言失败：实际响应与契约不一致') };
const FINDINGS_API = configuredGreenPassClient();

const API_STATUS: Record<string, FindingStatus> = {
  candidate: '待确认', triaged: '待确认', confirmed: '已确认', fixing: '修复中',
  pending_regression: '待回归', closed: '已关闭', false_positive: '误报',
};
const API_CONFIDENCE: Record<string, Finding['confidence']> = { high: '高', medium: '中', low: '低' };

function fromApiFinding(row: ApiFinding, media: Finding['media'] = { screenshot: '' }): Finding {
  const source = row.source_file ? `${row.source_file}${row.source_line ? `:${row.source_line}` : ''}` : '—';
  const updated = row.updated_at ? new Date(row.updated_at).toLocaleString('zh-CN', { hour12: false }) : '—';
  return {
    id: String(row.id),
    title: row.title,
    severity: row.severity as Severity,
    status: API_STATUS[row.status] ?? '待确认',
    module: row.module || '—',
    endpoint: row.endpoint || '—',
    owner: row.owner || '未分配',
    updated,
    run: row.linked_run_id ? `#${row.linked_run_id}` : '—',
    caseId: row.linked_case_id ? `TC-${row.linked_case_id}` : '—',
    confidence: API_CONFIDENCE[row.confidence] ?? '中',
    source,
    expected: row.expected,
    actual: row.actual,
    rootCause: row.root_cause,
    suggestion: '请结合根因与证据链评审修复方案；平台只读源码，不提交修改。',
    evidence: {
      request: row.evidence?.request ?? '',
      response: row.evidence?.response ?? '',
      log: row.evidence?.log ?? '',
      trace: row.evidence?.trace ?? '',
    },
    repo: {
      name: row.repository.name,
      url: row.repository.url,
      branch: row.repository.branch,
      commit: row.repository.commit_sha,
    },
    media,
    coverage: row.coverage_cells ?? [],
    timeline: (row.timeline ?? []).map((event) => ({ time: event.time, text: event.text, actor: event.actor })),
  };
}

const SEED: Finding[] = [
  {
    id: 'F-1042',
    title: '支付回调验签失败仍返回 500，重复回调可能造成订单状态不一致',
    severity: 'P0',
    status: '待确认',
    module: 'payment',
    endpoint: 'POST /api/v1/payments/callback',
    owner: '未分配',
    updated: '10 分钟前',
    run: '#4821',
    caseId: 'TC-018',
    confidence: '高',
    source: 'src/api/payment.js:42',
    expected: '回调验签失败返回 401 + code=SIGNATURE_INVALID；订单状态保持不变',
    actual: '返回 500 INTERNAL_ERROR；订单状态被写成 paid',
    rootCause: '验签异常未映射为业务错误，异常路径仍执行了订单状态更新；签名串拼接顺序与网关不一致。',
    suggestion: '建议在验签失败分支提前返回 401，并将订单状态更新移出异常路径；补充重复回调/错误签名回归用例。建议修改点：src/api/payment.js:42 附近，仅建议，不写入仓库。',
    evidence: {
      request: 'POST /api/v1/payments/callback\nX-Client-Platform: pc-admin\n\n{"orderId":"ORD-20261022-001","signature":"bad"}',
      response: 'HTTP/1.1 500\n{"code":"INTERNAL_ERROR","message":"signature verify failed"}',
      log: '2026-10-22T15:42:11Z ERROR payment.callback order=ORD-20261022-001 err=invalid signature\n2026-10-22T15:42:11Z INFO order.update order=ORD-20261022-001 status=paid',
      trace: 'trace 8f2c… span payment.callback 312ms\n  └ signature.verify 4ms error\n  └ order.update 8ms status=paid',
    },
    repo: REPO,
    media: {
      ...DEFAULT_MEDIA,
      video: {
        poster: screenshot('视频证据', '实机操作 00:30 · 支付回调重放', 'amber'),
        duration: '00:30',
        caption: '支付回调重放：订单状态从 pending 变为 paid，未回滚',
      },
    },
    coverage: ['payment.callback · error', 'payment.callback · idempotency', 'order.state · data'],
    timeline: [
      { time: '15:42', text: '执行失败，归因为产品缺陷', actor: 'triage-bot' },
      { time: '15:43', text: '创建缺陷候选，等待确认', actor: 'triage-bot' },
    ],
  },
  {
    id: 'F-1041',
    title: '删除用户时 confirmUsername 不一致仍执行删除',
    severity: 'P1',
    status: '已确认',
    module: 'user',
    endpoint: 'DELETE /api/v1/admin/users/{id}',
    owner: '张立',
    updated: '32 分钟前',
    run: '#4820',
    caseId: 'TC-006',
    confidence: '高',
    source: 'src/api/user.js:17',
    expected: 'confirmUsername 不匹配返回 400 USERNAME_CONFIRM_MISMATCH，用户仍存在',
    actual: '返回 200，用户被删除',
    rootCause: '后端只校验了用户存在性，未比较 confirmUsername 与目标用户名；前端只做必填校验。',
    suggestion: '建议服务端在删除事务内比较 confirmUsername 与目标用户名，不一致返回 400 并回滚；补充“错误确认名”回归用例。',
    evidence: {
      request: 'DELETE /api/v1/admin/users/10086\n{"confirmUsername":"wrong-name"}',
      response: 'HTTP/1.1 200\n{"code":"OK"}',
      log: '2026-10-22T15:20:04Z INFO user.delete id=10086 operator=admin',
      trace: 'trace 11ab… span user.delete 44ms\n  └ user.lookup 2ms\n  └ user.delete 12ms',
    },
    repo: REPO,
    media: DEFAULT_MEDIA,
    coverage: ['user.delete · error', 'user.delete · authz'],
    timeline: [
      { time: '15:20', text: '执行失败，归因为产品缺陷', actor: 'triage-bot' },
      { time: '15:24', text: '人工确认缺陷，指派给张立', actor: '张立' },
    ],
  },
  {
    id: 'F-1039',
    title: '客户端发布提交使用过期 rowVersion 返回 500 而非 409',
    severity: 'P1',
    status: '修复中',
    module: 'clientRelease',
    endpoint: 'POST /api/v1/admin/client-releases/{id}/submit',
    owner: '李航',
    updated: '1 小时前',
    run: '#4818',
    caseId: 'TC-036',
    confidence: '中',
    source: 'src/api/clientRelease.js:52',
    expected: '过期 rowVersion 返回 409 CLIENT_RELEASE_INVALID_STATE 或 VERSION_CONFLICT',
    actual: '返回 500，无稳定错误码',
    rootCause: '乐观锁冲突未映射为 409；提交路径对 rowVersion 缺失/过期没有统一错误处理。',
    suggestion: '建议把 rowVersion 冲突映射为 409，并返回稳定 code；回归覆盖“先提交成功、再用旧版本提交”的并发场景。',
    evidence: {
      request: 'POST /api/v1/admin/client-releases/778/submit\n{"expectedRowVersion":3,"reason":"e2e stale"}',
      response: 'HTTP/1.1 500\n{"code":"INTERNAL_ERROR"}',
      log: '2026-10-22T14:51:22Z ERROR release.submit id=778 expected=3 current=5',
      trace: 'trace 9d31… span release.submit 81ms\n  └ release.version.check 3ms conflict',
    },
    repo: REPO,
    media: DEFAULT_MEDIA,
    coverage: ['clientRelease.submit · concurrency', 'clientRelease.submit · error'],
    timeline: [
      { time: '14:51', text: '执行失败，归因为产品缺陷', actor: 'triage-bot' },
      { time: '14:58', text: '确认缺陷并进入修复中', actor: '李航' },
    ],
  },
  {
    id: 'F-1037',
    title: '审计日志时间区间倒置返回 200 空列表',
    severity: 'P2',
    status: '待确认',
    module: 'audit',
    endpoint: 'GET /api/v1/admin/audit-logs',
    owner: '未分配',
    updated: '2 小时前',
    run: '#4815',
    caseId: 'TC-013',
    confidence: '中',
    source: 'src/api/audit.js:5',
    expected: 'from > to 返回 400 TIME_RANGE_INVALID',
    actual: '返回 200，items=[]',
    rootCause: '后端未校验时间区间顺序，空结果被当作合法查询。',
    suggestion: '建议增加 from <= to 校验，返回 400 TIME_RANGE_INVALID；补充倒置区间和缺失边界用例。',
    evidence: {
      request: 'GET /api/v1/admin/audit-logs?from=2026-02-01&to=2026-01-01',
      response: 'HTTP/1.1 200\n{"items":[],"total":0,"retentionFloor":null}',
      log: '2026-10-22T13:55:10Z INFO audit.list from=2026-02-01 to=2026-01-01 items=0',
      trace: 'trace 41cc… span audit.list 17ms\n  └ audit.query 9ms rows=0',
    },
    repo: REPO,
    media: DEFAULT_MEDIA,
    coverage: ['audit.list · boundary', 'audit.list · error'],
    timeline: [{ time: '13:55', text: '执行失败，归因为产品缺陷', actor: 'triage-bot' }],
  },
  {
    id: 'F-1036',
    title: '媒体上传预签名返回 403，分片上传无法开始',
    severity: 'P2',
    status: '待回归',
    module: 'media',
    endpoint: 'POST /api/v1/media/multipart-upload-sessions',
    owner: '王珂',
    updated: '昨天',
    run: '#4809',
    caseId: 'TC-027',
    confidence: '中',
    source: 'src/api/media.js:89',
    expected: '返回 200 + partUrls，预签名 URL 可直接 PUT',
    actual: '返回 403 AccessDenied，bucket CORS 未允许 PUT',
    rootCause: '测试 bucket 的 CORS/预签名策略未包含 PUT 与 ETag 暴露；属于环境配置而非产品代码。',
    suggestion: '建议补齐测试 bucket CORS、ETag 暴露和清理策略；该缺陷归因为环境问题，修复后重跑分片上传用例。',
    evidence: {
      request: 'POST /api/v1/media/multipart-upload-sessions\n{"objectId":"avatar-778","size":1048576}',
      response: 'HTTP/1.1 403\n{"code":"AccessDenied"}',
      log: '2026-10-22T11:20:00Z ERROR media.presign bucket=gp-test AccessDenied',
      trace: 'trace 7b2e… span media.presign 120ms\n  └ s3.presign 18ms access-denied',
    },
    repo: REPO,
    media: DEFAULT_MEDIA,
    coverage: ['media.upload · boundary', 'media.upload · error'],
    timeline: [
      { time: '11:20', text: '执行失败，归因为环境问题', actor: 'triage-bot' },
      { time: '11:35', text: '环境修复完成，等待回归', actor: '王珂' },
    ],
  },
  {
    id: 'F-1034',
    title: '消息列表分页越界返回 500（疑似误报）',
    severity: 'P3',
    status: '误报',
    module: 'message',
    endpoint: 'GET /api/v1/admin/messages',
    owner: '张立',
    updated: '昨天',
    run: '#4802',
    caseId: 'TC-010',
    confidence: '低',
    source: 'src/api/message.js:3',
    expected: 'pageSize 超过上限返回 400 或截断到上限',
    actual: '返回 500',
    rootCause: '测试数据 pageSize 传入了非数值字符串，axios 序列化为空参数；用例数据缺陷，不是产品缺陷。',
    suggestion: '建议修正用例：pageSize 使用整数边界值，并补充 pageSize=0/负数/超上限用例；无需修改产品源码。',
    evidence: {
      request: 'GET /api/v1/admin/messages?page=1&pageSize=abc',
      response: 'HTTP/1.1 500\n{"code":"INTERNAL_ERROR"}',
      log: '2026-10-22T10:12:00Z WARN http.query pageSize=abc parse failed',
      trace: 'trace 2d90… span http.query 1ms parse-error',
    },
    repo: REPO,
    media: DEFAULT_MEDIA,
    coverage: ['message.list · boundary'],
    timeline: [
      { time: '10:12', text: '执行失败，归因为用例缺陷', actor: 'triage-bot' },
      { time: '10:20', text: '人工标记误报', actor: '张立' },
    ],
  },
];

const SEV_STYLE: Record<Severity, string> = {
  P0: 'bg-red-50 text-red-700 border-red-200',
  P1: 'bg-amber-50 text-amber-700 border-amber-200',
  P2: 'bg-slate-100 text-slate-600 border-slate-200',
  P3: 'bg-slate-100 text-slate-600 border-slate-200',
};
const STATUS_STYLE: Record<FindingStatus, string> = {
  待确认: 'bg-amber-50 text-amber-700',
  已确认: 'bg-red-50 text-red-700',
  修复中: 'bg-emerald-50 text-emerald-700',
  待回归: 'bg-amber-50 text-amber-700',
  已关闭: 'bg-emerald-50 text-emerald-700',
  误报: 'bg-slate-100 text-slate-500',
};

function evidenceCount(finding: Finding) {
  return Object.values(finding.evidence).filter(Boolean).length;
}

function hasScreenshot(finding: Finding) {
  return Boolean(finding.media.screenshot);
}

function hasVideo(finding: Finding) {
  return Boolean(finding.media.video);
}

export default function FindingsPage() {
  const [items, setItems] = useState<Finding[]>(SEED);
  const [selectedID, setSelectedID] = useState<string>('');
  const [query, setQuery] = useState('');
  const [severity, setSeverity] = useState('全部');
  const [status, setStatus] = useState('全部');
  const [lightbox, setLightbox] = useState('');
  const [apiBacked, setApiBacked] = useState(false);
  const selected = items.find((item) => item.id === selectedID) ?? null;

  useEffect(() => {
    if (!FINDINGS_API) return;
    let cancelled = false;
    FINDINGS_API.listFindings({ limit: 100 })
      .then((rows) => {
        if (cancelled || rows.length === 0) return;
        setItems(rows.map((row) => fromApiFinding(row)));
        setApiBacked(true);
      })
      .catch(() => { /* keep prototype data when the API is unavailable */ });
    return () => { cancelled = true; };
  }, []);

  const filtered = useMemo(() => {
    const term = query.trim().toLowerCase();
    return items.filter((item) => {
      if (severity !== '全部' && item.severity !== severity) return false;
      if (status !== '全部' && item.status !== status) return false;
      if (term && !`${item.id} ${item.title} ${item.module} ${item.endpoint}`.toLowerCase().includes(term)) return false;
      return true;
    });
  }, [items, query, severity, status]);

  const openFinding = (item: Finding) => {
    setSelectedID(item.id);
    if (!FINDINGS_API || !/^\d+$/.test(item.id)) return;
    FINDINGS_API.getFinding(Number(item.id))
      .then((row) => setItems((current) => current.map((entry) => entry.id === item.id ? fromApiFinding(row, entry.media) : entry)))
      .catch(() => { /* keep the list representation when detail is unavailable */ });
  };

  const applyAction = async (item: Finding, action: 'confirm' | 'false_positive' | 'pending_regression') => {
    if (!FINDINGS_API || !/^\d+$/.test(item.id)) {
      const next: FindingStatus = action === 'confirm' ? '已确认' : action === 'false_positive' ? '误报' : '待回归';
      setItems((current) => current.map((entry) => entry.id === item.id ? { ...entry, status: next, updated: '刚刚', timeline: [...entry.timeline, { time: '刚刚', text: `状态更新为 ${next}`, actor: '张立' }] } : entry));
      toast.success(`已更新为「${next}」`);
      return;
    }
    try {
      const updated = await FINDINGS_API.transitionFinding(Number(item.id), { action });
      setItems((current) => current.map((entry) => entry.id === item.id ? fromApiFinding(updated, entry.media) : entry));
      toast.success('缺陷状态已更新');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '状态更新失败');
    }
  };

  const copySuggestion = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success('修复建议已复制', { description: '仅复制建议，不会修改源码。' });
    } catch {
      toast.error('复制失败，请手动选择文本');
    }
  };

  return (
    <div>
      <PageHeader title="缺陷中心" desc="发现 → 证据 → 归因 → 建议 → 回归 · 源码只读，不修改仓库">
        <span className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs text-emerald-700">
          <ShieldCheck className="h-3.5 w-3.5" />源码只读
        </span>
        <span className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-600">
          <Bug className="h-3.5 w-3.5" />缺陷建议不写回仓库
        </span>
        <span className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs ${apiBacked ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-slate-200 bg-slate-50 text-slate-500'}`}>
          {apiBacked ? 'API 数据' : '原型数据'}
        </span>
      </PageHeader>

      <div className="mb-5 grid grid-cols-2 gap-3 md:grid-cols-5">
        {[
          { label: '待确认', value: items.filter((item) => item.status === '待确认').length, tone: 'text-amber-700 bg-amber-50' },
          { label: '已确认', value: items.filter((item) => item.status === '已确认').length, tone: 'text-red-700 bg-red-50' },
          { label: 'P0 未关闭', value: items.filter((item) => item.severity === 'P0' && !['已关闭', '误报'].includes(item.status)).length, tone: 'text-red-700 bg-red-50' },
          { label: '待回归', value: items.filter((item) => item.status === '待回归').length, tone: 'text-amber-700 bg-amber-50' },
          { label: '误报', value: items.filter((item) => item.status === '误报').length, tone: 'text-slate-600 bg-slate-100' },
        ].map((stat) => (
          <div key={stat.label} className="rounded-xl border border-slate-200 bg-white px-4 py-3">
            <div className="text-[11px] text-slate-500">{stat.label}</div>
            <div className={`mt-1 inline-flex rounded-md px-2 py-0.5 text-xl font-semibold ${stat.tone}`}>{stat.value}</div>
          </div>
        ))}
      </div>

      <div className="mb-3 flex flex-wrap items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-3">
        <div className="relative min-w-[240px] flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索缺陷、模块、端点" className="w-full rounded-lg border border-slate-200 py-2 pl-8 pr-3 text-xs outline-none focus:border-emerald-400" />
        </div>
        <select aria-label="严重级别" value={severity} onChange={(event) => setSeverity(event.target.value)} className="rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-600">
          {['全部', 'P0', 'P1', 'P2', 'P3'].map((value) => <option key={value}>{value}</option>)}
        </select>
        <select aria-label="缺陷状态" value={status} onChange={(event) => setStatus(event.target.value)} className="rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-600">
          {['全部', '待确认', '已确认', '修复中', '待回归', '已关闭', '误报'].map((value) => <option key={value}>{value}</option>)}
        </select>
        <span className="ml-auto text-[11px] text-slate-400">{filtered.length} 条 · 截图证据 {items.filter(hasScreenshot).length} 条 · 视频证据 {items.filter(hasVideo).length} 条</span>
      </div>

      <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
        <table className="w-full text-xs">
          <thead className="bg-slate-50 text-left text-[10px] text-slate-500">
            <tr>
              <th className="px-4 py-3 font-medium">级别</th>
              <th className="px-4 py-3 font-medium">缺陷</th>
              <th className="px-4 py-3 font-medium">状态</th>
              <th className="px-4 py-3 font-medium">负责人</th>
              <th className="px-4 py-3 font-medium">证据</th>
              <th className="px-4 py-3 font-medium">关联运行</th>
              <th className="px-4 py-3 font-medium">更新</th>
              <th className="px-3 py-3" />
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {filtered.map((item) => (
              <tr key={item.id} onClick={() => openFinding(item)} className={`cursor-pointer transition hover:bg-slate-50 ${selectedID === item.id ? 'bg-emerald-50/40' : ''}`}>
                <td className="px-4 py-3"><span className={`rounded-md border px-2 py-0.5 text-[10px] font-semibold ${SEV_STYLE[item.severity]}`}>{item.severity}</span></td>
                <td className="max-w-[460px] px-4 py-3">
                  <div className="truncate font-medium text-slate-800">{item.title}</div>
                  <div className="mt-0.5 flex flex-wrap items-center gap-2 text-[10px] text-slate-400"><span className="font-mono">{item.id}</span><span>{item.module}</span><span className="font-mono">{item.endpoint}</span><span className="inline-flex items-center gap-1 text-slate-500"><GitBranch className="h-3 w-3" />{item.repo.name}@{item.repo.commit.slice(0, 8)}</span></div>
                </td>
                <td className="px-4 py-3"><span className={`rounded-full px-2 py-0.5 text-[10px] ${STATUS_STYLE[item.status]}`}>{item.status}</span></td>
                <td className="px-4 py-3 text-slate-600"><span className="inline-flex items-center gap-1"><UserRound className="h-3 w-3 text-slate-400" />{item.owner}</span></td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap items-center gap-1.5 text-[10px] text-slate-600">
                    <span className="inline-flex items-center gap-1"><FileWarning className="h-3.5 w-3.5 text-amber-500" />{evidenceCount(item)}/4</span>
                    {hasScreenshot(item) && <span className="inline-flex items-center gap-1 text-emerald-700"><ImageIcon className="h-3.5 w-3.5" />截图</span>}
                    {hasVideo(item) && <span className="inline-flex items-center gap-1 text-slate-600"><Video className="h-3.5 w-3.5" />视频</span>}
                  </div>
                </td>
                <td className="px-4 py-3 font-mono text-emerald-700">{item.run}</td>
                <td className="px-4 py-3 text-slate-400">{item.updated}</td>
                <td className="px-3 py-3 text-slate-300"><ChevronRight className="h-4 w-4" /></td>
              </tr>
            ))}
            {!filtered.length && <tr><td colSpan={8} className="p-10 text-center text-sm text-slate-400">没有匹配的缺陷</td></tr>}
          </tbody>
        </table>
      </div>

      {selected && <div className="fixed inset-0 z-50 flex justify-end bg-slate-900/45" onMouseDown={() => setSelectedID('')}>
        <aside className="h-full w-full max-w-3xl overflow-y-auto bg-white shadow-2xl" onMouseDown={(event) => event.stopPropagation()}>
          <div className="sticky top-0 z-10 border-b border-slate-200 bg-white px-6 py-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <span className={`rounded-md border px-2 py-0.5 text-[10px] font-semibold ${SEV_STYLE[selected.severity]}`}>{selected.severity}</span>
                  <span className={`rounded-full px-2 py-0.5 text-[10px] ${STATUS_STYLE[selected.status]}`}>{selected.status}</span>
                  <span className="font-mono text-[10px] text-slate-400">{selected.id}</span>
                </div>
                <h2 className="mt-2 text-base font-semibold text-slate-900">{selected.title}</h2>
                <div className="mt-1 text-[11px] text-slate-500">{selected.module} · <span className="font-mono">{selected.endpoint}</span> · 来源 <span className="font-mono">{selected.source}</span></div>
              </div>
              <button onClick={() => setSelectedID('')} aria-label="关闭" className="rounded-md p-1 text-slate-400 hover:bg-slate-100"><X className="h-4 w-4" /></button>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              <button onClick={() => void applyAction(selected, 'confirm')} className="rounded-lg bg-red-600 px-3 py-1.5 text-xs text-white">确认缺陷</button>
              <button onClick={() => void applyAction(selected, 'false_positive')} className="rounded-lg border border-slate-200 px-3 py-1.5 text-xs text-slate-600">标记误报</button>
              <button onClick={() => void applyAction(selected, 'pending_regression')} className="rounded-lg border border-amber-200 px-3 py-1.5 text-xs text-slate-600">标记已修复，待回归</button>
              <button onClick={() => toast.success('已模拟创建工单', { description: '可对接 Jira / 飞书；不会修改源码。' })} className="rounded-lg border border-emerald-200 px-3 py-1.5 text-xs text-emerald-700">创建工单</button>
              <span className="ml-auto inline-flex items-center gap-1 text-[10px] text-slate-400"><ShieldCheck className="h-3.5 w-3.5" />只读源码 · 不提交 PR</span>
            </div>
          </div>

          <div className="space-y-5 p-6">
            <div className="grid gap-3 md:grid-cols-4">
              <div className="rounded-lg border border-slate-200 p-3"><div className="text-[10px] text-slate-400">负责人</div><div className="mt-1 text-sm font-medium text-slate-700">{selected.owner}</div></div>
              <a href={selected.repo.url} target="_blank" rel="noreferrer" className="rounded-lg border border-slate-200 p-3 hover:border-emerald-300">
                <div className="text-[10px] text-slate-400">归属仓库 / 提交</div>
                <div className="mt-1 inline-flex items-center gap-1 text-sm font-medium text-slate-700"><GitBranch className="h-3.5 w-3.5" />{selected.repo.name}</div>
                <div className="mt-0.5 font-mono text-[10px] text-slate-400">{selected.repo.branch}@{selected.repo.commit.slice(0, 8)}</div>
              </a>
              <div className="rounded-lg border border-slate-200 p-3"><div className="text-[10px] text-slate-400">关联运行 / 用例</div><div className="mt-1 font-mono text-sm text-slate-700">{selected.run} / {selected.caseId}</div></div>
              <div className="rounded-lg border border-slate-200 p-3"><div className="text-[10px] text-slate-400">根因置信度</div><div className="mt-1 text-sm font-medium text-slate-700">{selected.confidence}</div></div>
            </div>

            <section>
              <div className="mb-2 flex items-center gap-1.5 text-xs font-semibold text-slate-700"><AlertTriangle className="h-3.5 w-3.5 text-amber-500" />期望 vs 实际</div>
              <div className="grid gap-3 md:grid-cols-2">
                <div className="rounded-lg border border-emerald-200 bg-emerald-50/50 p-3"><div className="text-[10px] font-medium text-emerald-700">契约期望</div><div className="mt-1 text-xs leading-5 text-slate-700">{selected.expected}</div></div>
                <div className="rounded-lg border border-red-200 bg-red-50/50 p-3"><div className="text-[10px] font-medium text-red-700">实际结果</div><div className="mt-1 text-xs leading-5 text-slate-700">{selected.actual}</div></div>
              </div>
            </section>

            <section>
              <div className="mb-2 flex flex-wrap items-center gap-2">
                <div className="flex items-center gap-1.5 text-xs font-semibold text-slate-700"><Camera className="h-3.5 w-3.5 text-emerald-500" />多媒体证据</div>
                <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] text-emerald-700">截图必需</span>
                <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] text-slate-600">视频建议</span>
                <button onClick={() => toast.success('原型：已预留截图/视频上传入口', { description: '接入后按 MIME、尺寸、时长、SHA-256 入库，证据只读。' })} className="ml-auto inline-flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-[10px] text-slate-600"><Upload className="h-3 w-3" />补充证据</button>
              </div>
              <div className="grid gap-2 md:grid-cols-2">
                {selected.media.screenshot ? <button type="button" onClick={() => setLightbox(selected.media.screenshot)} className="group overflow-hidden rounded-lg border border-slate-200 bg-slate-50 text-left">
                  <div className="relative">
                    <img src={selected.media.screenshot} alt="缺陷截图证据" className="aspect-video w-full object-cover" />
                    <span className="absolute left-2 top-2 rounded bg-slate-900/80 px-1.5 py-0.5 text-[9px] text-white">截图 · 关键断言失败</span>
                    <span className="absolute right-2 top-2 rounded bg-white/90 p-1 text-slate-600 opacity-0 transition group-hover:opacity-100"><Maximize2 className="h-3.5 w-3.5" /></span>
                  </div>
                  <div className="flex items-center justify-between px-3 py-2 text-[10px] text-slate-500"><span>点击查看大图 · 只读证据</span><ImageIcon className="h-3.5 w-3.5" /></div>
                </button> : <div className="grid min-h-[150px] place-items-center rounded-lg border border-dashed border-amber-200 bg-amber-50/40 p-4 text-center text-[11px] text-amber-700">未提供截图证据<br />P0/P1 需补充截图后再关闭</div>}
                {selected.media.video ? <button type="button" onClick={() => toast.success('原型视频证据', { description: `${selected.media.video?.caption} · ${selected.media.video?.duration}` })} className="group overflow-hidden rounded-lg border border-slate-200 bg-slate-50 text-left">
                  <div className="relative">
                    <img src={selected.media.video.poster} alt="缺陷视频证据" className="aspect-video w-full object-cover" />
                    <span className="absolute inset-0 grid place-items-center bg-slate-900/20"><span className="grid h-10 w-10 place-items-center rounded-full bg-white/90 text-slate-800 shadow"><Play className="ml-0.5 h-4 w-4" /></span></span>
                    <span className="absolute left-2 top-2 rounded bg-slate-900/80 px-1.5 py-0.5 text-[9px] text-white">视频 · {selected.media.video.duration}</span>
                  </div>
                  <div className="flex items-center justify-between px-3 py-2 text-[10px] text-slate-500"><span className="truncate">{selected.media.video.caption}</span><Video className="h-3.5 w-3.5" /></div>
                </button> : <div className="grid min-h-[150px] place-items-center rounded-lg border border-dashed border-slate-200 bg-slate-50 p-4 text-center text-[11px] text-slate-400">未提供视频证据<br />关键实机流程建议补充录制</div>}
              </div>
              <div className="mt-2 rounded-md border border-slate-100 bg-slate-50 px-3 py-2 text-[10px] leading-4 text-slate-500">证据要求：UI/实机用例必须提供截图；关键流程建议视频；服务端保存 MIME、尺寸、时长、SHA-256，证据只读不可篡改。</div>
            </section>

            <section>
              <div className="mb-2 flex items-center gap-1.5 text-xs font-semibold text-slate-700"><FileWarning className="h-3.5 w-3.5 text-emerald-500" />证据链</div>
              <div className="grid gap-2 md:grid-cols-2">
                {([
                  ['请求', selected.evidence.request],
                  ['响应', selected.evidence.response],
                  ['日志', selected.evidence.log],
                  ['Trace', selected.evidence.trace],
                ] as const).map(([label, value]) => <div key={label} className="rounded-lg border border-slate-200 bg-slate-50 p-3"><div className="text-[10px] font-medium text-slate-500">{label}</div><pre className="mt-1 max-h-32 overflow-auto whitespace-pre-wrap font-mono text-[10px] leading-4 text-slate-600">{value}</pre></div>)}
              </div>
            </section>

            <section>
              <div className="mb-2 flex items-center gap-1.5 text-xs font-semibold text-slate-700"><GitBranch className="h-3.5 w-3.5 text-emerald-500" />根因分析</div>
              <div className="rounded-lg border border-slate-200 p-3 text-xs leading-5 text-slate-700">{selected.rootCause}</div>
            </section>

            <section>
              <div className="mb-2 flex items-center justify-between gap-2">
                <div className="flex items-center gap-1.5 text-xs font-semibold text-slate-700"><CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />修复建议（只读）</div>
                <button onClick={() => void copySuggestion(selected.suggestion)} className="inline-flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-[10px] text-slate-600"><Clipboard className="h-3 w-3" />复制建议</button>
              </div>
              <div className="rounded-lg border border-emerald-200 bg-emerald-50/40 p-3 text-xs leading-5 text-slate-700">{selected.suggestion}</div>
              <div className="mt-1 text-[10px] text-slate-400">建议仅供研发参考，平台不修改源码、不创建 PR、不提交 commit。</div>
            </section>

            <section>
              <div className="mb-2 flex items-center gap-1.5 text-xs font-semibold text-slate-700"><ShieldAlert className="h-3.5 w-3.5 text-amber-500" />关联覆盖 Cell</div>
              <div className="flex flex-wrap gap-1.5">{selected.coverage.map((cell) => <span key={cell} className="rounded-full bg-slate-100 px-2 py-0.5 font-mono text-[10px] text-slate-600">{cell}</span>)}</div>
            </section>

            <section>
              <div className="mb-2 text-xs font-semibold text-slate-700">流转记录</div>
              <div className="space-y-2">{selected.timeline.map((event, index) => <div key={`${event.time}-${index}`} className="flex gap-2 text-[11px]"><span className="w-12 shrink-0 font-mono text-slate-400">{event.time}</span><span className="text-slate-600">{event.text}</span><span className="ml-auto text-slate-400">{event.actor}</span></div>)}</div>
            </section>
          </div>
        </aside>
      </div>}

      {lightbox && <div className="fixed inset-0 z-[60] grid place-items-center bg-slate-950/80 p-8" onMouseDown={() => setLightbox('')}>
        <div className="relative w-full max-w-5xl" onMouseDown={(event) => event.stopPropagation()}>
          <img src={lightbox} alt="缺陷截图证据大图" className="w-full rounded-xl border border-white/10 shadow-2xl" />
          <div className="mt-2 flex items-center justify-between text-[11px] text-white/70"><span>缺陷截图证据 · 只读 · 不修改源码</span><button onClick={() => setLightbox('')} className="inline-flex items-center gap-1 rounded-md bg-white/10 px-2 py-1 text-white"><X className="h-3.5 w-3.5" />关闭</button></div>
        </div>
      </div>}
    </div>
  );
}
