const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { setTimeout: delay } = require('node:timers/promises');
const root = path.resolve(__dirname, '..');
const catalogs = JSON.parse(fs.readFileSync(path.join(root, 'src/test/fixtures/catalogs.json')));
const example = JSON.parse(fs.readFileSync(path.join(root, '../docs/examples/btcusdt-sma-response.json')));
const candleFixture = JSON.parse(fs.readFileSync(path.join(root, 'src/test/fixtures/btcusdt-1h-2024-01-01-08.json')));
const expected = JSON.parse(fs.readFileSync(path.join(root, '../docs/examples/btcusdt-sma-request.json')));
const server = spawn(process.execPath, [path.join(root,'node_modules/vite/bin/vite.js'), '--port', '5183'], {
  cwd: root, env: {...process.env, VITE_API_BASE_URL:'https://tradelab.test'}, windowsHide:true, stdio:'pipe'
});
let browser;
const errors = [];
server.stderr.on('data', chunk => errors.push(String(chunk)));
async function ready() {
  for(let i=0;i<100;i++) {
    try { if((await fetch('http://127.0.0.1:5183')).ok) return; } catch {}
    await delay(100);
  }
  throw new Error('Test Vite server did not start: '+errors.join(''));
}
async function waitFor(check, label) {
  for(let i=0;i<80;i++){if(await check()) return;await delay(50);}
  throw new Error(label);
}
(async()=>{
  try {
    await ready();
    browser=await chromium.launch(process.platform === 'win32' ? {channel:'msedge'} : {});
    const page=await browser.newPage({locale:'en-US',viewport:{width:1440,height:1100}});
    const pageErrors=[];page.on('pageerror', error=>pageErrors.push(error.message));
    let postCount=0, marketsCount=0, strategiesCount=0, lastBody, catalogMode='ok', postMode='success';
    let releaseCatalog;const catalogGate=new Promise(resolve=>{releaseCatalog=resolve;});
    let releasePost;let postGate;let candleCount=0;
    let candleMode='ok', releaseCandles, candleGate=null;
    // A structural mock variant tests a second period; it is not a second captured backtest.
    const cropStart='2024-01-04T00:00:00Z';
    const cropped={...example,market:{...example.market,start:cropStart},
      trades:example.trades.filter(t=>t.time>=cropStart),
      equity_curve:example.equity_curve.filter(p=>p.time>=cropStart),
      indicators:Object.fromEntries(Object.entries(example.indicators).map(([key,points])=>[key,points.filter(p=>p.time>=cropStart).map(p=>({...p,candle_index:p.candle_index-72}))]))};
    let returnedMarket=example.market;
    const fulfill=(route, body, status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)});
    await page.route('https://tradelab.test/**',async route=>{
      const url=new URL(route.request().url());
      if(url.pathname.endsWith('/markets')){
        marketsCount++;await catalogGate;
        return catalogMode==='error'?fulfill(route,{},503):fulfill(route,catalogs.markets);
      }
      if(url.pathname.endsWith('/strategies')){
        strategiesCount++;await catalogGate;return fulfill(route,catalogs.strategies);
      }
      if(url.pathname.endsWith('/candles')){
        candleCount++;
        assert.deepEqual(Object.fromEntries(url.searchParams),{symbol:returnedMarket.symbol,interval:'1h',start:returnedMarket.start,end:returnedMarket.end});
        const selectedMarket={...returnedMarket};
        if(candleGate)await candleGate;
        if(candleMode==='error')return fulfill(route,{},502);
        return fulfill(route,{market:selectedMarket,candles:candleFixture.candles.filter(c=>c.open_time>=selectedMarket.start&&c.open_time<selectedMarket.end)});
      }
      if(url.pathname.endsWith('/backtests')){
        postCount++;lastBody=route.request().postDataJSON();
        if(postGate) await postGate;
        if(postMode==='cropped'){returnedMarket=cropped.market;return fulfill(route,cropped);}
        if(postMode==='new-symbol'){returnedMarket={...example.market,symbol:lastBody.symbol};return fulfill(route,{...example,market:returnedMarket,strategy:lastBody.strategy});}
        returnedMarket=example.market;
        if(postMode==='error')return fulfill(route,{},502);
        if(postMode==='invalid')return fulfill(route,{unexpected:true});
        if(postMode==='empty')return fulfill(route,{...example,metrics:{...example.metrics,completed_trades:0,win_rate:null,profit_factor:null},trades:[]});
        if(postMode==='hold')return fulfill(route,{...example,strategy:{type:'buy_and_hold'}});
        return fulfill(route,example);
      }
      throw new Error('Unexpected API request: '+url.pathname);
    });
    const russian=await browser.newPage({locale:'ru-RU'});
    await russian.route('https://tradelab.test/**',route=>fulfill(route,route.request().url().endsWith('/markets')?catalogs.markets:catalogs.strategies));
    await russian.goto('http://127.0.0.1:5183');
    await russian.getByRole('tab',{name:'Обзор',exact:true}).waitFor();
    assert.equal(await russian.locator('html').getAttribute('lang'),'ru');
    await russian.getByRole('button',{name:'EN',exact:true}).click();
    await russian.getByRole('tab',{name:'Overview',exact:true}).waitFor();
    assert.equal(await russian.evaluate(()=>localStorage.getItem('tradelab.language')),'en');
    await russian.reload();await russian.getByRole('tab',{name:'Overview',exact:true}).waitFor();
    await russian.close();
    await page.goto('http://127.0.0.1:5183');releaseCatalog();
    const run=page.getByRole('button',{name:'Run Backtest',exact:true});
    const tab=name=>page.getByRole('tab',{name,exact:true});
    await run.waitFor();
    assert.equal(marketsCount,1);assert.equal(strategiesCount,1);
    assert.equal(await tab('Overview').getAttribute('aria-selected'),'true');
    await page.getByLabel('Start date').fill('2024-01-01');await page.getByLabel('End date').fill('2024-01-08');
    postGate=new Promise(resolve=>{releasePost=resolve;});
    await page.locator('form').evaluate(form=>{form.requestSubmit();form.requestSubmit();});
    await page.getByRole('button',{name:'Running Backtest...',exact:true}).waitFor();
    await waitFor(()=>postCount===1,'POST not received');assert.deepEqual(lastBody,expected);
    releasePost();postGate=null;
    await page.getByRole('button',{name:'Reset equity view'}).waitFor();
    assert.ok((await page.locator('.overview-metrics').innerText()).includes('10,329.08 USDT'));
    await page.getByRole('button',{name:'About Profit factor',exact:true}).click();
    assert.equal(await page.getByRole('button',{name:'About Profit factor',exact:true}).getAttribute('aria-expanded'),'true');
    await page.getByRole('button',{name:'About Profit factor',exact:true}).click();
    await page.getByRole('button',{name:'About Slippage (%)',exact:true}).click();
    assert.ok(await page.getByText(/Models a worse execution price/).isVisible());
    await page.getByRole('button',{name:'About Slippage (%)',exact:true}).click();
    await page.getByLabel('Initial Capital (USDT)',{exact:true}).fill('10001');
    await page.getByText('Settings changed — run a new backtest to update these results',{exact:true}).waitFor();
    assert.equal(postCount,1);
    await page.getByLabel('Initial Capital (USDT)',{exact:true}).fill('10000');
    assert.equal(await page.getByText('Settings changed — run a new backtest to update these results',{exact:true}).count(),0);
    await page.getByRole('button',{name:'Collapse settings',exact:true}).click();
    assert.equal(await page.getByLabel('Trading pair').isVisible(),false);
    await page.screenshot({path:path.join(process.env.TEMP||root,'tradelab-overview.png'),fullPage:true});
    await tab('Charts').click();const resetMarket=page.getByRole('button',{name:'Reset market view'});await resetMarket.waitFor();
    await page.locator('.market-canvas canvas').first().evaluate(el=>el.dataset.languageCheck='same');
    const beforeLanguage={post:postCount,candles:candleCount};
    await page.getByLabel('Trades',{exact:true}).uncheck();
    await page.getByRole('button',{name:'RU',exact:true}).click();
    await page.getByRole('tab',{name:'График',exact:true}).waitFor();
    assert.equal(await page.getByLabel('Сделки',{exact:true}).isChecked(),false);
    assert.equal(await page.locator('.market-canvas canvas').first().getAttribute('data-language-check'),'same');
    await page.getByRole('tab',{name:'Обзор',exact:true}).click();
    assert.ok((await page.locator('.primary-metrics').innerText()).replace(/[\u00a0\u202f]/g,' ').includes('10 329,08 USDT'));
    await page.screenshot({path:path.join(process.env.TEMP||root,'tradelab-overview-ru.png'),fullPage:true});
    await page.setViewportSize({width:375,height:1000});
    for(const name of ['Обзор','График','Сделки']){
      await page.getByRole('tab',{name,exact:true}).click();
      assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'Russian layout overflow: '+name);
    }
    await page.setViewportSize({width:1440,height:1100});
    await page.getByRole('button',{name:'EN',exact:true}).click();
    await tab('Charts').click();await resetMarket.waitFor();
    await page.getByLabel('Trades',{exact:true}).check();
    assert.deepEqual({post:postCount,candles:candleCount},beforeLanguage,'Language must not trigger API requests');
    const counts={post:postCount,candles:candleCount};
    assert.equal(await page.locator('.market-canvas').evaluate(el=>el.clientHeight),600);
    assert.equal(await page.getByRole('button',{name:'Reset equity view'}).count(),0);
    await page.getByLabel('SMA',{exact:true}).uncheck();await page.getByLabel('SMA',{exact:true}).check();
    await page.getByLabel('Trades',{exact:true}).uncheck();await page.getByLabel('Trades',{exact:true}).check();
    await page.screenshot({path:path.join(process.env.TEMP||root,'tradelab-charts.png'),fullPage:true});
    await page.getByLabel('SMA',{exact:true}).uncheck();
    await page.getByLabel('Trades',{exact:true}).uncheck();
    await tab('Trades').click();assert.equal(await page.locator('canvas').count(),0);
    assert.equal(await page.locator('.trade-table tbody tr').count(),9);
    await page.getByLabel('Side',{exact:true}).selectOption('buy');
    assert.equal(await page.locator('.trade-table tbody tr').count(),5);
    await page.getByLabel('Side',{exact:true}).selectOption('sell');
    assert.equal(await page.locator('.trade-table tbody tr').count(),4);
    await page.getByLabel('Side',{exact:true}).selectOption('all');
    await page.getByRole('button',{name:'Price',exact:true}).click();
    assert.equal(await page.locator('th[aria-sort="ascending"] button').innerText(),'Price ↑');
    await page.getByRole('button',{name:'Price ↑',exact:true}).click();
    assert.equal(await page.locator('th[aria-sort="descending"] button').innerText(),'Price ↓');
    await page.screenshot({path:path.join(process.env.TEMP||root,'tradelab-trades.png'),fullPage:true});
    await page.getByLabel('Side',{exact:true}).selectOption('sell');
    await tab('Charts').click();await resetMarket.waitFor();
    assert.equal(await page.getByLabel('SMA',{exact:true}).isChecked(),false);
    assert.equal(await page.getByLabel('Trades',{exact:true}).isChecked(),false);
    await tab('Trades').click();
    assert.equal(await page.getByLabel('Side',{exact:true}).inputValue(),'sell');
    assert.equal(await page.locator('th[aria-sort="descending"] button').innerText(),'Price ↓');
    assert.equal(await page.locator('.trade-table tbody tr').count(),4);
    await page.getByLabel('Side',{exact:true}).selectOption('all');
    for(const name of ['Overview','Charts','Trades','Overview','Charts']){await tab(name).click();if(name==='Charts')await resetMarket.waitFor();}
    assert.deepEqual({post:postCount,candles:candleCount},counts,'Tabs must not refetch or recalculate');
    await page.getByRole('button',{name:'Edit settings',exact:true}).click();
    assert.equal(await page.getByLabel('Start date').inputValue(),'2024-01-01');
    for(const width of [320,375,768,1024,1200,1440]){
      await page.setViewportSize({width,height:1000});
      for(const name of ['Overview','Charts','Trades']){await tab(name).click();assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'Overflow '+name+' '+width);}
    }
    await tab('Overview').focus();await page.keyboard.press('ArrowRight');assert.equal(await tab('Charts').getAttribute('aria-selected'),'true');
    await page.getByLabel('Strategy type').selectOption('buy_and_hold');postMode='hold';await run.click();await resetMarket.waitFor();
    assert.deepEqual(lastBody.strategy,{type:'buy_and_hold'});assert.equal(await page.getByLabel('SMA',{exact:true}).count(),0);
    await tab('Overview').click();await page.getByText('Buy & Hold selected',{exact:true}).waitFor();
    postMode='error';await run.click();await page.getByRole('alert').waitFor();assert.equal(await page.locator('canvas').count(),0);
    postMode='success';await run.click();await page.getByRole('button',{name:'Reset equity view'}).waitFor();
    await tab('Charts').click();await resetMarket.waitFor();assert.equal(candleCount,counts.candles);
    assert.equal(await page.getByLabel('SMA',{exact:true}).count(),1);
    await tab('Trades').click();assert.equal(await page.locator('.trade-table tbody tr').count(),9);
    await page.getByLabel('Search pair',{exact:true}).fill('dogecoin');
    assert.equal(await page.getByLabel('Trading pair',{exact:true}).locator('option').count(),2);
    const beforePair=postCount;
    await page.getByLabel('Trading pair',{exact:true}).selectOption('DOGEUSDT');
    await page.getByLabel('Search pair',{exact:true}).press('Enter');
    assert.equal(postCount,beforePair);
    assert.ok((await page.locator('.run-context').innerText()).includes('BTC/USDT'));
    postMode='new-symbol';await run.click();
    await waitFor(()=>lastBody.symbol==='DOGEUSDT','New symbol was not submitted');
    await page.getByText('DOGE/USDT',{exact:true}).waitFor();
    assert.equal(await page.getByLabel('Trading pair',{exact:true}).inputValue(),'DOGEUSDT');
    await page.getByRole('button',{name:'RU',exact:true}).click();
    assert.equal(await page.getByLabel('Торговая пара',{exact:true}).inputValue(),'DOGEUSDT');
    await page.getByLabel('Начальный капитал (USDT)',{exact:true}).fill('0');
    await page.getByText('Начальный капитал должен быть больше нуля.',{exact:true}).first().waitFor();
    await page.getByLabel('Начальный капитал (USDT)',{exact:true}).fill('10000');
    await page.getByRole('button',{name:'EN',exact:true}).click();
    postGate=new Promise(resolve=>{releasePost=resolve;});await run.click();await page.getByRole('button',{name:'Cancel request'}).click();
    await page.getByText('Request cancelled. No new result was saved.',{exact:true}).waitFor();releasePost();postGate=null;
    assert.deepEqual(pageErrors,[]);
    console.log('PASS: three tabs, keyboard navigation, collapse and form persistence, shared result, no duplicate requests, chart toggles/cleanup, sorting/filtering persistence, contextual help, draft warning, responsive layout, RU/EN persistence, locale formatting and searchable API instruments.');
  } finally {
    if(browser) await browser.close();
    server.kill();
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
