/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly MIAODA_CLIENT_BASE_PATH: string;
  readonly VITE_GP_API_BASE?: string;
  readonly VITE_GP_USER_ID?: string;
  readonly VITE_GP_TEAM_ID?: string;
  readonly VITE_GP_MOCK_API?: string;
}
