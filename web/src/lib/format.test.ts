import assert from 'node:assert/strict';
import { test } from 'node:test';
import { money, number, percent, signed, tone, utcTime } from './format.ts';

test('formats financial values without mutating or rounding source data', () => {
  const original=3.2907927505975376;
  assert.equal(signed(original), '+3.29%');
  assert.equal(original, 3.2907927505975376);
  assert.equal(signed(-2.15), '−2.15%');
  assert.equal(signed(-0.028565061319807405, ' pp'), '−0.03 pp');
  assert.equal(signed(0), '0.00%');
  assert.equal(money(10329.079275059754), '10,329.08 USDT');
  assert.equal(percent(25), '25.00%');
  assert.equal(number(2.60268), '2.60');
  assert.equal(number(4,0), '4');
});
test('undefined financial metrics remain em dashes, never zero', () => {
  for(const fn of [money, number, percent, signed]) assert.equal(fn(null), '—');
  assert.equal(tone(0), '');
  assert.equal(tone(1), 'metric-value-positive');
  assert.equal(tone(-1), 'metric-value-negative');
});
test('timestamps are explicitly formatted in UTC', () => {
  assert.equal(utcTime('2024-01-01T11:00:00Z'), '01 Jan 2024, 11:00');
});
