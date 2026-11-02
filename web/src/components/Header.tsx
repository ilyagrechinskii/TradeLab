import { useTranslation } from 'react-i18next';
import i18n from '../i18n/config';
import { tr } from '../i18n/config';
import { ChartNoAxesColumnIncreasing, GitFork } from 'lucide-react';

export default function Header() {
  useTranslation();
  return (
    <header className="header">
      <a className="brand" href="#main" aria-label={tr("TradeLab workspace")}>
        <span className="brand-mark"><ChartNoAxesColumnIncreasing size={22} aria-hidden="true" /></span>
        <span className="brand-name">Trade<span>Lab</span></span>
      </a>
      <span className="brand-caption">{tr("Crypto Strategy Backtesting")}</span>
      <div className="header-actions">
        <div className="language-switch" role="group" aria-label={tr('Interface language')}>{(['ru','en'] as const).map(language=><button type="button" key={language} lang={language} aria-pressed={i18n.resolvedLanguage===language} onClick={()=>void i18n.changeLanguage(language)}>{language.toUpperCase()}</button>)}</div>
        <span className="mode"><span className="status-dot" />{tr("Historical simulation")}</span>
        <a className="github-link" aria-label={tr("GitHub repository (opens in a new tab)")} href="https://github.com/ilyagrechinskii/TradeLab" target="_blank" rel="noreferrer">
          <GitFork size={16} strokeWidth={1.5} aria-hidden="true" /><span>GitHub</span><span className="sr-only">{" "}{tr("(opens in a new tab)")}</span>
        </a>
      </div>
    </header>
  );
}
