import assert from 'node:assert/strict';
import { test } from 'node:test';
import { readFileSync } from 'node:fs';
import { equityChartData, candleKey, marketChartData, toCandles, toEquity, toIndicator, toMarkers, unixSeconds, validateCandleRange } from './chartData.ts';
import type { BacktestResult, CandlesResult, Trade } from '../types/api.ts';

const example: BacktestResult = JSON.parse(readFileSync(new URL('../../../docs/examples/btcusdt-sma-response.json', import.meta.url), 'utf8'));
// Small synthetic data is used only for isolated conversion/validation tests.
const first = { open_time: '2024-01-01T00:00:00Z', open: 100, high: 104, low: 99, close: 103, volume: 10 };
const second = { ...first, open_time: '2024-01-01T01:00:00Z', close: 102 };
const response: CandlesResult = { market: { symbol: 'BTCUSDT', interval: '1h', start: first.open_time, end: '2024-01-01T02:00:00Z', source: 'test' }, candles: [first,second] };
const trade: Trade = { time: second.open_time, type: 'buy', price: 100, quantity: 1, fee: .1, notional: 100, cash_after: 0, asset_quantity_after: 1 };

test('converts ISO UTC to Unix seconds for hourly and daily bars', () => {
  assert.equal(unixSeconds(first.open_time), 1704067200);
  assert.equal(unixSeconds('2024-01-02T00:00:00Z') - unixSeconds(first.open_time), 86400);
  assert.equal(unixSeconds('2024-01-01T00:00:00.999Z'), 1704067200);
  for (const time of ['bad', '2024-02-30T00:00:00Z', '2024-01-01T00:00:00+03:00', '2024-01-01Z'])
    assert.throws(() => unixSeconds(time));
});
test('converts OHLC, sorts a copy, deduplicates exact bars, rejects conflicts', () => {
  const input=[second,first,{...first}];
  const result=toCandles(input);
  assert.deepEqual(result,[{time:1704067200,open:100,high:104,low:99,close:103},{time:1704070800,open:100,high:104,low:99,close:102}]);
  assert.equal(input[0],second);
  assert.throws(()=>toCandles([first,{...first,close:101}]),/Conflicting/);
  assert.throws(()=>toCandles([{...first,high:1}]),/OHLC/);
  assert.throws(()=>toCandles([{...first,open:NaN}]),/non-finite/);
  assert.deepEqual(toCandles([]),[]);
});
test('validates full range, gaps, conflicting volume and returned market metadata', () => {
  assert.equal(validateCandleRange(response,response.market),response);
  assert.throws(()=>validateCandleRange({...response,candles:[first]},response.market),/incomplete/);
  assert.throws(()=>validateCandleRange({...response,candles:[]},response.market),/empty/);
  assert.throws(()=>validateCandleRange({...response,market:{...response.market,symbol:'ETHUSDT'}},response.market),/metadata/);
  assert.throws(()=>validateCandleRange({...response,candles:[first,second,{...first,volume:11}]},response.market),/volumes/);
  assert.notEqual(candleKey(response.market),candleKey({...response.market,end:'2024-01-01T03:00:00Z'}));
});
test('converts server SMA without recalculation and checks timestamp/index correspondence', () => {
  const candles=toCandles(response.candles);
  assert.deepEqual(toIndicator([{time:second.open_time,candle_index:1,value:102.345}],candles),[{time:1704070800,value:102.345}]);
  assert.throws(()=>toIndicator([{time:second.open_time,candle_index:0,value:102}],candles),/indices/);
  assert.deepEqual(toIndicator([],candles),[]);
});
test('BUY/SELL markers use actual execution N+1, with no shift or inferred signal time', () => {
  const candles=toCandles(response.candles);
  const markers=toMarkers([trade,{...trade,type:'sell'}],candles);
  assert.equal(markers[0].time,unixSeconds(second.open_time));
  assert.notEqual(markers[0].time,unixSeconds(first.open_time));
  assert.equal(markers[0].position,'belowBar');assert.equal(markers[0].shape,'arrowUp');
  assert.equal(markers[1].position,'aboveBar');assert.equal(markers[1].shape,'arrowDown');
  assert.throws(()=>toMarkers([{...trade,time:response.market.end}],candles),/execution timestamp/);
  assert.throws(()=>toMarkers([{...trade,time:'2024-01-01T01:00:00.001Z'}],candles),/execution timestamp/);
});
test('equity is copied from server values, sorted and validated, never recomputed', () => {
  const points=toEquity(example.equity_curve);
  assert.equal(points.length,168);
  assert.equal(points.at(-1)?.value,example.portfolio.final_equity);
  assert.deepEqual(toEquity([]),[]);
  assert.throws(()=>toEquity([{time:first.open_time,equity:1},{time:first.open_time,equity:2}]),/Conflicting/);
});
test('Buy & Hold removes SMA and incomplete SMA results are explicitly rejected', () => {
  const hold: BacktestResult={...example,market:response.market,strategy:{type:'buy_and_hold'},trades:[trade]};
  assert.deepEqual(marketChartData(hold,response).fast,[]);
  assert.deepEqual(marketChartData(hold,response).slow,[]);
  assert.throws(()=>marketChartData({...hold,strategy:{type:'sma_crossover',fast_period:1,slow_period:2},indicators:{fast_sma:[],slow_sma:[]}},response),/did not provide/);
});

test('real Binance candles align with the saved Go SMA, execution prices and equity period', () => {
  const candles: CandlesResult = JSON.parse(readFileSync(new URL('../test/fixtures/btcusdt-1h-2024-01-01-08.json', import.meta.url), 'utf8'));
  const data = marketChartData(example, candles);
  assert.equal(data.candles.length,168);
  assert.equal(data.fast.length,168);
  assert.equal(data.slow.length,168);
  assert.equal(data.markers.length,9);
  for(const trade of example.trades) {
    const bar = data.candles.find(c=>c.time===unixSeconds(trade.time));
    assert.equal(bar?.open,trade.price);
  }
  assert.equal(data.markers[0].time,unixSeconds('2024-01-01T11:00:00Z'));
});

test('equity coverage matches the result range and empty curves stay empty', () => {
  assert.equal(equityChartData(example).length,168);
  assert.deepEqual(equityChartData({...example,equity_curve:[]}),[]);
  assert.throws(()=>equityChartData({...example,equity_curve:example.equity_curve.slice(1)}),/does not cover/);
});
