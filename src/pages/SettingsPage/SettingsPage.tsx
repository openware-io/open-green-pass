import { useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { useCurrentUser } from '@/context/login';
import { PageHeader, PrimaryButton } from '@/components/shared';
import { Building2, ShieldCheck, Wallet, Lock, Bell, Server, Save, ShieldAlert, Upload } from 'lucide-react';

function Toggle({ on, onChange }: { on: boolean; onChange: (v: boolean) => void }) {
  return (
    <button type="button" onClick={() => onChange(!on)}
      className={'w-9 h-5 rounded-full relative transition flex-shrink-0 ' + (on ? 'bg-emerald-500' : 'bg-slate-300')}>
      <span className={'absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-all ' + (on ? 'left-[18px]' : 'left-0.5')} />
    </button>
  );
}

function Row({ label, desc, children }: { label: string; desc?: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2.5 border-b border-slate-50 last:border-0">
      <div className="min-w-0">
        <div className="text-sm text-slate-700">{label}</div>
        {desc && <div className="text-[10px] text-slate-400 mt-0.5">{desc}</div>}
      </div>
      <div className="flex-shrink-0">{children}</div>
    </div>
  );
}

const SECTION_META = [
  { icon: Building2, title: '平台基础', desc: '平台名称、品牌、默认首页与区域' },
  { icon: ShieldCheck, title: '测试治理策略', desc: '门禁默认阈值、证据与资源配额' },
  { icon: Wallet, title: '成本与审计', desc: '成本单价、告警阈值与保留策略' },
  { icon: Lock, title: '安全与登录', desc: '登录方式、密码策略与 API 安全' },
  { icon: Bell, title: '通知与集成', desc: '通知渠道与 CI/CD 默认对接' },
  { icon: Server, title: '系统与维护', desc: '版本、数据导出与备份' },
];

const inputCls = 'px-3 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg outline-none focus:border-emerald-400 focus:bg-white w-40 text-right';
const selectCls = 'px-2.5 py-1.5 text-xs bg-white border border-slate-200 rounded-lg outline-none focus:border-emerald-400 w-40 text-right';

export default function SettingsPage() {
  const user = useCurrentUser();
  const isSuper = user.role === '团队所有者';
  const [maintenance, setMaintenance] = useState(false);
  const [antigCheat, setAntigCheat] = useState(true);
  const [pwLogin, setPwLogin] = useState(true);
  const [phoneLogin, setPhoneLogin] = useState(true);
  const [wechatLogin, setWechatLogin] = useState(true);
  const [sso, setSso] = useState(false);
  const [notifyBlock, setNotifyBlock] = useState(true);
  const [notifyReport, setNotifyReport] = useState(true);
  const [threshold, setThreshold] = useState('85');

  if (!isSuper) {
    return (
      <div className="card bg-white rounded-xl border border-slate-200 p-10 text-center max-w-xl mx-auto mt-10">
        <div className="w-12 h-12 rounded-full bg-red-50 text-red-500 flex items-center justify-center mx-auto mb-3"><ShieldAlert className="w-6 h-6" /></div>
        <div className="text-base font-semibold text-slate-800">无权限访问</div>
        <p className="text-[11px] text-slate-400 mt-2">系统设置为超级管理员专属，当前账号（{user.name} · {user.role}）无权访问。请切换为团队所有者账号。</p>
      </div>
    );
  }

  const save = () => toast.success('配置已保存（原型示意）', { description: '系统设置已应用，涉及安全项将即时生效' });

  return (
    <div>
      <PageHeader title="系统设置" desc="超级管理员系统级配置 · 普通用户 / 无权限账号不可见">
        <PrimaryButton onClick={save}><Save className="w-4 h-4" />保存全部设置</PrimaryButton>
      </PageHeader>

      <div className="grid grid-cols-2 gap-5">
        {/* 平台基础 */}
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <div className="flex items-center gap-2 mb-1">
            <Building2 className="w-4 h-4 text-emerald-500" />
            <h2 className="font-semibold text-slate-700 text-sm">{SECTION_META[0].title}</h2>
          </div>
          <div className="text-[10px] text-slate-400 mb-3">{SECTION_META[0].desc}</div>
          <Row label="平台名称"><input className={inputCls} defaultValue="GreenPass" /></Row>
          <Row label="品牌 Logo" desc="浏览器图标 / 登录页品牌"><button type="button" onClick={() => toast('上传（原型示意）')} className="text-[11px] text-emerald-600 border border-emerald-200 rounded-lg px-3 py-1.5 flex items-center gap-1"><Upload className="w-3 h-3" />上传</button></Row>
          <Row label="默认首页"><select className={selectCls} defaultValue="测试中心"><option>测试中心</option><option>被测对象</option><option>测试执行</option></select></Row>
          <Row label="时区 / 语言"><select className={selectCls} defaultValue="Asia/Shanghai · 简体中文"><option>Asia/Shanghai · 简体中文</option><option>UTC · English</option></select></Row>
          <Row label="维护模式" desc="开启后普通用户暂停操作"><Toggle on={maintenance} onChange={setMaintenance} /></Row>
        </div>

        {/* 测试治理策略 */}
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <div className="flex items-center gap-2 mb-1">
            <ShieldCheck className="w-4 h-4 text-emerald-500" />
            <h2 className="font-semibold text-slate-700 text-sm">{SECTION_META[1].title}</h2>
          </div>
          <div className="text-[10px] text-slate-400 mb-3">{SECTION_META[1].desc}</div>
          <Row label="门禁默认达标阈值" desc="低于该分的判定为阻断"><div className="flex items-center gap-1.5"><input className={inputCls} value={threshold} onChange={(e) => setThreshold(e.target.value)} /><span className="text-xs text-slate-500">分</span></div></Row>
          <Row label="防 AI 自放水判定" desc="判定阶段隔离 AI 生成与执行结果"><Toggle on={antigCheat} onChange={setAntigCheat} /></Row>
          <Row label="截图证据默认模式" desc="服务级可单独覆盖"><select className={selectCls} defaultValue="on-fail"><option value="always">始终截图</option><option value="on-fail">仅失败时</option><option value="off">关闭</option></select></Row>
          <Row label="资源配额默认" desc="真机 / 浏览器实例 / 沙箱"><input className={inputCls} defaultValue="真机 20 · 浏览器 40" /></Row>
        </div>

        {/* 成本与审计 */}
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <div className="flex items-center gap-2 mb-1">
            <Wallet className="w-4 h-4 text-emerald-500" />
            <h2 className="font-semibold text-slate-700 text-sm">{SECTION_META[2].title}</h2>
          </div>
          <div className="text-[10px] text-slate-400 mb-3">{SECTION_META[2].desc}</div>
          <Row label="成本单价模型" desc="模型 token / 真机时长价目"><select className={selectCls} defaultValue="默认价目表"><option>默认价目表</option><option>模型 token 差异化</option><option>自定义</option></select></Row>
          <Row label="成本告警阈值" desc="月度成本超限告警"><input className={inputCls} defaultValue="¥5000" /></Row>
          <Row label="审计日志保留" desc="操作 / 执行审计留存时长"><select className={selectCls} defaultValue="180 天"><option>90 天</option><option>180 天</option><option>365 天</option></select></Row>
          <Row label="测试历史保留" desc="用例执行历史 / 报告留存"><select className={selectCls} defaultValue="12 个月"><option>6 个月</option><option>12 个月</option><option>永久</option></select></Row>
        </div>

        {/* 安全与登录 */}
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <div className="flex items-center gap-2 mb-1">
            <Lock className="w-4 h-4 text-emerald-500" />
            <h2 className="font-semibold text-slate-700 text-sm">{SECTION_META[3].title}</h2>
          </div>
          <div className="text-[10px] text-slate-400 mb-3">{SECTION_META[3].desc}</div>
          <Row label="账号密码登录"><Toggle on={pwLogin} onChange={setPwLogin} /></Row>
          <Row label="手机号登录" desc="短信验证码 + 自动绑定"><Toggle on={phoneLogin} onChange={setPhoneLogin} /></Row>
          <Row label="微信扫码登录" desc="微信第三方授权"><Toggle on={wechatLogin} onChange={setWechatLogin} /></Row>
          <Row label="企业 SSO" desc="对接企业统一身份"><Toggle on={sso} onChange={setSso} /></Row>
          <Row label="密码策略"><select className={selectCls} defaultValue="8 位以上含大小写"><option>8 位以上含大小写</option><option>10 位以上含符号</option><option>SSO 管理</option></select></Row>
        </div>

        {/* 通知与集成 */}
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <div className="flex items-center gap-2 mb-1">
            <Bell className="w-4 h-4 text-emerald-500" />
            <h2 className="font-semibold text-slate-700 text-sm">{SECTION_META[4].title}</h2>
          </div>
          <div className="text-[10px] text-slate-400 mb-3">{SECTION_META[4].desc}</div>
          <Row label="通知渠道" desc="IM / 邮件 / Webhook"><input className={inputCls} defaultValue="IM + 邮件" /></Row>
          <Row label="门禁阻断通知" desc="判定阻断即时通知负责人"><Toggle on={notifyBlock} onChange={setNotifyBlock} /></Row>
          <Row label="报告完成通知"><Toggle on={notifyReport} onChange={setNotifyReport} /></Row>
          <Row label="CI/CD 默认连接" desc="流水线回写默认目标"><select className={selectCls} defaultValue="Jenkins + GitLab"><option>Jenkins + GitLab</option><option>GitHub Actions</option><option>自研流水线</option></select></Row>
        </div>

        {/* 系统与维护 */}
        <div className="card bg-white rounded-xl border border-slate-200 p-5">
          <div className="flex items-center gap-2 mb-1">
            <Server className="w-4 h-4 text-emerald-500" />
            <h2 className="font-semibold text-slate-700 text-sm">{SECTION_META[5].title}</h2>
          </div>
          <div className="text-[10px] text-slate-400 mb-3">{SECTION_META[5].desc}</div>
          <Row label="版本信息" desc="当前部署版本"><span className="text-xs text-slate-500 font-mono">v1.7.19 · 2026-09-27</span></Row>
          <Row label="数据导出" desc="导出配置 / 门禁 / 审计快照"><button type="button" onClick={() => toast('导出任务已创建（原型示意）')} className="text-[11px] text-emerald-600 border border-emerald-200 rounded-lg px-3 py-1.5">导出</button></Row>
          <Row label="备份策略"><select className={selectCls} defaultValue="每日自动"><option>每日自动</option><option>每周自动</option><option>手动</option></select></Row>
          <Row label="维护窗口" desc="允许维护的时段"><input className={inputCls} defaultValue="02:00 - 06:00" /></Row>
        </div>
      </div>
    </div>
  );
}
