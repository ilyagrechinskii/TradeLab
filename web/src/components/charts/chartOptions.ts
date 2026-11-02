import { locale } from '../../i18n/config';
import { ColorType, CrosshairMode } from 'lightweight-charts';
import type { DeepPartial, ChartOptions, Time, UTCTimestamp } from 'lightweight-charts';
import { number, utcTime } from '../../lib/format';

export function chartOptions(height: number, interval: string): DeepPartial<ChartOptions> {
  return {
    height, layout: { background: { type: ColorType.Solid, color: '#101010' }, textColor: '#949494', fontSize: 11, attributionLogo: true },
    grid: { vertLines: { color: 'rgba(255,255,255,0.04)' }, horzLines: { color: 'rgba(255,255,255,0.04)' } },
    crosshair: { mode: CrosshairMode.Normal, vertLine: { color: '#777777', labelBackgroundColor: '#333333' }, horzLine: { color: '#777777', labelBackgroundColor: '#333333' } },
    rightPriceScale: { borderColor: 'rgba(255,255,255,0.07)', scaleMargins: { top: .14, bottom: .12 } },
    timeScale: { borderColor: 'rgba(255,255,255,0.07)', timeVisible: !interval.endsWith('d'), secondsVisible: false, rightOffset: 3 },
    localization: { locale: locale(), priceFormatter: (value: number) => number(value), timeFormatter: (time: Time) => utcTime(new Date((time as UTCTimestamp) * 1000).toISOString()) + ' UTC' },
    handleScroll: { mouseWheel: true, pressedMouseMove: true, horzTouchDrag: true, vertTouchDrag: false },
    handleScale: { mouseWheel: true, pinch: true, axisPressedMouseMove: true },
  };
}
