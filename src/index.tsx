import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { HashRouter } from 'react-router-dom';
import { ErrorBoundary } from 'react-error-boundary';
import { ErrorFallback } from '@/components/ErrorFallback';
import App from './app';
import './index.css';

async function start() {
  if (import.meta.env.VITE_GP_MOCK_API?.trim().toLowerCase() === 'true' && !import.meta.env.VITE_GP_API_BASE) {
    const { worker } = await import('./mocks/browser');
    await worker.start({ onUnhandledFrame: 'bypass' });
  }
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <HashRouter>
        <ErrorBoundary FallbackComponent={ErrorFallback}>
          <App />
        </ErrorBoundary>
      </HashRouter>
    </StrictMode>,
  );
}

void start();
