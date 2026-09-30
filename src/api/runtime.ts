import { GreenPassClient } from './client';

function configuredTeamId(): number | undefined {
  const raw = import.meta.env.VITE_GP_TEAM_ID?.trim();
  if (!raw) return undefined;
  const teamId = Number(raw);
  return Number.isSafeInteger(teamId) && teamId > 0 ? teamId : undefined;
}

const baseUrl = import.meta.env.VITE_GP_API_BASE?.trim();
const teamId = configuredTeamId();

/**
 * Returns a real API client only when both endpoint and tenant are explicitly
 * configured. The frontend never guesses a tenant from prototype data.
 */
export function configuredGreenPassClient(): GreenPassClient | undefined {
  if (!baseUrl || !teamId) return undefined;
  return new GreenPassClient({ baseUrl, teamId });
}

export function greenPassConnectionHint(): string {
  if (!baseUrl) return '未配置 VITE_GP_API_BASE';
  if (!teamId) return '未配置有效的 VITE_GP_TEAM_ID';
  return `${baseUrl.replace(/\/$/, '')} · team ${teamId}`;
}
