import { Routes, Route, Navigate } from 'react-router-dom';
import { Toaster } from '@/components/ui/sonner';
import { Layout } from '@/components/Layout';
import TargetPage from '@/pages/TargetPage/TargetPage';
import TracePage from '@/pages/TracePage/TracePage';
import GenerationPage from '@/pages/GenerationPage/GenerationPage';
import CasesPage from '@/pages/CasesPage/CasesPage';
import ContractsPage from '@/pages/ContractsPage/ContractsPage';
import ExecPage from '@/pages/ExecPage/ExecPage';
import GatePage from '@/pages/GatePage/GatePage';
import HistoryPage from '@/pages/HistoryPage/HistoryPage';
import ReportPage from '@/pages/ReportPage/ReportPage';
import AuditPage from '@/pages/AuditPage/AuditPage';
import AuditCostPage from '@/pages/AuditCostPage/AuditCostPage';
import AuditExecPage from '@/pages/AuditExecPage/AuditExecPage';
import AuditOpPage from '@/pages/AuditOpPage/AuditOpPage';
import ConcurrencyPage from '@/pages/ConcurrencyPage/ConcurrencyPage';
import ScenarioCenterPage from '@/pages/ScenarioCenterPage/ScenarioCenterPage';
import TeamPage from '@/pages/TeamPage/TeamPage';
import ModelPage from '@/pages/ModelPage/ModelPage';
import NotFoundPage from '@/pages/NotFoundPage/NotFoundPage';

export default function App() {
  return (
    <>
      <Routes>
      <Route element={<Layout />}>
        <Route index element={<Navigate to="/scenarios" replace />} />
        <Route path="target" element={<TargetPage />} />
        <Route path="trace" element={<TracePage />} />
        <Route path="generation" element={<GenerationPage />} />
        <Route path="cases" element={<CasesPage />} />
        <Route path="contracts" element={<ContractsPage />} />
        <Route path="exec" element={<ExecPage />} />
        <Route path="gate" element={<GatePage />} />
        <Route path="history" element={<HistoryPage />} />
        <Route path="report" element={<ReportPage />} />
        <Route path="audit" element={<AuditPage />} />
        <Route path="audit-cost" element={<AuditCostPage />} />
        <Route path="audit-exec" element={<AuditExecPage />} />
        <Route path="audit-op" element={<AuditOpPage />} />
        <Route path="resources" element={<ConcurrencyPage />} />
        <Route path="scenarios" element={<ScenarioCenterPage />} />
        <Route path="teams" element={<TeamPage />} />
        <Route path="models" element={<ModelPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
      </Routes>
      <Toaster richColors position="top-center" />
    </>
  );
}
