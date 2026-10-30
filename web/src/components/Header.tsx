import { ChartNoAxesColumnIncreasing, GitFork } from 'lucide-react';

export default function Header() {
  return (
    <header className="header">
      <a className="brand" href="#main" aria-label="TradeLab workspace">
        <span className="brand-mark"><ChartNoAxesColumnIncreasing size={22} aria-hidden="true" /></span>
        <span className="brand-name">Trade<span>Lab</span></span>
      </a>
      <span className="brand-caption">Crypto Strategy Backtesting</span>
      <div className="header-actions">
        <span className="mode"><span className="status-dot" />Historical Simulation</span>
        <a className="github-link" aria-label="GitHub repository (opens in a new tab)" href="https://github.com/ilyagrechinskii/TradeLab" target="_blank" rel="noreferrer">
          <GitFork size={16} aria-hidden="true" /><span>GitHub</span><span className="sr-only"> (opens in a new tab)</span>
        </a>
      </div>
    </header>
  );
}
