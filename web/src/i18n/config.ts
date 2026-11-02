import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import ru from './ru.json' with { type: 'json' };
import en from './en.json' with { type: 'json' };

export const languageKey = 'tradelab.language';

export function detectLanguage(saved: string | null, browser: string): 'ru' | 'en' {
  if (saved === 'ru' || saved === 'en') return saved;
  return browser.toLowerCase().startsWith('ru') ? 'ru' : 'en';
}

let saved: string | null = null;
try {
  if (typeof localStorage !== 'undefined') saved = localStorage.getItem(languageKey);
} catch {
  // Storage may be disabled; language selection still works for this session.
}

void i18n.use(initReactI18next).init({
  resources: { ru: { translation: ru }, en: { translation: en } },
  lng: detectLanguage(saved, typeof window === 'undefined' ? 'en' : navigator.language),
  fallbackLng: 'en',
  keySeparator: false,
  nsSeparator: false,
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
});

function updateLanguage() {
  const lang = i18n.resolvedLanguage === 'ru' ? 'ru' : 'en';
  if (typeof document !== 'undefined') {
    document.documentElement.lang = lang;
    document.title = lang === 'ru'
      ? 'TradeLab — тестирование стратегий'
      : 'TradeLab — Strategy Backtesting';
  }
  try {
    if (typeof localStorage !== 'undefined') localStorage.setItem(languageKey, lang);
  } catch {
    // A blocked store must not prevent changing the interface language.
  }
}

i18n.on('languageChanged', updateLanguage);
updateLanguage();

export const locale = () => i18n.resolvedLanguage === 'ru' ? 'ru-RU' : 'en-US';

// Dynamic validation messages contain counts and technical values. Translate their
// known fragments longest-first, preserving those values and unknown API text.
const fragments = Object.entries(ru)
  .filter(([source]) => source.length > 12 || source.endsWith(' '))
  .sort((a, b) => b[0].length - a[0].length);

export function tr(value: string): string {
  if (i18n.exists(value)) return String(i18n.t(value));
  if (i18n.resolvedLanguage !== 'ru') return value;
  let result = value;
  for (const [source, target] of fragments) result = result.split(source).join(target);
  return result;
}

export default i18n;
