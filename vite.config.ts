import fs from 'node:fs';
import path from 'node:path';
import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// ── 妙搭部署协议（参考《妙搭应用构建产物规范》）─────────────────────────
// 构建期环境变量（托管构建时由部署链路自动注入；本地开发不需要，缺省回退）：
//   MIAODA_CLIENT_BASE_PATH     应用路由根目录，如 /app/app_xxx
//   MIAODA_RESOURCE_CDN_PREFIX  JS/CSS 静态资源 CDN 前缀
const basePath = process.env.MIAODA_CLIENT_BASE_PATH || '/';
const cdnPrefix = process.env.MIAODA_RESOURCE_CDN_PREFIX;

// 产物分层：vite 原生产物（dist/client，中间产物，整理后删除）→ 妙搭托管产物结构：
//   dist/output/           index.html + public 同源资源 + routes.json（走应用权限校验）
//   dist/output_resource/  assets JS/CSS（推 CDN，公开）
function miaodaOutputPlugin(): Plugin {
  return {
    name: 'miaoda-output',
    apply: 'build',
    // closeBundle 在 vite 全部写盘后执行，此时可安全整理并清理中间产物
    closeBundle() {
      const dist = path.resolve(import.meta.dirname, 'dist');
      const client = path.join(dist, 'client');
      const output = path.join(dist, 'output');
      const outputResource = path.join(dist, 'output_resource');

      fs.rmSync(output, { recursive: true, force: true });
      fs.rmSync(outputResource, { recursive: true, force: true });
      fs.mkdirSync(output, { recursive: true });

      // assets 之外的所有产物（index.html + public/ 平铺文件）→ 同源 output/
      for (const entry of fs.readdirSync(client)) {
        if (entry === 'assets') continue;
        fs.cpSync(path.join(client, entry), path.join(output, entry), {
          recursive: true,
        });
      }
      // assets → CDN 桶
      const assets = path.join(client, 'assets');
      if (fs.existsSync(assets)) {
        fs.cpSync(assets, path.join(outputResource, 'assets'), {
          recursive: true,
        });
      }
      // routes.json：扫描 src 内 <Route path> 生成路由枚举（TNS 按此遍历送审）；
      // SPA 场景各路由均由 index.html 服务
      const routes = collectRoutePaths(
        path.resolve(import.meta.dirname, 'src'),
      ).map((p) => ({ path: p, file: 'index.html' }));
      fs.writeFileSync(
        path.join(output, 'routes.json'),
        JSON.stringify(routes, null, 2) + '\n',
      );
      // 清理中间产物，dist 只保留部署分层
      fs.rmSync(client, { recursive: true, force: true });
    },
  };
}

// 本地开发自描述端点（协议 v0.4）：GET /spark.json 原样返回项目根声明文件，
// 供消费方（豆包客户端/Agent）识别妙搭托管协议应用；仅 dev server，线上不暴露
function sparkJsonPlugin(): Plugin {
  return {
    name: 'spark-json-endpoint',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use('/spark.json', (_req, res) => {
        res.setHeader('Content-Type', 'application/json');
        res.setHeader('Access-Control-Allow-Origin', '*');
        res.setHeader('Cache-Control', 'no-store');
        res.end(
          fs.readFileSync(path.resolve(import.meta.dirname, 'spark.json')),
        );
      });
    },
  };
}

// 收集 <Route path="..."> 声明的路由；index 路由计为 "/"，通配 "*" 不进枚举
function collectRoutePaths(srcDir: string): string[] {
  const paths = new Set<string>(['/']);
  const walk = (dir: string) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(p);
      } else if (/\.(tsx|jsx|ts|js)$/.test(entry.name)) {
        const code = fs.readFileSync(p, 'utf-8');
        for (const m of code.matchAll(/<Route[^>]*\bpath=["']([^"']+)["']/g)) {
          const route = m[1];
          if (route.includes('*')) continue;
          paths.add(route.startsWith('/') ? route : `/${route}`);
        }
      }
    }
  };
  walk(srcDir);
  return [...paths];
}

// ── 生产 chunk 拆分 ────────────────────────────────────────────────────
// 单入口 SPA 默认把 React/图表/页面代码全部打进 index-*.js（~980KB）。
// 按“依赖域 + 页面域”拆成稳定 chunk：shared 入口变小、页面按域并行加载并缓存，
// 避免首屏加载一个超大文件。命名稳定（文件名带 hash，内容变才失效）。
// 注：页面仍是静态 import（不改变路由/首屏行为）；如需真正按路由懒加载，
// 可后续把 app.tsx 的页面 import 改为 React.lazy（会引入 Suspense fallback）。
const PAGE_CHUNKS: Record<string, string> = {
  GenerationPage: 'page-generation',
  GenerationHistoryPage: 'page-generation',
  TargetPage: 'page-target',
  TargetEnvPage: 'page-target',
  CasesPage: 'page-cases-exec',
  ExecPage: 'page-cases-exec',
  GatePage: 'page-governance',
  FindingsPage: 'page-governance',
  HistoryPage: 'page-governance',
  ReportPage: 'page-governance',
  AuditPage: 'page-governance',
  AuditCostPage: 'page-governance',
  AuditExecPage: 'page-governance',
  AuditOpPage: 'page-governance',
  TeamPage: 'page-platform',
  ModelPage: 'page-platform',
  SettingsPage: 'page-platform',
  CicdPage: 'page-platform',
  CicdTriggerPage: 'page-platform',
  CicdConnectorPage: 'page-platform',
  ConcurrencyPage: 'page-platform',
  ScenarioCenterPage: 'page-platform',
  AssistantPage: 'page-platform',
  TracePage: 'page-platform',
  LoginPage: 'page-access',
  NotFoundPage: 'page-access',
};

function manualChunks(id: string): string | undefined {
  const normalized = id.replace(/\\/g, '/');
  if (!normalized.includes('node_modules')) {
    // src 内页面按功能域拆分；shared/components/api 留在入口 chunk。
    const pageMatch = normalized.match(/\/src\/pages\/([^/]+)\//);
    if (pageMatch) return PAGE_CHUNKS[pageMatch[1]] ?? 'page-other';
    return undefined;
  }
  // React runtime + router（所有页面共享，必须最早加载）。
  if (normalized.includes('/react/') || normalized.includes('/react-dom/') || normalized.includes('/scheduler/') || normalized.includes('/react-router')) {
    return 'react-vendor';
  }
  // Radix/UI 原子组件（Dialog/Drawer/Select/Command 等）。
  if (normalized.includes('@radix-ui') || normalized.includes('/vaul/') || normalized.includes('/cmdk/') || normalized.includes('@floating-ui') || normalized.includes('react-remove-scroll') || normalized.includes('/aria-hidden/')) {
    return 'ui-vendor';
  }
  // 图表：echarts（AI 覆盖率/执行面板）与 recharts/d3（报告/成本）。
  if (normalized.includes('/echarts/') || normalized.includes('/zrender/')) return 'echarts-vendor';
  if (normalized.includes('/recharts/') || normalized.includes('/d3-') || normalized.includes('/victory-vendor/')) return 'charts-vendor';
  // 动效与交互增强。
  if (normalized.includes('framer-motion') || normalized.includes('/motion-dom/') || normalized.includes('/motion-utils/') || normalized.includes('/gsap/') || normalized.includes('@gsap/') || normalized.includes('@formkit')) {
    return 'motion-vendor';
  }
  // Markdown/助手渲染。
  if (normalized.includes('react-markdown') || normalized.includes('remark-') || normalized.includes('/micromark') || normalized.includes('/mdast-') || normalized.includes('/hast-') || normalized.includes('/unist-') || normalized.includes('/unified/') || normalized.includes('/vfile') || normalized.includes('property-information') || normalized.includes('space-separated-tokens') || normalized.includes('comma-separated-tokens') || normalized.includes('html-url-attributes') || normalized.includes('trim-lines') || normalized.includes('/devlop/')) {
    return 'markdown-vendor';
  }
  // 表单/校验/日期。
  if (normalized.includes('react-hook-form') || normalized.includes('@hookform') || normalized.includes('/zod/') || normalized.includes('date-fns') || normalized.includes('react-day-picker') || normalized.includes('input-otp')) {
    return 'form-vendor';
  }
  if (normalized.includes('lucide-react')) return 'icons-vendor';
  return 'vendor';
}

export default defineConfig(({ command }) => ({
  plugins: [react(), tailwindcss(), miaodaOutputPlugin(), sparkJsonPlugin()],
  // 生产构建：JS/CSS 引用带 CDN 前缀（无 CDN 时退回 base path）；dev 恒为 /
  base: command === 'build' ? cdnPrefix || basePath : '/',
  define: {
    // 路由 basename 与资源前缀解耦，单独注入
    'import.meta.env.MIAODA_CLIENT_BASE_PATH': JSON.stringify(basePath),
  },
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, 'src'),
    },
  },
  build: {
    outDir: 'dist/client',
    // Vite 8/rolldown：output.manualChunks 等价于 codeSplitting 分组（保持稳定 vendor 命名）。
    rolldownOptions: {
      output: {
        manualChunks,
      },
    },
  },
}));
