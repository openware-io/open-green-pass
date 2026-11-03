/**
 * Pure presentation helpers for the preflight estimate.
 *
 * Kept dependency-free so it can be unit-tested with the Node test runner
 * (the repo has no vitest installed) and reused by PlanPreview/Drawer.
 */
export type PlanRiskLike = { code?: string; severity?: string };
export type PlanEstimateLike = {
  estimate?: { unknown?: boolean; cost_amount?: number; currency?: string };
  risks?: PlanRiskLike[];
};

/** Risk codes the backend emits when a cost price is not configured. */
export const PRICE_MISSING_RISK_CODES = ['price_missing', 'price_unavailable', 'cost_price_missing'] as const;

export function hasPriceMissingRisk(plan: PlanEstimateLike): boolean {
  const risks = plan?.risks ?? [];
  return risks.some((risk) => PRICE_MISSING_RISK_CODES.includes((risk?.code ?? '') as typeof PRICE_MISSING_RISK_CODES[number]));
}

export function formatCost(amount?: number, currency = 'CNY'): string {
  if (typeof amount !== 'number' || !Number.isFinite(amount)) return '未知';
  const symbol = currency === 'CNY' ? '¥' : currency === 'USD' ? '$' : `${currency} `;
  return `${symbol}${amount.toFixed(amount > 0 && amount < 0.01 ? 4 : 2)}`;
}

export type PlanCostView = { known: boolean; text: string; hint: string };

/**
 * Cost display for the plan estimate:
 * - unknown estimate (no history)          -> 未知
 * - known tokens but no configured price   -> 未知（待配置单价），never ¥0.00
 * - otherwise                              -> formatted amount
 */
export function planCostView(plan: PlanEstimateLike): PlanCostView {
  const estimate = plan?.estimate;
  if (!estimate || estimate.unknown) {
    return { known: false, text: '未知', hint: '暂无历史生成数据，不显示 0' };
  }
  if (hasPriceMissingRisk(plan)) {
    return { known: false, text: '未知（待配置单价）', hint: '未配置模型单价，无法估算成本' };
  }
  if (typeof estimate.cost_amount !== 'number' || !Number.isFinite(estimate.cost_amount)) {
    return { known: false, text: '未知', hint: '暂无定价' };
  }
  return { known: true, text: formatCost(estimate.cost_amount, estimate.currency), hint: '' };
}
