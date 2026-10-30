# Docker и Render Free

Подготовка этапа 6. Ресурсы Render не создавались. Docker-сборка и локальный
контейнер проверены 29.10.2026; доступ Binance из Render ещё не подтверждён.
Приложение технически подходит для
синхронного Web Service; пригодность конкретного региона проверяется после
разрешённого деплоя.

## Windows: сначала диск D:

Первоначально Docker Desktop был остановлен, а VHDX находился на C:;
сборка была отложена. Пользователь вручную перенёс хранилище, после чего
подтверждены `CustomWslDistroDir=D:\докер хранилище\DockerDesktopWSL` и
активный `disk\docker_data.vhdx` в этой папке. Сборка выполнена из
`D:\go_projects\TradeLab`; VHDX на D: изменялся во время сборки и проверок.
Сам путь `/var/lib/docker` из `docker info` не доказывает размещение на D:.

Ручные действия владельца компьютера перед любой сборкой:

1. Откройте Docker Desktop → Settings → Resources → Advanced.
2. В **Disk image location** выберите новую пустую папку на D:,
   например `D:\DockerDesktopData`. Не выбирайте папку с чужим disk image:
   не подтверждайте его замену.
3. Примените изменение и дождитесь переноса/перезапуска. Не перемещайте VHDX
   через Проводник и не выполняйте `wsl --unregister`, reset или prune.
4. Снова откройте Settings: путь disk image должен начинаться с `D:\`.
   Проверьте существование VHDX в выбранной папке и свободное место D:.
   Если настройка отсутствует или перенос не удался — остановитесь, не собирайте.
5. Откройте рабочую копию `D:\go_projects\TradeLab`. Оригинал на C: оставлен
   для сохранности, не удалён. После перехода работайте и коммитьте из одной
   выбранной копии, чтобы они не разошлись.

Это соответствует [инструкции Docker для WSL2](https://docs.docker.com/desktop/features/wsl/).
Перенос disk image и любые изменения WSL выполняет пользователь вручную.
Даже после переноса небольшие настройки/логи Docker Desktop могут оставаться
на C:; цель переноса — большие образы, контейнерные слои и build cache.

Для Go в текущем PowerShell (без изменения глобальных каталогов Windows):

```powershell
Set-Location D:\go_projects\TradeLab
$env:PATH = "D:\TradeLabTools\go\bin;" + $env:PATH
$env:GOCACHE = "D:\GoCache"
$env:GOMODCACHE = "D:\GoModCache"
$env:GOTMPDIR = "D:\GoTemp"
$env:GOPATH = "D:\GoPath"
$env:TEMP = "D:\GoTemp"
$env:TMP = "D:\GoTemp"
$env:GOTOOLCHAIN = "local"
$env:GOENV = "off"
go env GOCACHE GOMODCACHE GOTMPDIR
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

Переменные Go не перемещают Docker storage. Хранить исходники на D: тоже
недостаточно: disk image необходимо проверить отдельно. Ничего не удаляется
автоматически, включая старые Docker-образы и кэш.

## Docker: только после подтверждения D:

Dockerfile использует `golang:1.27.3-bookworm` и `scratch`. Builder формирует
Linux amd64 бинарник с CGO=0; runtime содержит только бинарник и CA bundle
из builder для HTTPS. Пользователь — числовой UID/GID 65532, без root.
Exec-form ENTRYPOINT доставляет SIGTERM самому API. Shell/curl в runtime нет.
Тег Go фиксирует patch-версию; это не immutable digest. Для строгой фиксации
можно передать проверенный digest через `--build-arg GO_IMAGE=golang@sha256:...`.

В PowerShell из копии D:, **после ручной проверки disk image location**:

```powershell
if ((Get-Location).Drive.Name -ne 'D') { throw 'Сборка разрешена только из D:' }
docker context show
docker info --format 'OS={{.OSType}} Arch={{.Architecture}} Root={{.DockerRootDir}}'
docker build --platform linux/amd64 -t tradelab-api .
docker image inspect tradelab-api --format '{{.Os}}/{{.Architecture}} user={{.Config.User}} size={{.Size}}'
docker run --rm --name tradelab-check --stop-timeout 15 -p 18080:8080 -e PORT=8080 tradelab-api
```

Другой терминал, тот же checkout на D::

```powershell
docker inspect tradelab-check --format 'running={{.State.Running}} image={{.Config.Image}} user={{.Config.User}}'
docker port tradelab-check
$Api = 'http://localhost:18080'
curl.exe -i "$Api/healthz"
curl.exe -i "$Api/api/v1/markets"
curl.exe -i "$Api/api/v1/strategies"
curl.exe -i --max-time 35 "$Api/api/v1/candles?symbol=BTCUSDT&interval=1h&start=2024-01-01T00:00:00Z&end=2024-01-01T03:00:00Z"
curl.exe -i --max-time 35 -X POST "$Api/api/v1/backtests" -H 'Content-Type: application/json' --data-binary '@docs/examples/btcusdt-sma-request.json'
docker logs tradelab-check
docker stop --timeout 15 tradelab-check
```

Порт 18080 выделен контейнеру, а не локальному `go run`. Это вместе с
`docker inspect` подтверждает, что обработчик и обращения Binance работают
в контейнере. Последняя команда отправляет SIGTERM; в логах ожидаются
`shutting down API` и `API stopped`. `--rm` удаляет только этот новый
проверочный контейнер после завершения, не существующие контейнеры/образы.
Для альтернативного порта проверьте `-e PORT=10000 -p 18080:10000`.
Для будущих изменений Dockerfile cross-compilation не заменяет контейнерные
проверки: размер, платформу, CA/HTTPS и SIGTERM проверяйте на собранном образе.

### Результат локальной проверки 29.10.2026

- `docker build --platform linux/amd64 -t tradelab-api .` — успешно.
- `docker image inspect`: `linux/amd64`, `User=65532:65532`, ENTRYPOINT `/api`.
- Контейнер публиковал `127.0.0.1:18080 → 8080`; проверено через inspect.
- `/healthz`, `/api/v1/markets`, `/api/v1/strategies` — HTTP 200.
- `/api/v1/candles` — HTTP 200, 3 настоящие BTCUSDT-свечи, `source=binance`.
- `/api/v1/backtests` — HTTP 200, 168 equity-точек, 9 операций;
  `total_return=3.2907927505975376`, совпадает с сохранённым локальным примером.
- Успешные HTTPS-запросы Binance выполнены обработчиком внутри контейнера,
  поэтому работа CA bundle также подтверждена этим запуском.
- При `docker stop` получены `shutting down API`, `API stopped`, ExitCode=0,
  OOMKilled=false.
- Отдельно проверен `PORT=10000` с отображением `18081 → 10000`: healthcheck 200.

Оба созданных проверочных контейнера остановлены и сохранены. Образ и build
cache не удалялись; существующие контейнеры и образы не затрагивались очисткой.
Проверка Render, его холодного старта и сетевых ограничений остаётся отдельной.

## Демонстрация без Binance

```powershell
go run ./cmd/marketcheck -fixture -backtest
```

CLI использует существующие шесть синтетических свечей и FixtureProvider,
запускает Buy & Hold через прежний движок с обычной комиссией. Вывод содержит
предупреждение о синтетических данных, `source: fixture` и имя сценария.
Это локальная демонстрация без сети, не подмена котировок Binance. Флаг
`-backtest` без `-fixture` отклоняется. Новых REST-endpoints и глобального
demo-переключателя в публичном сервере нет. В минимальный runtime CLI не входит.

## Ручной Render Free deploy — после отдельного разрешения

Проверено по официальным документам 29.10.2026:
[Docker](https://render.com/docs/docker),
[Web Services / PORT](https://render.com/docs/web-services),
[Free limits](https://render.com/docs/free),
[Blueprint](https://render.com/docs/blueprint-spec).

1. После отдельного согласования commit/push откройте Render Dashboard →
   New → Web Service и выберите GitHub-репозиторий TradeLab и нужную ветку.
2. Language/Runtime — **Docker**, Dockerfile Path — `./Dockerfile`, контекст —
   корень репозитория. Команду запуска не переопределяйте: используется `/api`.
3. Выберите **Free**, не создавая баз, дисков и workers. Health Check Path —
   `/healthz`. Auto-Deploy отключите для ручного контроля.
4. `PORT` берётся из окружения Render (обычно 10000); код слушает все интерфейсы.
   `CORS_ALLOWED_ORIGINS` оставьте пустым для curl или задайте точный HTTPS
   frontend-origin без завершающего `/`. API-ключи Binance не нужны.
5. Проверьте план/лимиты workspace до нажатия Create Web Service: кнопка
   создаёт ресурс и запускает первый deploy. Здесь это действие не выполнялось.
6. После успешной сборки используйте выданный публичный HTTPS `onrender.com`
   URL. TLS завершается у Render; внутри контейнера API работает по HTTP.

`render.yaml` описывает только один Docker Web Service на Free, healthcheck
и отключённые auto-deploys. Он сам ничего не разворачивает. Создание Blueprint
тоже требует отдельного разрешения; первый deploy не отменяется настройкой
`autoDeployTrigger: off`.

Free Web Service засыпает после 15 минут без входящего трафика; пробуждение
обычно занимает около минуты. Локальная файловая система эфемерная. У workspace
750 free instance hours в месяц, также действуют лимиты bandwidth/build minutes.
При превышении некоторые лимиты могут приводить к доплатам при наличии способа
оплаты либо приостановке сервиса/сборок. Проверяйте Billing и spend controls,
не подключайте платные ресурсы. Free не предназначен для production-нагрузки.

TradeLab не требует always-on: вычисления выполняются только по HTTP-запросу,
между запросами нет работы, которая должна продолжаться. Периодические пинги
для обхода сна не нужны. Высокий объём исходящих API-запросов также может
привести к приостановке Free-сервиса. Доступ Binance из конкретного региона,
реальные ограничения CPU/памяти и холодный старт ещё надо проверить.

## Проверка с будущего Render, не нагрузочный тест

В PowerShell из checkout проекта:

```powershell
$Api = 'https://YOUR-SERVICE.onrender.com'
curl.exe -i --max-time 120 "$Api/healthz"
curl.exe -i --max-time 35 "$Api/api/v1/markets"
curl.exe -i --max-time 35 "$Api/api/v1/strategies"
curl.exe -i --max-time 35 "$Api/api/v1/candles?symbol=BTCUSDT&interval=1h&start=2024-01-01T00:00:00Z&end=2024-01-01T03:00:00Z"
# 1104 свечи: больше одной Binance-страницы, но меньше лимита 90 дней.
curl.exe -i --max-time 35 "$Api/api/v1/candles?symbol=BTCUSDT&interval=1h&start=2024-01-01T00:00:00Z&end=2024-02-16T00:00:00Z"
$timer = [System.Diagnostics.Stopwatch]::StartNew()
$result = Invoke-RestMethod -Method Post -Uri "$Api/api/v1/backtests" -ContentType 'application/json' -Body (Get-Content docs/examples/btcusdt-sma-request.json -Raw) -TimeoutSec 35
$timer.Stop()
$result.metrics
$result.benchmark.metrics
"Elapsed: $($timer.Elapsed.TotalSeconds) seconds"
$local = Get-Content docs/examples/btcusdt-sma-response.json -Raw | ConvertFrom-Json
[math]::Abs($result.metrics.total_return - $local.metrics.total_return) -lt 1e-8
```

Сначала дождитесь JSON `{"status":"ok"}`, не принимайте HTML страницы
пробуждения за результат API. Затем выполните каждую проверку один раз.
Для сохранённого недельного сценария ожидаются 168 equity-точек, 9 операций,
доходность SMA около 3,290793%, Buy & Hold около 3,319358%.
Сравните также времена/типы операций и комиссии с сохранённым локальным ответом.

Проверьте Render logs: безопасные предупреждения содержат upstream HTTP-статус
или признак таймаута, но не upstream body, URL с credentials или тело запроса.
Binance 403/451/418 отображается как 503, 429 сохраняется с Retry-After,
невалидная история — 502, таймаут — 504. Не провоцируйте бан для тестирования:
эти ошибки покрыты локальными HTTP-тестами. При 429 соблюдайте Retry-After;
при недоступности остановитесь, не повторяйте запросы бесконечно.

Оставьте сервис без пользовательского трафика более 15 минут и повторите
один healthcheck с `--max-time 120`, затем один короткий запрос свечей.
Зафиксируйте время холодного старта отдельно от времени расчёта. Нагрузочные
тесты, настоящий deploy и проверка сна/доступа Binance с Render не выполнялись.
