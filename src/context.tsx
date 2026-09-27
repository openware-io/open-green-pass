import { createContext, useContext } from 'react';

export type AssetLevel = 'system' | 'group' | 'service' | 'module';

export const LEVEL_ORDER: AssetLevel[] = ['system', 'group', 'service', 'module'];
export const LEVEL_LABEL: Record<AssetLevel, string> = {
  system: '工程',
  group: '服务组',
  service: '服务',
  module: '模块',
};

export const AssetLevelContext = createContext<{ level: AssetLevel; setLevel: (l: AssetLevel) => void }>({
  level: 'service',
  setLevel: () => {},
});

export const useAssetLevel = () => useContext(AssetLevelContext);
