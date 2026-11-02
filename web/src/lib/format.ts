import { locale, tr } from '../i18n/config.ts';
const decimal = (value: number, digits = 2) => new Intl.NumberFormat(locale(), { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(value);
export const number = (value: number | null, digits = 2) => value === null ? '—' : decimal(value, digits);
export const money = (value: number | null) => value === null ? '—' : decimal(value) + ' USDT';
export const percent = (value: number | null) => value === null ? '—' : decimal(value) + '%';
export const signed = (value: number | null, suffix = '%') => value === null ? '—' : (value > 0 ? '+' : value < 0 ? '−' : '') + decimal(Math.abs(value)) + tr(suffix);
export const tone = (value: number) => value > 0 ? 'metric-value-positive' : value < 0 ? 'metric-value-negative' : '';
export const strategyName = (type: string) => type === 'sma_crossover' ? tr('SMA Crossover') : type === 'buy_and_hold' ? tr('Buy & Hold') : type;
export const utcTime = (value: string) => new Intl.DateTimeFormat(locale() === 'ru-RU' ? 'ru-RU' : 'en-GB', { timeZone: 'UTC', year: 'numeric', month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(new Date(value));
