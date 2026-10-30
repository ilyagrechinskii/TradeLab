const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { setTimeout: delay } = require('node:timers/promises');
const root = path.resolve(__dirname, '..');
const catalogs = JSON.parse(fs.readFileSync(path.join(root, 'src/test/fixtures/catalogs.json')));
const example = JSON.parse(fs.readFileSync(path.join(root, '../docs/examples/btcusdt-sma-response.json')));
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
    const page=await browser.newPage({viewport:{width:1440,height:1100}});
    const pageErrors=[];page.on('pageerror', error=>pageErrors.push(error.message));
    let postCount=0, marketsCount=0, strategiesCount=0, lastBody, catalogMode='ok', postMode='success';
    let releaseCatalog;const catalogGate=new Promise(resolve=>{releaseCatalog=resolve;});
    let releasePost;let postGate;
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
      if(url.pathname.endsWith('/backtests')){
        postCount++;lastBody=route.request().postDataJSON();
        if(postGate) await postGate;
        if(postMode==='error')return fulfill(route,{},502);
        if(postMode==='invalid')return fulfill(route,{unexpected:true});
        if(postMode==='empty')return fulfill(route,{...example,metrics:{...example.metrics,completed_trades:0,win_rate:null,profit_factor:null},trades:[]});
        if(postMode==='hold')return fulfill(route,{...example,strategy:{type:'buy_and_hold'}});
        return fulfill(route,example);
      }
      throw new Error('Unexpected API request: '+url.pathname);
    });
    await page.goto('http://127.0.0.1:5183');
    await page.getByText('Loading markets & strategies...',{exact:true}).waitFor();
    assert.equal(await page.getByRole('button',{name:'Run Backtest',exact:true}).count(),0);
    releaseCatalog();
    const run=page.getByRole('button',{name:'Run Backtest',exact:true});
    await run.waitFor();
    assert.equal(marketsCount,1,'StrictMode must not duplicate markets');
    assert.equal(strategiesCount,1,'StrictMode must not duplicate strategies');
    assert.equal(await page.getByLabel('Trading pair').locator('option').count(),catalogs.markets.symbols.length);
    await page.getByLabel('Start date').fill('2024-01-01');
    await page.getByLabel('End date').fill('2024-01-08');
    postGate=new Promise(resolve=>{releasePost=resolve;});
    await page.locator('form').evaluate(form=>{form.requestSubmit();form.requestSubmit();});
    await page.getByRole('button',{name:'Running Backtest...',exact:true}).waitFor();
    assert.equal(await page.getByRole('button',{name:'Running Backtest...',exact:true}).isDisabled(),true);
    await waitFor(()=>postCount===1,'POST not received');
    assert.equal(postCount,1,'Repeated submits must be ignored');
    assert.deepEqual(lastBody,expected);
    releasePost();postGate=null;
    await page.getByText('+3.29%',{exact:true}).first().waitFor();
    assert.equal(await page.locator('.trade-table tbody tr').count(),9);
    assert.ok((await page.locator('.metrics-grid').innerText()).includes('10,329.08 USDT'));
    assert.ok((await page.locator('.metrics-grid').innerText()).includes('25.00%'));
    assert.ok((await page.locator('.benchmark-values').innerText()).includes('+3.32%'));
    assert.ok((await page.locator('.benchmark-difference').innerText()).includes('−0.03 pp'));
    assert.ok((await page.locator('.result-context').innerText()).includes('92.63 USDT'));
    assert.equal(await page.locator('canvas').count(),0);
    await page.screenshot({path:path.join(process.env.TEMP || root,'tradelab-api-results.png'),fullPage:true});
    for(const width of [320,375,768,1024,1200,1440]){
      await page.setViewportSize({width,height:1000});
      assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'Overflow at '+width);
    }
    await page.getByLabel('Trading pair').selectOption('ETHUSDT');
    assert.ok((await page.locator('.result-context').innerText()).includes('BTCUSDT'),'Editing must not relabel the old result');
    await page.getByLabel('Trading pair').selectOption('BTCUSDT');
    postMode='error';await run.click();
    await page.getByRole('alert').waitFor();
    assert.equal(await page.getByText('+3.29%',{exact:true}).count(),0,'Old results must be cleared');
    postMode='invalid';await run.click();
    await page.getByText(/unexpected response/).waitFor();
    postMode='empty';await run.click();
    await page.getByText('No trades were executed during this backtest.',{exact:true}).waitFor();
    assert.ok((await page.locator('.metrics-grid').innerText()).includes('—'));
    postMode='hold';
    await page.getByLabel('Strategy type').selectOption('buy_and_hold');
    assert.equal(await page.getByLabel('Fast Period',{exact:true}).count(),0);
    await run.click();
    await page.getByText('Buy & Hold selected',{exact:true}).waitFor();
    assert.deepEqual(lastBody.strategy,{type:'buy_and_hold'});
    postMode='success';
    postGate=new Promise(resolve=>{releasePost=resolve;});
    await run.click();
    await page.getByRole('button',{name:'Cancel request'}).click();
    await page.getByText('Request cancelled. No new result was saved.',{exact:true}).waitFor();
    releasePost();postGate=null;
    assert.equal(await page.getByText('+3.29%',{exact:true}).count(),0);
    await page.waitForTimeout(100);
    catalogMode='error';await page.reload();
    await page.getByRole('button',{name:'Retry catalogs'}).waitFor();
    catalogMode='ok';await page.getByRole('button',{name:'Retry catalogs'}).click();
    await run.waitFor();
    assert.deepEqual(pageErrors,[]);
    console.log('PASS: catalog loading/retry, one POST, running/disabled, cancellation, exact JSON, saved Go response, nulls, errors, trades, benchmark, responsive layout.');
  } finally {
    if(browser) await browser.close();
    server.kill();
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
