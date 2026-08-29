# go-trading

以 **Go + Gin** 打造的交易服務後端 REST API。

目前提供 **K 線（KCandle）的完整讀寫**：新增（同交易標的同起始時間即覆蓋）、
依交易標的與時間區間查詢、以及單一 K 線的讀取、修改、刪除。
一根 K 線固定涵蓋五分鐘。

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
| `KCANDLE_QUERY_MAX_RESULTS` | `1000` | 單次區間查詢最多回傳幾根 K 線；超過即拒絕 |
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
| `GET` | `/k-candles/{symbol}/{openTime}` | 讀取單一 K 線 |
| `PUT` | `/k-candles/{symbol}/{openTime}` | 修改單一 K 線的價量數字 |
| `DELETE` | `/k-candles/{symbol}/{openTime}` | 刪除單一 K 線 |

時間一律為 RFC3339 的世界標準時間（`2026-08-29T09:00:00Z`）。
**修改的對象由網址決定**：內文若帶了與網址不同的交易標的或起始時間，會被拒絕。

狀態碼：規則不通過 `400`、指名的 K 線不存在 `404`、資料庫讀寫失敗 `502`、刪除成功 `204`。
查詢成功但區間內無資料回 `200` 與空陣列。

```bash
curl -X POST localhost:8080/k-candles -H 'Content-Type: application/json' -d '{
  "symbol":"BTCUSDT","openTime":"2026-08-28T09:00:00Z",
  "open":"100","high":"120","low":"90","close":"110",
  "volume":"11","quoteVolume":"1200","takerBuyBaseVolume":"5","takerBuyQuoteVolume":"600"}'

curl "localhost:8080/k-candles?symbol=BTCUSDT&startTime=2026-08-28T09:00:00Z&endTime=2026-08-28T09:10:00Z"
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
    └── persistence/     GORM 連線、schema migrator 與 repository 實作
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
