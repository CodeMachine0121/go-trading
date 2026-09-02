# go-trading

以 **Go + Gin** 打造的交易服務後端 REST API。

目前提供兩件事：

1. **K 線（KCandle）的完整讀寫** —— 新增（同交易標的同起始時間即覆蓋）、依交易標的與
   時間區間查詢、以及單一 K 線的讀取、修改、刪除。一根 K 線固定涵蓋五分鐘。
2. **自訂指標計算** —— 你自己寫一段 Go 算式送進來，系統拿指定根數的 K 線餵它，
   把輸出統一收成一組「名稱 → 數字」。加新指標不用改程式。

## Tech Stack

| 層面 | 選型 |
| :--- | :--- |
| 語言 | Go 1.26.1 |
| Web 框架 | [Gin](https://github.com/gin-gonic/gin) |
| ORM | [GORM](https://gorm.io)（Code First，`AutoMigrate` 同步 schema） |
| 資料庫 | PostgreSQL |
| 數值處理 | [shopspring/decimal](https://github.com/shopspring/decimal)（價格與量值一律精確小數，禁用 float） |
| 設定 | 環境變數 + [godotenv](https://github.com/joho/godotenv)（`.env`） |
| 測試 | `testing` + [uber-go/mock](https://github.com/uber-go/mock)（gomock） |
| 指標算式 | [traefik/yaegi](https://github.com/traefik/yaegi)（內嵌 Go 直譯器，白名單控管） |
| 架構 | Clean / Onion Architecture |

## 快速開始

需要一個可連線的 PostgreSQL。

```bash
cp .env.example .env     # 填入你的 DB 連線資訊
go mod download
make migrate             # 建立 / 更新資料表（可重複執行）
make start               # 啟動於 SERVER_PORT（預設 8080）
```

`make migrate` 與 `make start` 是分開的：**server 啟動時不會動 schema**，
資料表變更一律由 migrate 指令明確套用。

確認服務活著：

```bash
curl localhost:8080/health
# {"status":"Healthy"}
```

## Commands

| 指令 | 用途 |
| :--- | :--- |
| `make start` | 啟動 server（`go run ./cmd/server`） |
| `make migrate` | 套用 schema 到資料庫（`go run ./cmd/migrate`） |
| `make build` | 編譯到 `bin/server` 與 `bin/migrate` |
| `make test` | 跑全部測試（`go test ./...`） |
| `make test-storage` | 連同資料庫測試一起跑（需要一個可用的 PostgreSQL） |
| `make mock` | 重新產生所有 mock（`go generate ./...`） |
| `go vet ./...` | 靜態檢查 |

`mockgen` 已用 Go 1.24+ 的 `tool` 指示詞釘在 `go.mod`，**不需要另外全域安裝**。

## 環境變數

讀取於 `cmd/server/config.go`，**全部都有預設值**，`.env` 可整份省略。

| 變數 | 預設值 | 用途 |
| :--- | :--- | :--- |
| `SERVER_PORT` | `8080` | HTTP 服務埠號 |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | 允許讀取本 API 回應的前端來源，逗號分隔；清單外的來源不會拿到授權標頭 |
| `KCANDLE_QUERY_MAX_RESULTS` | `1000` | 單次區間查詢最多回傳幾根 K 線；超過即拒絕。指標計算的最大根數也用這個值 |
| `INDICATOR_SCRIPT_TIMEOUT_SECONDS` | `40` | 一段指標算式最多能跑幾秒；超過即中止 |
| `BACKGROUND_JOBS_ENABLED` | `true` | 背景工作總開關；`false` 時完全不回補、不自動抓取 |
| `KCANDLE_INGESTION_SYMBOLS` | 空 | 觀察清單，逗號分隔（如 `BTCUSDT,ETHUSDT`）。啟動時讀一次，執行中不變；空清單等同關閉自動抓取 |
| `KCANDLE_INGESTION_ROUND_CANDLE_COUNT` | `5` | 每輪針對單一交易標的取回幾根已收完的 K 線 |
| `KCANDLE_INGESTION_BACKFILL_LOOKBACK_HOURS` | `24` | 啟動回補最多往回幾小時 |
| `MARKET_DATA_BASE_URL` | Binance 公開行情網址 | 行情來源位址 |
| `MARKET_DATA_REQUEST_TIMEOUT_SECONDS` | `10` | 單次向行情來源請求的逾時 |
| `POSTGRES_HOST` | `localhost` | 資料庫主機 |
| `POSTGRES_PORT` | `5432` | 資料庫埠號 |
| `POSTGRES_USER` | `postgres` | 資料庫帳號 |
| `POSTGRES_PASSWORD` | `postgres` | 資料庫密碼 |
| `POSTGRES_DATABASE` | `go_trading` | 資料庫名稱 |
| `POSTGRES_SSL_MODE` | `disable` | SSL 模式 |

## API Routes

| Method | Path | 說明 |
| :--- | :--- | :--- |
| `GET` | `/health` | 健康檢查，恆回 `200 {"status":"Healthy"}` |
| `POST` | `/k-candles` | 新增一根 K 線；同交易標的同起始時間即覆蓋 |
| `GET` | `/k-candles?symbol=&startTime=&endTime=` | 依交易標的與時間區間查詢，起訖兩端都包含，依起始時間由早到晚 |
| `GET` | `/k-candles/series?symbol=&startTime=&endTime=&interval=` | 同一段區間，依彙總刻度合併後回覆；`interval` 為 `5m`／`15m`／`1h`／`4h`／`1d`，省略視為 `5m` |
| `GET` | `/k-candles/{symbol}/{openTime}` | 讀取單一 K 線 |
| `PUT` | `/k-candles/{symbol}/{openTime}` | 修改單一 K 線的價量數字 |
| `DELETE` | `/k-candles/{symbol}/{openTime}` | 刪除單一 K 線 |
| `POST` | `/indicator-calculations` | 用自訂算式計算指標 |

時間一律為 RFC3339 的世界標準時間（`2026-08-29T09:00:00Z`）。
**修改的對象由網址決定**：內文若帶了與網址不同的交易標的或起始時間，會被拒絕。

狀態碼：規則不通過 `400`、指名的 K 線不存在 `404`、資料庫讀寫失敗 `502`、刪除成功 `204`。
查詢成功但區間內無資料回 `200` 與空陣列。

彙總查詢（`/k-candles/series`）的刻度區間邊界一律自**世界標準時間當日零點**起依刻度長度切分，
查詢區間的起訖不必對齊；一個刻度區間裡的 K 線合併成一根（開盤取最早、收盤取最晚、
最高取最高、最低取最低，成交數字加總），**沒有資料的刻度區間不產出那一根**。
區間依刻度切出的根數超過 `KCANDLE_QUERY_MAX_RESULTS` 時回 `400`，訊息同時給出縮小區間與改用更長刻度兩條出路。
回覆是一個物件（不是陣列）：`{"symbol":…,"interval":…,"kCandles":[…]}`。

```bash
curl -X POST localhost:8080/k-candles -H 'Content-Type: application/json' -d '{
  "symbol":"BTCUSDT","openTime":"2026-08-28T09:00:00Z",
  "open":"100","high":"120","low":"90","close":"110",
  "volume":"11","quoteVolume":"1200","takerBuyBaseVolume":"5","takerBuyQuoteVolume":"600"}'

curl "localhost:8080/k-candles?symbol=BTCUSDT&startTime=2026-08-28T09:00:00Z&endTime=2026-08-28T09:10:00Z"

curl "localhost:8080/k-candles/series?symbol=BTCUSDT&startTime=2026-08-01T00:00:00Z&endTime=2026-08-28T00:00:00Z&interval=1d"
```

`/health` 刻意**直接寫在路由註冊處**（`cmd/server/dependencies.go`），不經過任何
application / service 層——它檢查的是行程活著與否，不是業務行為。

## 專案結構

```
cmd/
├── server/
│   ├── main.go          進入點：載入設定、開 DB、啟動 Gin
│   └── dependencies.go  組裝根：手動 DI 與路由註冊
└── migrate/
    └── main.go          migration 指令：把 entity 定義同步進資料庫

internal/
├── config/              環境變數讀取與 PostgreSQL DSN 組裝
├── controller/          Gin handler + Request struct
├── job/                 背景工作：一工作一檔，統一由 BackgroundJobManager 啟動
│   └── tests/
├── application/         用例編排，呼叫 Domain Service
│   └── tests/
├── domain/              核心，不依賴任何其他層
│   ├── models/
│   │   ├── entities/    乾淨的 Data Model（只有欄位與 ORM 標註）
│   │   ├── domains/     Domain Model：業務行為所在地
│   │   ├── dto/         domain 對 application 的回傳形狀
│   │   └── vo/          不可變、無行為的值物件
│   ├── service/         Domain Service：application 的唯一入口
│   └── interface/       repository / proxy 介面，一介面一檔
│       └── mocks/       mockgen 產生的 mock
└── infrastructure/
    ├── clock/           系統時鐘（讓「不得指向未來」這條規則可被測試）
    ├── marketdata/      行情來源的呼叫與電報格式正規化
    ├── persistence/     GORM 連線、schema migrator 與 repository 實作
    └── script/          指標算式的執行環境（內嵌直譯器與白名單）
```

依賴方向一律指向 `domain/`；`domain/` 不認識 HTTP、GORM 或任何 SDK。

## 新增資料表（Code First）

1. 在 `internal/domain/models/entities/` 定義 entity struct。
2. 到 `internal/infrastructure/persistence/schema_migrator.go` 的 `migratedEntities` 補上它。
3. 執行 `make migrate`，GORM 自動同步 schema。

**不手寫 SQL 字串、不手寫 DDL、不從既有 DB 反向產生模型。**

## 新增可 mock 的介面

介面放 `internal/domain/interface/`，一介面一檔，並在檔內加上 generate 指示詞：

```go
package _interface

//go:generate go tool mockgen -source=i_stock_proxy.go -destination=mocks/mock_i_stock_proxy.go -package=mocks

type IStockProxy interface {
	Fetch(symbol string) (StockQuote, error)
}
```

接著 `make mock`，測試裡就能用：

```go
controller := gomock.NewController(t)
stockProxy := mocks.NewMockIStockProxy(controller)
stockProxy.EXPECT().Fetch("2330").Return(quote, nil)
```

## 自訂指標計算

送出交易標的、要用幾根 K 線、算式產出的**指標值種類**，以及一段算式：

```bash
curl -X POST localhost:8080/indicator-calculations -H 'Content-Type: application/json' -d '{
  "symbol": "BTCUSDT",
  "candleCount": 4,
  "resultType": "float",
  "script": "package main\nimport \"indicator\"\nfunc Calculate(data []indicator.KCandle) map[string]float64 {\n sum := 0.0\n for _, c := range data { sum += c.Close }\n return map[string]float64{\"ma\": sum / float64(len(data))}\n}"
}'
# {"symbol":"BTCUSDT","usedCandleCount":4,"resultType":"float","values":{"ma":115}}
```

**指標值種類**（`resultType`）決定算式要回傳什麼形狀，四選一，**省略等同 `float`**：

| `resultType` | 算式要回傳 | 回應中的值長這樣 | 適合 |
| :--- | :--- | :--- | :--- |
| `float`（預設） | `map[string]float64` | `{"ma":115}` | 一個數字的指標，如均價 |
| `floatList` | `map[string][]float64` | `{"line":[110,112,115]}` | 一整條線，如逐根均線 |
| `bool` | `map[string]bool` | `{"crossed":true}` | 是非題，如黃金交叉發生了嗎 |
| `boolList` | `map[string][]bool` | `{"red":[true,false,true]}` | 逐根的是非，如每根是否收紅 |

**一次計算只有一種**：同一次算出的所有指標都是同一種。要不同種類就分兩次算。

**算式的形狀**固定為（`map` 的值型別跟著 `resultType` 走）：

```go
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
    // 你的運算
}
```

算式回傳的形狀與宣告的種類不符時，會被當成**算式的問題**（`422`）擋下，
訊息會告訴你宣告的是哪一種、`Calculate` 該長什麼樣。

`indicator.KCandle` 有：`Symbol`、`OpenTimeUnixSeconds`、`Open`、`High`、`Low`、
`Close`、`Volume`、`QuoteVolume`、`TakerBuyBaseVolume`、`TakerBuyQuoteVolume`。

**幾條規則**

- **一律排除最新那一根** K 線，因為它涵蓋的五分鐘還沒走完。要 4 根就會撈 5 根丟掉最新的。
- `data` 依起始時間**由早到晚**排列。
- 回傳是**一組「名稱 → 值」**，值的形狀由 `resultType` 決定，放幾個由你決定；回空的也算成功，`floatList` / `boolList` 下某個名稱給空的一串也算成功。
- 單次最多用 `KCANDLE_QUERY_MAX_RESULTS` 根，且不得為零或負數。
- 排除最新一根後不夠用，會拒絕並告訴你**實際可用幾根**。

**算式能用什麼**

只有**純運算**：四則運算、比較、迴圈，加上 `math` 與 `sort` 兩個套件。
`os`、`net/http`、`time`、亂數一律 import 不到——這是白名單擋的，也是為了讓同一批 K 線
跑兩次結果必定相同。想放寬只要改
`internal/infrastructure/script/yaegi_indicator_script_proxy.go` 裡的白名單一處。

**回應狀態**

| 狀況 | 狀態碼 |
| :--- | :--- |
| 成功（含空的結果） | `200` |
| 根數不對、K 線不夠 | `400` |
| 算式跑不動（無法解讀、執行失敗、越權） | `422` |
| 資料庫讀取失敗 | `502` |

**算不完會被砍掉。** 超過 `INDICATOR_SCRIPT_TIMEOUT_SECONDS`（預設 40 秒）即中止，
回 `422` 並告知逾時；被放棄的算式不會繼續佔用資源。

## K 線自動抓取

設定 `KCANDLE_INGESTION_SYMBOLS` 之後，系統會自己把 K 線抓回來，不必手動餵。

**啟動時**先回補：每個交易標的從「最後一根的下一根」補到現在，最多往回
`KCANDLE_INGESTION_BACKFILL_LOOKBACK_HOURS` 小時。**回補全部跑完**才開始定時抓取——
兩邊同時跑會互相覆蓋同一根。

**之後每五分鐘**一輪，每輪取回最近 `KCANDLE_INGESTION_ROUND_CANDLE_COUNT` 根**已收完**的
K 線。取多於一根有兩個用處：吸收行情來源事後修正的數字，以及自動補回上一輪失敗漏掉的。
重複的一律覆蓋，不會產生第二根。

**間隔沒有設定值**，固定五分鐘。它跟 K 線本身的長度綁在一起，設成別的值會有 K 線永遠抓不到。
要停掉只有兩條路：`BACKGROUND_JOBS_ENABLED=false`，或觀察清單留空。

幾條刻意的行為：

- **進行中那一根不存。** 它的數字還會變，而且指標計算本來就排除最新一根。
- **各交易標的彼此獨立。** 一個取不到資料不影響其他，也不用人管——五分鐘後下一輪自然重試。
- **違反 K 線規則的逐根跳過。** 同一批其他合規的照常存入，並留下紀錄指出是哪一根、哪條規則。
- **只在出事時留紀錄。** 正常的輪次不寫任何東西。

換行情來源就是實作 `IMarketDataProxy` 再改組裝根一行，其餘一律不動。

## Postman

`postman/` 底下有一份完整的 collection 與一份本機環境檔。

```
postman/go-trading.postman_collection.json
postman/go-trading.postman_environment.json
```

**可以整包按順序跑**（Postman Runner 或 Newman）：

```bash
newman run postman/go-trading.postman_collection.json \
       -e postman/go-trading.postman_environment.json
```

四個資料夾：健康檢查、K 線、指標計算、收尾。31 個請求、45 條斷言，
含成功路徑與每一種被拒絕的情形（`400` / `404` / `422`）。

**兩個設計上的選擇：**

- **時間在執行當下算出來。** 所有起始時間由 collection 的 pre-request script
  對齊到五分鐘刻度、且一定落在過去，所以不會有寫死的日期過期。
- **CRUD 一律用 `TESTONLY` 這個交易標的**，刻意避開自動抓取的觀察清單，
  測試資料不會跟真實行情互相干擾。收尾資料夾會清掉自己建立的東西，
  所以整包可以重複跑。

**新增或修改路由時，這份 collection 要同步更新。**

> 自動抓取**沒有任何端點**，這是刻意的——它是背景工作，只由環境變數控制，
> 觀察清單無法從外部改動。要看它做了什麼請看伺服器的執行紀錄。

## 資料庫測試

`internal/infrastructure/persistence/tests/` 直接對真實的 PostgreSQL 驗證覆蓋語意、
排序與閉區間。未設定 `TEST_POSTGRES_DSN` 時**這些測試會 skip**，所以 `make test`
可以離線跑；要真的驗到，開一個丟棄式資料庫：

```bash
docker run -d --name go-trading-test -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=go_trading_test -p 55433:5432 postgres:17-alpine

TEST_POSTGRES_DSN="host=localhost port=55433 user=postgres password=testpass dbname=go_trading_test sslmode=disable" \
  make test-storage
```

## 開發規範

架構、命名、測試、風格規範全部寫在 [`.claude/rules/`](.claude/rules/)，
入口是 [`CLAUDE.md`](CLAUDE.md)（情境 → 該讀哪一份規則）。

動工前至少讀這兩份：

- [architecture.md](.claude/rules/architecture.md) — 東西該放哪一層
- [naming.md](.claude/rules/naming.md) — 東西該叫什麼名字

commit 前請自行跑過：

```bash
go build ./... && go vet ./... && go test ./...
```

commit 訊息一律 **English Conventional Commits**。
