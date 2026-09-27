import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { ACCOUNTS, type IAccount } from '@/data/mock';
import { loginStore } from '@/context/login';
import { Check, Lock, Smartphone, ScanLine, UserPlus, ShieldCheck, GitBranch, Wallet, ChevronRight, ArrowLeft } from 'lucide-react';

const FEATURES = [
  { icon: ShieldCheck, title: '质量门禁', desc: '防 AI 自放水的判定，管控放行' },
  { icon: GitBranch, title: '可信审计', desc: '哈希链留痕，成本与执行全可溯' },
  { icon: Wallet, title: '成本治理', desc: '用例级成本明细，一目了然' },
];

// 微信二维码占位（SVG 模拟码点，示意）
function WechatQR() {
  const cells = [] as { x: number; y: number }[];
  for (let i = 0; i < 21; i++) for (let j = 0; j < 21; j++) if ((i * 7 + j * 13) % 3 !== 0) cells.push({ x: i, y: j });
  return (
    <svg viewBox="0 0 21 21" className="w-full h-full">
      <rect width="21" height="21" fill="white" />
      {cells.map((c, i) => (
        <rect key={i} x={c.x} y={c.y} width="1" height="1" fill="#1f2937" />
      ))}
      <rect x="0" y="0" width="7" height="7" fill="white" stroke="#1f2937" strokeWidth="0.6" />
      <rect x="14" y="0" width="7" height="7" fill="white" stroke="#1f2937" strokeWidth="0.6" />
      <rect x="0" y="14" width="7" height="7" fill="white" stroke="#1f2937" strokeWidth="0.6" />
    </svg>
  );
}

function ApplyForm({ onDone }: { onDone: () => void }) {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [team, setTeam] = useState('');
  const submit = () => {
    if (!name.trim() || !email.trim()) { toast.warning('请填写姓名与企业邮箱'); return; }
    toast.success('账号申请已提交', { description: `「${name}」· ${email} · ${team || '默认团队'}，管理员审核后开通（原型示意）` });
    onDone();
  };
  return (
    <div className="mt-5 border border-emerald-200 rounded-xl p-4 bg-emerald-50/40 space-y-3">
      <div className="text-xs font-medium text-slate-700 flex items-center gap-1.5"><UserPlus className="w-3.5 h-3.5 text-emerald-600" />申请自有账号</div>
      <input value={name} onChange={(e) => setName(e.target.value)} placeholder="姓名" className="w-full px-3 py-2 text-sm bg-white border border-slate-200 rounded-lg outline-none focus:border-emerald-400" />
      <input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="企业邮箱" className="w-full px-3 py-2 text-sm bg-white border border-slate-200 rounded-lg outline-none focus:border-emerald-400" />
      <input value={team} onChange={(e) => setTeam(e.target.value)} placeholder="所属团队（可选）" className="w-full px-3 py-2 text-sm bg-white border border-slate-200 rounded-lg outline-none focus:border-emerald-400" />
      <textarea rows={2} placeholder="申请理由（可选）" className="w-full px-3 py-2 text-sm bg-white border border-slate-200 rounded-lg outline-none focus:border-emerald-400 resize-none" />
      <button type="button" onClick={submit} className="w-full py-2.5 text-sm font-medium text-white bg-emerald-600 hover:bg-emerald-700 rounded-lg transition">提交申请</button>
    </div>
  );
}

export default function LoginPage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState<'password' | 'phone' | 'wechat'>('password');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [phone, setPhone] = useState('');
  const [code, setCode] = useState('');
  const [applyOpen, setApplyOpen] = useState(false);
  const [sent, setSent] = useState(false);

  const enter = (u: IAccount, via: IAccount['via']) => {
    loginStore.setCurrent({ ...u, via });
    toast.success('登录成功', { description: `欢迎回来，${u.name} · ${u.role}` });
    navigate('/scenarios');
  };

  const loginByPassword = () => {
    if (!username.trim() || !password.trim()) { toast.warning('请输入账号与密码'); return; }
    const acc = ACCOUNTS.find((a) => a.username === username.trim().toLowerCase()) ?? ACCOUNTS[0];
    enter(acc, 'password');
  };
  const loginByPhone = () => {
    if (phone.trim().length < 11) { toast.warning('请输入 11 位手机号'); return; }
    if (!sent) { toast.warning('请先获取短信验证码'); return; }
    const acc = ACCOUNTS.find((a) => a.phone.replace('*', '').startsWith(phone.slice(0, 3))) ?? ACCOUNTS[2];
    enter({ ...acc, phone: phone.replace(/(\d{3})\d{4}(\d{4})/, '$1****$2') }, 'phone');
  };
  const loginByWechat = () => {
    // 扫码后：已绑定微信的账号直接进入；未绑定则新建并绑定微信（原型示意）
    const bound = ACCOUNTS.find((a) => a.wechat);
    enter(bound ?? { ...ACCOUNTS[2], wechat: 'wx_' + username || 'greenpass_user' }, 'wechat');
  };

  return (
    <div className="min-h-screen flex bg-slate-50">
      {/* 左栏：品牌区 */}
      <div className="hidden lg:flex w-[46%] bg-gradient-to-br from-emerald-700 via-emerald-600 to-teal-600 text-white flex-col justify-between p-12">
        <div>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-white/15 backdrop-blur flex items-center justify-center"><Check className="w-5 h-5" strokeWidth={3} /></div>
            <div>
              <div className="text-lg font-semibold">GreenPass</div>
              <div className="text-emerald-100/80 text-xs">软件工程测试治理平台</div>
            </div>
          </div>
          <h1 className="mt-10 text-3xl font-bold leading-snug">质量放行的<br />统一守门人</h1>
          <p className="mt-4 text-emerald-100/80 text-sm leading-relaxed max-w-md">通用软件工程测试治理——从用例生成、测试执行、质量门禁到成本审计的可信闭环，为 AI 与人协作的研发流程护航。</p>
        </div>
        <div className="space-y-3">
          {FEATURES.map((f) => {
            const Icon = f.icon;
            return (
              <div key={f.title} className="flex items-center gap-3 bg-white/10 rounded-xl px-4 py-3">
                <Icon className="w-5 h-5 text-emerald-100 flex-shrink-0" />
                <div>
                  <div className="text-sm font-medium">{f.title}</div>
                  <div className="text-[11px] text-emerald-100/70">{f.desc}</div>
                </div>
              </div>
            );
          })}
        </div>
        <div className="text-[11px] text-emerald-100/60">© 2026 openware-io · 软件工程测试治理平台</div>
      </div>

      {/* 右栏：登录卡片 */}
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="w-full max-w-md">
          <button type="button" onClick={() => navigate('/scenarios')} className="text-xs text-slate-400 hover:text-emerald-600 flex items-center gap-1 mb-6"><ArrowLeft className="w-3.5 h-3.5" />返回演示首页</button>
          <div className="card bg-white rounded-2xl border border-slate-200 shadow-sm p-8">
            <h2 className="text-xl font-semibold text-slate-800">登录 GreenPass</h2>
            <p className="text-[11px] text-slate-400 mt-1 mb-5">支持自有账号、手机号与微信扫码，账号可绑定手机 / 微信</p>

            <div className="flex rounded-lg bg-slate-100 p-0.5 text-[12px] mb-5">
              {([['password', '账号密码'], ['phone', '手机号'], ['wechat', '微信扫码']] as const).map(([k, label]) => (
                <button key={k} type="button" onClick={() => setTab(k)}
                  className={'flex-1 py-1.5 rounded flex items-center justify-center gap-1 transition ' + (tab === k ? 'bg-white text-emerald-600 shadow-sm font-medium' : 'text-slate-500')}>
                  {k === 'password' ? <Lock className="w-3 h-3" /> : k === 'phone' ? <Smartphone className="w-3 h-3" /> : <ScanLine className="w-3 h-3" />}
                  {label}
                </button>
              ))}
            </div>

            {tab === 'password' && (
              <div className="space-y-3">
                <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="账号（如 zhangli）" className="w-full px-3.5 py-2.5 text-sm bg-slate-50 border border-slate-200 rounded-lg outline-none focus:border-emerald-400 focus:bg-white transition" />
                <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="密码" className="w-full px-3.5 py-2.5 text-sm bg-slate-50 border border-slate-200 rounded-lg outline-none focus:border-emerald-400 focus:bg-white transition" />
                <button type="button" onClick={loginByPassword} className="w-full py-2.5 text-sm font-medium text-white bg-emerald-600 hover:bg-emerald-700 rounded-lg transition">登录</button>
                <div className="flex items-center justify-between text-[11px] text-slate-400">
                  <button type="button" onClick={() => setApplyOpen(!applyOpen)} className="text-emerald-600 hover:underline flex items-center gap-0.5">申请账号</button>
                  <button type="button" onClick={() => toast('原型示意', { description: '忘记密码走企业 SSO / 管理员重置' })} className="hover:text-emerald-600">忘记密码</button>
                </div>
                {applyOpen && <ApplyForm onDone={() => setApplyOpen(false)} />}
              </div>
            )}

            {tab === 'phone' && (
              <div className="space-y-3">
                <input value={phone} onChange={(e) => setPhone(e.target.value.replace(/\D/g, '').slice(0, 11))} placeholder="11 位手机号（如 13800002101）" className="w-full px-3.5 py-2.5 text-sm bg-slate-50 border border-slate-200 rounded-lg outline-none focus:border-emerald-400 focus:bg-white transition" />
                <div className="flex gap-2">
                  <input value={code} onChange={(e) => setCode(e.target.value)} placeholder="短信验证码" className="flex-1 px-3.5 py-2.5 text-sm bg-slate-50 border border-slate-200 rounded-lg outline-none focus:border-emerald-400 focus:bg-white transition" />
                  <button type="button" onClick={() => { setSent(true); toast.success('验证码已发送（原型示意：1234）', { description: '新手机号将自动注册并绑定' }); }}
                    className="px-3 text-[11px] font-medium text-emerald-600 border border-emerald-200 rounded-lg hover:bg-emerald-50 flex-shrink-0">{sent ? '已发送' : '获取验证码'}</button>
                </div>
                <button type="button" onClick={loginByPhone} className="w-full py-2.5 text-sm font-medium text-white bg-emerald-600 hover:bg-emerald-700 rounded-lg transition">手机号登录</button>
                <p className="text-[11px] text-slate-400">未注册手机号：验证码登录后自动创建账号并绑定该手机号</p>
              </div>
            )}

            {tab === 'wechat' && (
              <div className="space-y-3">
                <div className="flex justify-center">
                  <div className="w-44 h-44 border-2 border-emerald-100 rounded-xl p-3 bg-white relative">
                    <WechatQR />
                    <div className="absolute inset-0 flex items-center justify-center">
                      <div className="bg-white/90 rounded px-2 py-1 text-[10px] text-emerald-600 border border-emerald-200">示意二维码</div>
                    </div>
                  </div>
                </div>
                <p className="text-center text-[11px] text-slate-400">使用微信扫一扫，扫码后关联自有账号登录</p>
                <button type="button" onClick={loginByWechat} className="w-full py-2.5 text-sm font-medium text-white bg-[#07c160] hover:bg-[#06ad56] rounded-lg transition flex items-center justify-center gap-1.5"><ScanLine className="w-4 h-4" />模拟扫码成功，微信登录</button>
                <p className="text-[11px] text-slate-400 text-center">已绑定微信的账号将直接进入；未绑定则自动创建账号并绑定该微信号</p>
              </div>
            )}

            <div className="mt-6 pt-4 border-t border-slate-100 flex items-center justify-between text-[11px] text-slate-400">
              <span>没有账号？</span>
              <button type="button" onClick={() => { setTab('password'); setApplyOpen(true); }} className="text-emerald-600 hover:underline flex items-center gap-0.5">申请自有账号<ChevronRight className="w-3 h-3" /></button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
