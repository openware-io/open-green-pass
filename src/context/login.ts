import { useSyncExternalStore } from 'react';
import { ACCOUNTS, type IAccount } from '@/data/mock';

const KEY = 'greenpass_current_user_id';

function load(): IAccount {
  try {
    const id = localStorage.getItem(KEY);
    const u = ACCOUNTS.find((a) => a.id === id);
    if (u) return u;
  } catch {
    /* 忽略存储不可用 */
  }
  return ACCOUNTS[0];
}

let current: IAccount = load();
const listeners = new Set<() => void>();
function emit() {
  listeners.forEach((l) => l());
}

export const loginStore = {
  get: (): IAccount => current,
  subscribe: (l: () => void) => {
    listeners.add(l);
    return () => {
      listeners.delete(l);
    };
  },
  /** 设置当前登录账号（登录 / 切换账号），并持久化到 localStorage 以跨刷新保留 */
  setCurrent: (u: IAccount) => {
    current = u;
    try {
      localStorage.setItem(KEY, u.id);
    } catch {
      /* 忽略存储不可用 */
    }
    emit();
  },
};

export function useCurrentUser(): IAccount {
  return useSyncExternalStore(loginStore.subscribe, loginStore.get);
}
