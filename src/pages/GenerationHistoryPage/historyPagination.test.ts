import assert from 'node:assert/strict';
import test from 'node:test';
import {
  filterRunPageByState,
  hasNextPage,
  mergeRunPages,
  parseHistoryStateFilter,
} from './historyPagination.ts';

test('cursor page merge de-duplicates and preserves id-desc order', () => {
  const page1 = [{ id: 30 }, { id: 20 }, { id: 10 }];
  const page2 = [{ id: 20 }, { id: 10 }, { id: 5 }];
  assert.deepEqual(mergeRunPages(page1, page2), [{ id: 30 }, { id: 20 }, { id: 10 }, { id: 5 }]);
  assert.deepEqual(mergeRunPages([], page2), page2);
});

test('legacy single-page filtering by run state', () => {
  const runs = [{ id: 3, state: 'succeeded' }, { id: 2, state: 'cancelled' }, { id: 1, state: 'succeeded' }];
  assert.deepEqual(filterRunPageByState(runs, 'succeeded'), [{ id: 3, state: 'succeeded' }, { id: 1, state: 'succeeded' }]);
  assert.deepEqual(filterRunPageByState(runs, 'cancelled'), [{ id: 2, state: 'cancelled' }]);
  assert.equal(filterRunPageByState(runs, 'all').length, 3);
});

test('history state filter comes from a validated URL query value', () => {
  assert.equal(parseHistoryStateFilter(null), 'all');
  assert.equal(parseHistoryStateFilter(''), 'all');
  assert.equal(parseHistoryStateFilter('waiting_review'), 'waiting_review');
  assert.equal(parseHistoryStateFilter('succeeded'), 'succeeded');
  assert.equal(parseHistoryStateFilter('bogus'), 'all');
  assert.equal(parseHistoryStateFilter('RUNNING'), 'all');
});

test('next page only when a non-empty cursor is present', () => {
  assert.equal(hasNextPage('2933567498752000'), true);
  assert.equal(hasNextPage(null), false);
  assert.equal(hasNextPage(''), false);
  assert.equal(hasNextPage(undefined), false);
});
