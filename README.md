# TradeLab

TradeLab — пет-проект для GitHub-портфолио: веб-сервис для тестирования
криптовалютных торговых стратегий на исторических данных (backtesting).
Основа проекта — собственный движок на Go. Финансовые и статистические
вычисления выполняются на backend.

## Технологии и статус

Реализованы этапы 1–3: HTTP-сервер на стандартном `net/http`, конфигурация
порта, JSON-логи через `log/slog`, graceful shutdown и независимое расчётное
ядро бэктестинга с unit-тестами. Ядро принимает готовые сигналы и выполняет
симуляцию Spot-торговли по историческим OHLCV-свечам. Добавлены стратегии
SMA Crossover и Buy & Hold, показатели эффективности и сравнение стратегий.
Внешних Go-зависимостей нет. Архитектура — модульный монолит; расчёты
отделены от HTTP и не зависят от биржи, БД и frontend.

В следующих этапах планируются Binance Public Market Data API, PostgreSQL
через `pgx`, React + TypeScript и Docker. Целевые площадки — Render Free
для API и Cloudflare Pages для frontend; платные сервисы не требуются.
Эти интеграции пока не реализованы. Постоянные фоновые процессы не нужны.

## Структура

```text
cmd/api/main.go             — запуск сервера и обработка сигналов
internal/backtest/          — модели, исполнение сигналов, портфель и результаты
internal/strategy/          — SMA, стратегии и запуск через существующий движок
internal/config/            — загрузка и проверка PORT
internal/httpapi/           — HTTP-маршруты и тесты
```

## Локальный запуск

Требуется Go 1.27.3 или новее (стабильный выпуск). База данных, ключи API
и дополнительные сервисы для этого этапа не нужны.

Из корня репозитория:

```sh
go run ./cmd/api
```

По умолчанию сервер слушает `:8080` на всех сетевых интерфейсах.
Переменная окружения `PORT` задаёт порт от 1 до 65535. Если она отсутствует
или пуста, используется `8080`; некорректное значение останавливает запуск.

Пример в PowerShell:

```powershell
$env:PORT = "9000"
go run ./cmd/api
```

Пример в Linux/macOS:

```sh
PORT=9000 go run ./cmd/api
```

`.env` автоматически не загружается. Для единственной необязательной настройки
отдельный `.env.example` не требуется.

Для остановки нажмите Ctrl+C. Сервер обрабатывает SIGINT/SIGTERM и даёт
активным запросам до 10 секунд на завершение, затем закрывает соединения.
Таймаут чтения заголовков — 5 секунд, чтения/записи — 10 секунд,
ожидания следующего запроса — 60 секунд.

## Проверка API

```sh
curl -i http://localhost:8080/healthz
```

В Windows PowerShell при наличии алиаса `curl` используйте `curl.exe`.

Ожидаемый ответ: HTTP 200, `Content-Type: application/json` и тело:

```json
{"status":"ok"}
```

Все методы, кроме GET (включая HEAD), для `/healthz` получают HTTP 405
с заголовком `Allow: GET`. Это проверка работоспособности HTTP-сервера;
внешние зависимости сейчас отсутствуют.

## Тесты и сборка

```sh
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
go build -o bin/api ./cmd/api
```

На Windows для сборки можно использовать `go build -o bin/api.exe ./cmd/api`.
HTTP-тесты используют `net/http/httptest` и не требуют запуска сервера.
Для `go test -race` требуется поддерживаемый C toolchain и `CGO_ENABLED=1`.

## Ядро бэктестинга

Пакет `github.com/ilyagrechinskii/TradeLab/internal/backtest` доступен коду
внутри этого репозитория согласно правилам Go для `internal`. Это отдельный
расчётный пакет в существующем Go-модуле, без собственного HTTP API.

Основные функции:

```go
func DefaultConfig() Config
func (cfg Config) Validate() error
func ValidateCandles(candles []Candle) error
func Run(candles []Candle, signals []Signal, cfg Config) (BacktestResult, error)
func CalculateMetrics(result BacktestResult) (Metrics, error)
```

Модели: `Candle`, `Signal`, `SignalType` (`Hold`, `Buy`, `Sell`), `Config`,
`Portfolio`, `Trade`, `EquityPoint`, `BacktestResult`. Результат содержит
капитал, итоговую equity, доходность в процентах, сумму комиссий, исполненные
операции, equity curve и конечные остатки денег и актива.

Пример вызова (внутри функции; нужны импорты `fmt`, `time` и пакета `backtest`):

```go
start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
candles := []backtest.Candle{
    {OpenTime: start, Open: 100, High: 100, Low: 100, Close: 100},
    {OpenTime: start.Add(time.Hour), Open: 100, High: 110, Low: 100, Close: 110},
    {OpenTime: start.Add(2*time.Hour), Open: 120, High: 120, Low: 120, Close: 120},
}
signals := []backtest.Signal{
    {CandleIndex: 0, Type: backtest.Buy},
    {CandleIndex: 1, Type: backtest.Sell},
}
cfg := backtest.DefaultConfig()
cfg.InitialCapital = 1010
cfg.FeePercent = 1 // 1%, а не доля 0.01.
result, err := backtest.Run(candles, signals, cfg)
if err != nil {
    panic(err)
}
fmt.Printf("Equity: %.2f, fees: %.2f, return: %.2f%%\n",
    result.FinalEquity, result.TotalFees, result.TotalReturn)
// Equity: 1188.00, fees: 22.00, return: 17.62%
```

Этот пример также проверяется командой `go test ./...` как `ExampleRun`.

Правила расчёта и ограничения:

- По умолчанию: капитал 10 000, комиссия 0,1%, проскальзывание 0%.
  Все суммы выражены в одной валюте котировки, например USDT.
  `Run` использует переданный конфиг без подстановки значений: нулевая комиссия
  допустима, нулевой начальный капитал — ошибка. Проценты конечные, от 0
  включительно до 100 исключительно.
- Сигнал после закрытия свечи N исполняется по `Open` свечи N+1.
  Проскальзывание увеличивает цену покупки и уменьшает цену продажи.
  Сигнал последней свечи не исполняется.
- Разрешены только одна Spot-позиция, покупка на весь денежный баланс
  и продажа всего актива. Повторный Buy и Sell без позиции игнорируются.
  Отсутствующий сигнал равнозначен Hold. Сигналы могут поступать в любом порядке;
  индексы должны быть допустимыми и уникальными, включая Hold.
- Комиссия списывается в валюте котировки. Количество при покупке рассчитывается
  как `Cash / (1 + FeePercent/100) / ExecutionPrice`. При погрешности округления
  оно уменьшается до доступного количества; возможен малый денежный остаток.
  `Trade.Notional` — стоимость актива без комиссии; денежный расход на Buy
  равен `Notional + Fee`, поступление от Sell — `Notional - Fee`.
- После каждой свечи: `Equity = Cash + AssetQuantity * Close`. Открытая позиция
  в конце остаётся открытой и оценивается без гипотетической комиссии за выход.
  `TotalReturn = (FinalEquity / InitialCapital - 1) * 100`.
- Время исполнения — открытие следующей свечи в UTC. В equity curve время —
  `OpenTime` оцениваемой свечи в UTC, а стоимость рассчитана на её закрытии.
  Это метка свечи: точное время закрытия невозможно вывести без её интервала.
- Требуется хотя бы одна корректная свеча. Временные метки строго возрастают,
  пропуски интервалов допускаются; исполнение происходит на следующей
  предоставленной свече. Входные срезы не изменяются. Некорректные данные,
  неизвестные/дублирующиеся сигналы и непредставимые результаты `float64`
  возвращают ошибку без частичного результата.
- Используется `float64`, без правил биржевого округления, lot size,
  минимальной стоимости ордера, ликвидности и частичных исполнений.
  Объём валидируется, но не ограничивает исполнение. Сигналы предоставляются
  вызывающим кодом: их генератор должен самостоятельно избегать будущих данных.
- Журнал хранит исполненные операции, а не пары «вход — выход».
  Метрики вычисляются отдельно через `CalculateMetrics`.

## Стратегии и показатели

Пакет `internal/strategy` зависит от моделей и публичного API `backtest`.
Обратной зависимости нет. Контракт стратегии содержит один метод:

```go
type Strategy interface {
    Generate(candles []backtest.Candle) ([]backtest.Signal, error)
}
```

Стратегии только анализируют свечи и создают сигналы. Исполнение, комиссии,
проскальзывание и состояние портфеля остаются в `backtest.Run`.

Публичный API пакета `strategy`:

```go
func DefaultSMACrossover() SMACrossover
func (s SMACrossover) Validate() error
func (s SMACrossover) Generate(candles []backtest.Candle) ([]backtest.Signal, error)
func (s SMACrossover) Analyze(candles []backtest.Candle) (CrossoverResult, error)
func (BuyAndHold) Generate(candles []backtest.Candle) ([]backtest.Signal, error)
func SMA(candles []backtest.Candle, period int) ([]SMAPoint, error)
func Run(candles []backtest.Candle, selected Strategy, cfg backtest.Config) (Report, error)
func CompareWithBuyAndHold(candles []backtest.Candle, selected Strategy, cfg backtest.Config) (Comparison, error)
```

`SMACrossover` задаёт `FastPeriod` и `SlowPeriod`, по умолчанию 10 и 30.
Нужно `0 < FastPeriod < SlowPeriod` и не менее `SlowPeriod + 1` свечей:
пересечение требует двух полных окон медленной SMA. Нулевые параметры
не заменяются значениями по умолчанию — используйте `DefaultSMACrossover()`.

Обе SMA считаются по `Close` скользящей суммой с компенсацией погрешности,
за O(n), без пересчёта каждого окна. `Analyze` возвращает сигналы и обе серии
для графика. Точки `SMAPoint` содержат индекс свечи, её `OpenTime` в UTC и
среднее; до завершения окна точек нет. Отдельная `SMA` требует хотя бы
`period` свечей. Переполнение суммы или неположительное вычисленное среднее
возвращает ошибку.

BUY возникает при переходе быстрой SMA снизу вверх относительно медленной,
SELL — сверху вниз. Первое доступное положение средних само по себе сигналом
не является. При равенстве сигнал не создаётся; сохраняется последняя строго
неравная сторона. Поэтому касание с возвратом на ту же сторону не даёт
повторный BUY/SELL, а переход через равенство на противоположную — даёт.
Если первая пара средних равна, выход из равенства создаёт первый сигнал.
Сигналы последней свечи сохраняются, но движок их не исполняет.

Buy & Hold требует минимум две свечи и генерирует ровно один BUY на индексе 0.
Покупка исполняется на `Open` свечи 1. Принудительной продажи в конце нет.

Прямой запуск SMA Crossover через прежний API движка (для подготовленных
свечей `candles`, с импортами `backtest` и `strategy` из `internal`):

```go
selected := strategy.DefaultSMACrossover()
cfg := backtest.DefaultConfig()
signals, err := selected.Generate(candles)
if err != nil {
    return err
}
result, err := backtest.Run(candles, signals, cfg)
if err != nil {
    return err
}
metrics, err := backtest.CalculateMetrics(result)
if err != nil {
    return err
}
fmt.Printf("Доходность: %.2f%%, максимальная просадка: %.2f%%\n",
    result.TotalReturn, metrics.MaxDrawdown)
```

`strategy.Run(candles, selected, cfg)` объединяет эти шаги и возвращает `Report`
с полями `Result` и `Metrics`. Для сравнения с контрольной стратегией:

```go
comparison, err := strategy.CompareWithBuyAndHold(candles, selected, cfg)
if err != nil {
    return err
}
fmt.Printf("SMA: %.2f%%, Buy & Hold: %.2f%%, разница: %.2f п.п.\n",
    comparison.Strategy.Result.TotalReturn,
    comparison.BuyAndHold.Result.TotalReturn,
    comparison.ExcessReturn)
```

Оба запуска используют весь переданный интервал и один `cfg`. Период прогрева
SMA включён в интервал сравнения: SMA в это время держит деньги, Buy & Hold
уже может владеть активом. `ExcessReturn` — разность доходностей в процентных
пунктах. Доходность Buy & Hold берётся из того же движка, а не из отношения
первого и последнего `Close`.

Полные проверяемые примеры с фиксированными свечами:
`internal/strategy/example_test.go`. На этих демонстрационных данных SMA даёт
100%, Buy & Hold −50%, разница — 150 п.п.; это проверка правил исполнения,
а не оценка реальной прибыльности стратегии.

`backtest.CalculateMetrics` принимает результат движка и возвращает:

- `TotalReturn` — доходность, уже рассчитанную движком.
- `MaxDrawdown` — максимальное процентное падение от предшествующего пика;
  начальный капитал включается как первый пик. Показатель неотрицательный.
- `CompletedTrades` — число завершённых пар Buy/Sell, а не число ордеров.
- `WinRate` (`*float64`) — долю прибыльных закрытых позиций в процентах.
- `ProfitFactor` (`*float64`) — сумму положительных PnL, делённую на модуль
  суммы отрицательных PnL.

PnL закрытой позиции: `(Sell.Notional - Sell.Fee) - (Buy.Notional + Buy.Fee)`.
Проскальзывание уже входит в исполненные цены и `Notional`. Незакрытая
позиция участвует в общей доходности и просадке через equity curve, но
не участвует в Win Rate и Profit Factor. Безубыточная позиция считается
завершённой, но не прибыльной и не убыточной.

| Ситуация | Win Rate | Profit Factor |
| --- | --- | --- |
| Нет завершённых позиций | `nil` | `nil` |
| Есть прибыль, нет убытков | Рассчитан | `nil` |
| Только безубыточные позиции | 0% | `nil` |
| Все позиции убыточные | 0% | 0 |
| Есть прибыль и убытки | Рассчитан | Рассчитан |

`nil` означает неопределённый показатель и сериализуется как JSON `null`.
NaN/Infinity не возвращаются. Метрики рассчитаны на формат текущего движка:
одна позиция и полное закрытие. Некорректная последовательность операций,
частичные продажи, невалидные числа и переполнение возвращают ошибку.
Сравнение SMA использует обычную точность `float64`, без настраиваемого
порога пересечения. Sharpe Ratio, Sortino Ratio и оптимизация параметров
не реализованы.

## Следующие этапы

1. Получение и валидация исторических свечей Binance.
2. HTTP API для запуска бэктестинга и получения результатов.
3. Хранение данных в PostgreSQL через `pgx`.
4. Интерфейс на React + TypeScript.
5. Контейнеризация и развёртывание на выбранных бесплатных площадках.
