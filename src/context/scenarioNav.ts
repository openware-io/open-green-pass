import { useSyncExternalStore } from 'react';

export type ScenarioTab = 'cases' | 'exec' | 'gate' | 'history' | 'report';

export interface ScenarioNavState {
  focus: string | null;
  tab: ScenarioTab | null;
  nonce: number;
}

let state: ScenarioNavState = { focus: null, tab: null, nonce: 0 };
const listeners = new Set<() => void>();
function emit() {
  listeners.forEach((l) => l());
}

export const scenarioNav = {
  get: (): ScenarioNavState => state,
  subscribe: (l: () => void) => {
    listeners.add(l);
    return () => {
      listeners.delete(l);
    };
  },
  /** 全局各页下钻：聚焦测试中心某场景（可选定位到闭环 tab） */
  go: (focus: string, tab?: ScenarioTab) => {
    state = { focus, tab: tab ?? null, nonce: state.nonce + 1 };
    emit();
  },
};

export function useScenarioNav(): ScenarioNavState {
  return useSyncExternalStore(scenarioNav.subscribe, scenarioNav.get);
}
