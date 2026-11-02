import assert from 'node:assert/strict';
import { test } from 'node:test';
import i18n, { detectLanguage, locale, tr } from '../i18n/config.ts';
import { money, signed, utcTime } from './format.ts';

test('language detection respects a saved RU/EN choice and falls back to browser',()=>{
 assert.equal(detectLanguage(null,'ru-RU'),'ru');
 assert.equal(detectLanguage(null,'en-US'),'en');
 assert.equal(detectLanguage(null,'de-DE'),'en');
 assert.equal(detectLanguage('en','ru'),'en');
 assert.equal(detectLanguage('ru','en'),'ru');
 assert.equal(detectLanguage('invalid','ru'),'ru');
});
test('central dictionaries translate errors, nulls and financial/date formatting',async()=>{
 await i18n.changeLanguage('ru');
 assert.equal(locale(),'ru-RU');
 assert.equal(tr('Charts'),'График');
 assert.equal(tr('Initial capital must be greater than 0.'),'Начальный капитал должен быть больше нуля.');
 assert.equal(money(10329.08).replace(/[\u00a0\u202f]/g,' '),'10 329,08 USDT');
 assert.equal(signed(3.29),'+3,29%');
 assert.equal(money(null),'—');
 assert.match(utcTime('2024-01-01T11:00:00Z'),/11:00/);
 assert.match(tr('Enter a positive whole number up to 2998.'),/2998/);
 await i18n.changeLanguage('en');
 assert.equal(money(10329.08),'10,329.08 USDT');
});
