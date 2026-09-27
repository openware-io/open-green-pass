import { useSyncExternalStore } from 'react';
import { ACCOUNTS, type IAccount } from '@/data/mock';

let current: IAccount = ACCOUNTS[0];
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
  /** 设置当前登录账号（登录 / 切换账号） */
  setCurrent: (u: IAccount) => {
    current = u;
    emit();
  },
};

export function useCurrentUser(): IAccount {
  return useSyncExternalStore(loginStore.subscribe, loginStore.get);
}
