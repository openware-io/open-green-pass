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
import AuditPage from '@/pages/AuditPage/AuditPage';
import ConcurrencyPage from '@/pages/ConcurrencyPage/ConcurrencyPage';
import TeamPage from '@/pages/TeamPage/TeamPage';
import ModelPage from '@/pages/ModelPage/ModelPage';
import NotFoundPage from '@/pages/NotFoundPage/NotFoundPage';

export default function App() {
  return (
    <>
      <Routes>
      <Route element={<Layout />}>
        <Route index element={<Navigate to="/target" replace />} />
        <Route path="target" element={<TargetPage />} />
        <Route path="trace" element={<TracePage />} />
        <Route path="generation" element={<GenerationPage />} />
        <Route path="cases" element={<CasesPage />} />
        <Route path="contracts" element={<ContractsPage />} />
        <Route path="exec" element={<ExecPage />} />
        <Route path="gate" element={<GatePage />} />
        <Route path="audit" element={<AuditPage />} />
        <Route path="concurrency" element={<ConcurrencyPage />} />
        <Route path="teams" element={<TeamPage />} />
        <Route path="models" element={<ModelPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
      </Routes>
      <Toaster richColors position="top-center" />
    </>
  );
}
