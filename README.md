# go-trading

以 **Go + Gin** 打造的交易服務後端 REST API。

目前提供三件事：

1. **K 線（KCandle）的完整讀寫** —— 新增（同交易標的同起始時間即覆蓋）、依交易標的與
   時間區間查詢、以及單一 K 線的讀取、修改、刪除。一根 K 線固定涵蓋五分鐘。
2. **自訂指標計算** —— 你自己寫一段 Go 算式送進來，系統拿指定根數的 K 線餵它，
   把輸出統一收成一組「名稱 → 數字」。加新指標不用改程式。
3. **即時跟盤** —— 有人正在看某個交易標的時，系統跟著那個市場：最新那一根一邊成形、
   畫面一邊跟著動。**成形中的那一根只給看，不存也不參與計算**——它的數字還會變。
   跟盤是加在每五分鐘一輪的自動抓取之上的，壞掉時系統退回原本的樣子。

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

### 關機

`Ctrl+C`（或 `SIGTERM`）是有序關機，不是砍掉：

1. 背景工作被要求**不再開新一輪**。
2. server 停止收新請求，並等**已經收下的請求**回答完，上限 15 秒。
3. 排空結束後（沒有請求要排空時，就是立刻），背景工作手上那一輪被中止。

那 15 秒**只管請求**。手上那一輪不會被等——job 目前還無法回報自己跑完了，
要等就得無條件等滿 15 秒才關得掉。切掉不會壞資料：K 線是一根一根存的，
缺的那幾根由下次啟動的 backfill 補回來，那正是 backfill 先跑的理由。

每一層都吃 `context.Context`，所以第 3 步是真的中止，不是等它自己想起來。
同理，呼叫端斷線時，那個請求觸發的算式與查詢也會跟著收掉。

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

### CI

`.github/workflows/ci.yml` 在每個 PR 與每次推上 `main` 時跑：`gofmt`、`go build`、
`go vet`、mock 是否與介面同步、`go test ./... -race`。

它會開一個 PostgreSQL service container，所以**資料庫測試在 CI 一定會真的跑**——
而且有一步專門在**確認它們沒有 skip**。這是整個檔案存在的理由：
沒設 `TEST_POSTGRES_DSN` 時那些測試會自己 skip，而 skip 掉的守衛跟通過的守衛長得一模一樣。
本機 `make test` 綠不代表它們驗過了，`make test-storage` 才是。

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
| `MARKET_DATA_STREAM_URL` | Binance 公開即時行情網址 | 即時跟盤的行情來源位址 |
| `LIVE_UPDATE_INTERVAL_CEILING_SECONDS` | `10` | 成形中的那一根至多多久送給觀看者一次；**一根走完不受此限**，一律立即送出 |
| `LIVE_FEED_QUIET_TIMEOUT_SECONDS` | `30` | 多久沒收到任何東西就當成跟不動。寧可誤判：白重連一次的代價，遠低於讓人盯著停格的圖 |
| `LIVE_FEED_MAX_RETRY_DELAY_SECONDS` | `30` | 重連間隔逐次加倍的上限；系統不放棄重試 |
| `AUTH_ACCESS_TOKEN_SIGNING_KEY` | 空 | 簽發登入憑證的鑰匙。**沒有預設值也不該有**——有預設值就是所有人共用一把，那樣的憑證誰都能自己偽造。沒設時只有 `POST /sessions` 不能用（回 `503`），其餘功能照常。產生一把：`openssl rand -base64 48` |
| `AUTH_ACCESS_TOKEN_LIFETIME_MINUTES` | `15` | 一份**登入憑證**能用多久（分鐘）。它仍然不留存、撤不掉，所以這個數字就等於「登出之後那一張還通得過多久」。**舊的 `AUTH_ACCESS_TOKEN_LIFETIME_HOURS` 已不再讀取**——單位換了，沿用舊名會讓寫著 `24` 的設定安靜地從一天變成 24 分鐘 |
| `AUTH_REFRESH_TOKEN_LIFETIME_DAYS` | `30` | 一份**續用憑證**能用多久（天）。每次續用都從當下重算：持續使用就不必重登，連續不用超過這個天數才要 |
| `ANTHROPIC_API_KEY` | 空 | 行情對話助手的憑證。沒設就只有 `/chat` 不能用，其餘功能照常 |
| `ASSISTANT_MODEL` | `claude-opus-5` | 要問哪一個助手 |
| `ASSISTANT_EFFORT` | `low` | 助手能想多久。對話不需要想太久；挑錯工具的代價比想得淺重得多，所以模型維持能幹的那個、只把力度調低 |
| `ASSISTANT_BASE_URL` | 空 | 助手的位址。空的就是助手自己的位址；填了是為了指向閘道、錄製代理或測試用的替身 |
| `ASSISTANT_RECENT_MESSAGE_LIMIT` | `20` | 每次回答給助手看幾則訊息。更早的仍然讀得到，只是不送——這是「對話越長越貴」的煞車 |
| `ASSISTANT_QUERY_LIMIT` | `8` | 一次回答最多發動幾次工具查詢 |
| `ASSISTANT_CANDLE_LIMIT` | `200` | 一次工具查詢最多給助手幾根 K 線。刻意遠低於 `KCANDLE_QUERY_MAX_RESULTS`：那個管回應裝得下多少，這個管一次回答花多少 |
| `ASSISTANT_DAILY_USAGE_ALLOWANCE` | `300000` | 一天的助手用量絕對上限。用盡即拒絕新提問——這是讓帳單「不可能」爆，而不只是「不太會」爆的那一道 |
| `ASSISTANT_ANSWER_LENGTH_LIMIT` | `2000` | 一次回答的長度上限 |
| `ASSISTANT_RESPONSE_TIMEOUT_SECONDS` | `120` | 一次回答最長等多久；逾時即中止，不留下半截的問答 |
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
| `GET` | `/trading-symbols` | 列出系統認得的每一個交易標的：**已登錄的**加上**實際有 K 線的**，去重、依名稱由小到大 |
| `POST` | `/indicator-calculations` | 用自訂算式計算指標；可指定彙總刻度、要看幾格、算到哪個時間為止，以及這一次的參數值 |
| `GET` | `/k-candles/live?symbol=` | 持續送出該交易標的的即時更新（Server-Sent Events）；每則一個事件 |
| `POST` | `/chat` | 問行情助手一句話；不指名對話即開一段新的，回覆一段回答與這段對話的識別碼 |
| `GET` | `/chat/conversations` | 列出每一段對話，最近有動靜的排前面 |
| `GET` | `/chat/conversations/{id}` | 讀一段對話的每一則訊息，依時間由早到晚 |
| `POST` | `/users` | 用電子郵件與密碼建立一位使用者；回覆識別碼與電子郵件，**永遠不含密碼或由它算出來的任何東西** |
| `POST` | `/sessions` | 登入；對得上就開一段**登入階段**，回覆**一對憑證**（登入憑證＋續用憑證）與各自的到期時刻 |
| `POST` | `/sessions/renewal` | 帶著**續用憑證**換一對全新的憑證；舊的那一份當場作廢 |
| `POST` | `/sessions/revocation` | 登出：帶著續用憑證，把整條換發鏈作廢。恆回 `204` |
| `GET` | `/users/me` | 帶著 `Authorization: Bearer <登入憑證>` 問「我是誰」 |

時間一律為 RFC3339 的世界標準時間（`2026-08-29T09:00:00Z`）。
**修改的對象由網址決定**：內文若帶了與網址不同的交易標的或起始時間，會被拒絕。

狀態碼：規則不通過 `400`、指名的 K 線不存在 `404`、資料庫讀寫失敗 `502`、刪除成功 `204`。
查詢成功但區間內無資料回 `200` 與空陣列。

彙總查詢（`/k-candles/series`）的刻度區間邊界一律自**世界標準時間當日零點**起依刻度長度切分，
查詢區間的起訖不必對齊；一個刻度區間裡的 K 線合併成一根（開盤取最早、收盤取最晚、
最高取最高、最低取最低，成交數字加總），**沒有資料的刻度區間不產出那一根**。
區間依刻度切出的根數超過 `KCANDLE_QUERY_MAX_RESULTS` 時回 `400`，訊息同時給出縮小區間與改用更長刻度兩條出路。
回覆是一個物件（不是陣列）：`{"symbol":…,"interval":…,"kCandles":[…]}`。

`/trading-symbols` 回的是**兩邊的聯集**：`TradingSymbols` 裡已登錄的市場，
加上 `KCandles` 裡實際出現過的交易標的。已登錄但還沒有資料的**會出現**——
資料庫剛建好時它們就是選單上唯一的選項；有資料但沒登錄過的（例如手動建的新市場）**也會出現**。
兩邊都空時回 `200` 與空陣列。**它不取自觀察清單設定**——那是「打算抓什麼」，與「系統認得什麼」是兩件事。

```bash
curl -X POST localhost:8080/k-candles -H 'Content-Type: application/json' -d '{
  "symbol":"BTCUSDT","openTime":"2026-08-28T09:00:00Z",
  "open":"100","high":"120","low":"90","close":"110",
  "volume":"11","quoteVolume":"1200","takerBuyBaseVolume":"5","takerBuyQuoteVolume":"600"}'

curl "localhost:8080/k-candles?symbol=BTCUSDT&startTime=2026-08-28T09:00:00Z&endTime=2026-08-28T09:10:00Z"

curl "localhost:8080/k-candles/series?symbol=BTCUSDT&startTime=2026-08-01T00:00:00Z&endTime=2026-08-28T00:00:00Z&interval=1d"

curl localhost:8080/trading-symbols
```

`/health` 刻意**直接寫在路由註冊處**（`cmd/server/dependencies.go`），不經過任何
application / service 層——它檢查的是行程活著與否，不是業務行為。

## 使用者登入

系統認得人：一個人用**電子郵件**當帳號、配一組**密碼**建立自己的使用者，
用這兩樣登入拿到一份**登入憑證**，往後帶著它，系統就認得出他是誰。

```bash
curl -X POST localhost:8080/users -H 'Content-Type: application/json' \
  -d '{"email":"james@example.com","password":"correct horse"}'

SESSION=$(curl -s -X POST localhost:8080/sessions -H 'Content-Type: application/json' \
  -d '{"email":"james@example.com","password":"correct horse"}')
TOKEN=$(echo "$SESSION" | jq -r .accessToken)
REFRESH=$(echo "$SESSION" | jq -r .refreshToken)

curl localhost:8080/users/me -H "Authorization: Bearer $TOKEN"

# 十五分鐘後：不必重打密碼，換一對新的
curl -X POST localhost:8080/sessions/renewal -H 'Content-Type: application/json' \
  -d "{\"refreshToken\":\"$REFRESH\"}"

# 登出（真的撤得掉）
curl -i -X POST localhost:8080/sessions/revocation -H 'Content-Type: application/json' \
  -d "{\"refreshToken\":\"$REFRESH\"}"
```

### 一次登入拿到的是一對憑證，不是一張

| | 活多久 | 留存嗎 | 幹嘛用的 |
| :--- | :--- | :--- | :--- |
| **登入憑證** | 15 分鐘 | **不留存** | 每一次請求帶的就是它。驗它只看簽章，**一次資料庫都不讀** |
| **續用憑證** | 30 天 | **留存**（存的是算不回去的留存樣） | 只做一件事：換一對新的。因為留存，所以**撤得掉** |

這兩件事本來是互斥的：撤得掉就得每次查資料庫，不查資料庫就撤不掉。
拆成一對之後，把狀態放在**只有低頻操作會碰到的那一半**，兩邊各取所需。

### 續用憑證只能用一次

每次換發，舊的那一份**當場作廢**，新的那一份接在同一條**換發鏈**上。

所以一份**已經作廢**的續用憑證又被拿來換發，只有兩種可能：被複製，或被偷。
這時系統**把整條換發鏈全部作廢**——包含目前那一份還沒用過的。

真正的持有者會因此被登出。這是刻意的：此刻**沒有辦法分辨誰是真的**，
只撤其中一邊等於什麼都沒做。

代價是會誤傷（例如兩個分頁同時續用），而誤傷的代價是重登一次。

### 登出是真的登出，但有一個 15 分鐘的尾巴

`POST /sessions/revocation` 撤的是**整條換發鏈**，所以那台裝置之後換不到任何東西。

但**登入憑證撤不掉**——它不留存。所以最長還有 `AUTH_ACCESS_TOKEN_LIFETIME_MINUTES`
那麼久，它仍然通得過。**那個設定值就是這個尾巴的長度**，這是無狀態換來的，寫出來而不是假裝沒有。

登出一段不存在或已經作廢的登入階段**不算失敗**——目的已經達成了。
而且**登出的是這一台，不是這個人**：同一位使用者在別台的登入階段照常有效。

### 密碼從來沒有被存下來過

存的是一段由密碼算出來、**算不回去**的**密碼證明**，而且每次算都摻不同的隨機料——
兩個人剛好用同一組密碼，留下的兩份證明也不一樣。整個資料庫被整份拿走，也拿不到任何人的密碼。

算它**刻意是慢的**（`bcrypt`，成本 12，約兩百毫秒）。這讀起來像缺點，
但要從偷走的資料反推密碼得猜上幾十億次，而擋在中間的就只有「每一次猜要多少錢」。
快到我們免費的算法，對他也一樣免費。

密碼因此有一條**上限 72 個位元組**的規則（中文字一個算三個），而且**超過是拒絕，不是截短**。
截短的話，使用者以為自己設了一組很長的密碼，實際生效的只有前面那一段，而他永遠不會知道。

### 登入失敗永遠只有一種說法

不論是查無這個電子郵件、還是密碼打錯，回的都是同一句「電子郵件或密碼不正確」——
**一字不差**。分開講等於免費奉送一份「哪些電子郵件註冊過」的名單。

連花掉的時間都一樣：查無此人時系統**仍然**拿一份誘餌證明去比對一次。
少了這一步，這條路會回得比密碼錯明顯更快，而「這次特別快」跟直接講出來沒有兩樣。

### 換掉鑰匙仍然是唯一的「一次登出所有人」

要讓**每一個人、每一台裝置**立刻重新登入，換掉 `AUTH_ACCESS_TOKEN_SIGNING_KEY` 即可——
所有已簽發的登入憑證同時對不上簽章。

### 建立使用者目前是開放的，而其餘端點目前不需要憑證

兩件事都是刻意的，而且都會再回來處理：

- **建立使用者不需要先登入**——系統一位使用者都沒有時，關起來就沒有人建得出第一位。
- **K 線、指標計算、策略、助手、即時跟盤目前一律不問來者是誰。**
  把門裝上去是另一個切片，因為 `GET /k-candles/live` 是瀏覽器的持續連線、**送不出授權標頭**，
  憑證要怎麼過去得先想清楚。和其餘端點一起改，等於在一個切片裡塞兩個不同的問題。

  要裝的時候，落點是 `UserController` 裡讀 `Authorization` 標頭的那段私有 method：
  它會長成一個中介層，掛在路由群組上、把認出來的使用者放進請求脈絡。

## 行情對話助手

`/chat` 讓你用日常講話的方式問行情：助手自己去借用系統原本就有的能力取數，再用一段話講回來。
問完能接著問，因為一連串問答被留成一段**對話**。

```bash
curl -X POST localhost:8080/chat -H 'Content-Type: application/json' \
  -d '{"question":"BTCUSDT 最近一天每小時的形狀？"}'
# {"conversationId":1,"answer":"...","queryCount":2,"stoppedAtQueryLimit":false,"usage":3184}

curl -X POST localhost:8080/chat -H 'Content-Type: application/json' \
  -d '{"conversationId":1,"question":"那 ETHUSDT 呢？"}'

curl localhost:8080/chat/conversations
curl localhost:8080/chat/conversations/1
```

### 一則帶著查詢請求的回覆不是答案

助手**很常在同一則回覆裡同時做兩件事**：說一句「我先看一下既有的算式寫法」，
並附上它要發動的查詢。因此迴圈是**先問「有沒有要查」，再問「有沒有說話」**——
反過來的話，那句旁白會被當成最終答案回傳，工具一次都沒跑，
使用者拿到一個承諾然後只能自己再問一次「好了沒」。

那句話不會被丟掉：它作為**旁白**跟著自己那一輪的查詢請求一起回到助手面前，
否則助手下一輪是從一個它已經看不到的想法往下接，答案會從半句話開始。

以**輪**為單位還修掉另一件事：助手一次問三件事，就要以一則助手訊息帶三個請求、
一則回覆帶三個結果送回去。拆成三次各自的一問一答，它會學到
「一次問幾件事沒有用」，從此每件事都多花一次往返。

### 一次往返，不是一整個迴圈

`IAssistantProxy` 只做**一次**請求／回應：給它訊息，它回「一段回答」或「我要發動哪幾次查詢」，
外加這次的用量。它不執行任何查詢、不迭代、不記狀態。

迴圈、次數上限、截斷、額度結算、落地，全部住在 `AssistantConversationService` 裡。
這樣切的理由是：那些全都是**業務規則**，得住在測得到的地方——交給 SDK 自帶的 tool runner，
每一條就都搬進 infrastructure，唯一的驗證方式變成付錢買一次真的回答。

換一家助手是換一個 `IAssistantProxy` 實作，流程一行都不用改。

### 助手能做什麼，是一份注入的清單

助手可發動的每一項能力都是一個 `IAssistantQuery` 實作，清單在 `cmd/server/dependencies.go`
的 `assistantQueriesFor` 組出來。目前八項：列出交易標的、查 K 線、查彙總 K 線序列、
算指標、讀策略、列策略、建策略、改策略。

**沒有刪除策略，也沒有任何 K 線寫入。** 這不是一段防禦碼，是**清單裡沒有那一項**——
不存在的能力沒有辦法被誤呼叫，也沒有哪次重構能不小心把它放回來。
`cmd/server/assistant_queries_test.go` 守著這條界線。

每一項能力都呼叫**跟人一樣的那個用例**，所以沒有一條規則為助手放寬，也沒有一條規則寫兩次。
規則拒絕時，助手拿到的是**拒絕的原因**，它可以改一改再試或直接說辦不到——
查詢被拒**不等於**整次回答失敗。

### 帳單的四道天花板

| 天花板 | 預設 | 管什麼 |
| :--- | :--- | :--- |
| 近期訊息則數 | 20 則 | 對話越長越貴。更早的訊息仍然讀得到，只是不送給助手 |
| 查詢次數 | 8 次 | 助手不會為一個問題無止盡翻資料。用完仍無結論時，**回答目前所得並說明已達上限** |
| 單次查詢根數 | 200 根 | 一個大區間不會讓一次回答變得極貴。超過只給最新的 200 根，並**明確告知已截斷** |
| 每日用量額度 | 300000 | 一天的絕對上限。用盡即拒絕新提問，回 `429` 並說明何時重置 |

**截斷一定要告知。** 助手看到五百根中的最新兩百根卻不知道，會把片段當全貌，
描述一個不存在的趨勢——這是成本上限唯一可能自己造成的錯。

額度是**事後結算**的：提問時還沒達標就放行，即使這一則答完會超出。最多超一則問答，
換來的是「開始時還在額度內的回答不會被中途拒絕」。單則的份量本身已被上面三道封頂。

### 助手沒回應時，不留半截

一次問答是**一個 row**（`AssistantTurns`：提問、回答、用量、查詢次數、是否提早收尾）。
提問與回答同生共死——助手掛掉或逾時，這一則問答整個不留下，對話維持原狀。
放在同一個 row 裡，「不留半截」是**寫或不寫**，而不是一段要記得回滾的兩步。

狀態碼：提問空白 `400`、對話不存在 `404`、今日額度用盡 `429`、助手沒回應或逾時 `503`、
其餘讀寫失敗 `502`。

### 費用感

用 `claude-opus-5`（$5 / $25 每百萬 token）、`effort=low`，一輪對話大約 input 6k、output 600，
約 **$0.045 一輪（新台幣 1.4 元）**。System prompt 與工具定義是固定不變的位元組並下了快取斷點，
重複問答時那一段以一折價計。每日額度 300000 對應每天約新台幣 50 元的天花板。

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
├── job/                 背景工作：一工作一檔，統一由 BackgroundJobManager 啟動與停止
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

送出交易標的、要吃**多粗**的 K 線、要**幾根**、算到**哪個時間為止**，
算式產出的**指標值種類**，以及一段算式：

```bash
curl -X POST localhost:8080/indicator-calculations -H 'Content-Type: application/json' -d '{
  "symbol": "BTCUSDT",
  "aggregationInterval": "1h",
  "candleCount": 4,
  "endTime": "2026-08-29T09:00:00Z",
  "resultType": "float",
  "script": "package main\nimport \"indicator\"\nfunc Calculate(data []indicator.KCandle) map[string]float64 {\n sum := 0.0\n for _, c := range data { sum += c.Close }\n return map[string]float64{\"ma\": sum / float64(len(data))}\n}"
}'
# {"symbol":"BTCUSDT","interval":"1h","usedCandleCount":4,
#  "openTimes":["2026-08-29T05:00:00Z","2026-08-29T06:00:00Z","2026-08-29T07:00:00Z","2026-08-29T08:00:00Z"],
#  "resultType":"float","values":{"ma":115}}
```

**這三樣都不記在策略身上。** 一支策略只記著名稱、算式與指標值種類——
要多粗、要幾根、算到什麼時候，是**這一次執行**的事。所以同一支「二十根均線」
可以在一小時的刻度上看一次、再在五分鐘的刻度上看一次，不必存成兩支。

| 欄位 | 省略時 | 說明 |
| :--- | :--- | :--- |
| `aggregationInterval` | `5m` | 五選一：`5m`／`15m`／`1h`／`4h`／`1d`，與彙總查詢共用同一組刻度 |
| `candleCount` | 必填 | 要幾根**彙總後**的 K 線；一小時 × 24 與五分鐘 × 288 都是回看一天 |
| `endTime` | 現在 | 算到哪個時間為止；**指向未來時視同現在**，不拒絕 |

回覆多帶三樣：`interval`（這次實際用的刻度）、`usedCandleCount`、
以及 `openTimes`——**這次餵給算式的每一根從哪裡開始**，由早到晚。
`floatList` / `boolList` 的第 n 個值就對應 `openTimes` 的第 n 個，
所以要把一條線畫回圖上，不必自己從刻度與根數反推是哪幾根。

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

- **只採用走完的刻度區間。** 還在走的那一格不會被讀進來——它裝了一半，
  算出來的值會隨著時間自己變。`endTime` 剛好落在格子邊界上時，前一格已經走完，**要用**。
  這條**不分現在與過去**：`endTime` 指向去年某個時刻時，它落入的那一格同樣還沒走完。
  五分鐘刻度下，這剛好就是舊的「排除最新一根」；一小時刻度下，它排掉的是一個只走了 35 分鐘的小時。
- `data` 依起始時間**由早到晚**排列，每一格一根，起始時間是**那一格的起點**（不是格子裡任何一根 K 線的）。
- **沒有資料的刻度區間不產出那一根**，也不補洞、不沿用前一格；有一根 K 線就算數，不要求裝滿。
- 回傳是**一組「名稱 → 值」**，值的形狀由 `resultType` 決定，放幾個由你決定；回空的也算成功，`floatList` / `boolList` 下某個名稱給空的一串也算成功。
- 單次最多用 `KCANDLE_QUERY_MAX_RESULTS` 根**彙總後**的 K 線，且不得為零或負數；與一根多粗無關。
- 走完的格子湊不滿要求的根數，會拒絕並告訴你**實際湊得出幾根**——
  十二根的二十根均線一樣算得出一個數字，而那個數字錯得看不出來。
- **「這支算法至少需要幾根」由算式自己把關**（`len(data)` 不夠就讓它失敗）。系統不替算式猜。

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

- **進行中那一根不存。** 它的數字還會變，而且指標計算本來就不採用還在走的那一格。
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

## 預設交易標的

`make migrate` 除了建立資料表，還會把**預設交易標的**（`BTCUSDT`、`ETHUSDT`）登錄進
`TradingSymbols`，讓操作介面的交易標的選單在第一批 K 線抓回來之前就有東西可挑。

登錄前會先讀出已經登錄了哪些，**只寫還沒有的那些**，並印出這次新登錄了哪幾個：

```
migration applied to 2 table(s): KCandles, TradingSymbols
default trading symbols: registered 2 new (BTCUSDT, ETHUSDT)
```

重跑幾次都安全，第二次起會說 `already registered, nothing to add`。
預設清單寫在 `internal/domain/service/trading_symbol_service.go` 的 `defaultTradingSymbols`，
目前不可用環境變數設定。

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

**這些測試每次都會清空 `KCandles`**，所以 `TEST_POSTGRES_DSN` 指到的資料庫**名稱必須以 `_test` 結尾**——
不是的話測試會直接失敗並說明原因，而不是把應用程式在用的那一份資料清掉。

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

## 即時跟盤

有人正在看某個交易標的時，系統就跟著那個市場，並把更新持續送給觀看者：

```
curl -N 'localhost:8080/k-candles/live?symbol=BTCUSDT'

data: {"symbol":"BTCUSDT","status":"forming","kCandle":{...}}
data: {"symbol":"BTCUSDT","status":"closed","kCandle":{...}}
data: {"symbol":"BTCUSDT","status":"stalled","kCandle":{...}}
```

每則更新的 `status` 是三者之一：

| `status` | 意思 |
| :--- | :--- |
| `forming` | 這一根還在走。**數字還會變，系統不會存它，指標計算也不會用它** |
| `closed` | 這一根走完了，這是它的最終數字。**送出的同一刻就被存進系統** |
| `stalled` | 即時更新已經停止。系統自己在重連，不需要人介入 |

幾件值得先知道的事：

- **跟盤的單位是交易標的**：同一個標的無論多少人在看，系統只跟一份。
  最後一個觀看者離開就停——沒有人看的時候，跟盤買不到任何每五分鐘那一輪不會補上的東西。
- **跟的是觀看者要的那一個**，與觀察清單（`KCANDLE_INGESTION_SYMBOLS`）無關。
  清單管「長期要留下哪些市場的資料」，跟盤管「現在誰在看什麼」。
- **每五分鐘一輪的自動抓取照常運作**，它補上沒人看時走完的每一根，
  並吸收行情來源事後對已收完 K 線的修正——兩件跟盤做不到的事。
- **即時完全不能用時，其他功能一律照常**：查詢、新增、修改、刪除、指標計算與自動抓取
  都不受影響，畫面退回原本的樣子（新資料五分鐘內出現）。

## 策略自己的旋鈕

一支策略除了名稱、算式與指標值種類之外，還記著**它自己的參數**——
算式裡那些可以調的數字，例如均線的期數、布林通道的倍數。

```jsonc
{
  "name": "布林通道",
  "resultType": "floatList",
  "script": "...",
  "parameters": [
    { "name": "期數", "kind": "lookbackCount", "defaultValue": 20 },
    { "name": "倍數", "kind": "number",        "defaultValue": 2 }
  ]
}
```

種類只有兩種，而分成兩種**只有一個理由**——系統必須知道要拿多少 K 線：

| `kind` | 是什麼 | 系統怎麼用它 |
| :--- | :--- | :--- |
| `lookbackCount` | 大於零的**整數**，這條線要看過去多少根 | **拿所有這一種的最大值**去算要讀幾根 |
| `number` | 任何數字，含負數與小數 | 不解讀，原樣交給算式 |

算式以名字取用它們，拿到的就是那一種該有的樣子——沒有型別斷言：

```go
func Calculate(data []indicator.KCandle) map[string][]float64 {
    period := indicator.LookbackCount("期數")   // int，可以直接拿去切片
    factor := indicator.Number("倍數")          // float64
    ...
}
```

**取用一個沒有宣告的名字，這一次計算就失敗**，並指出是哪一個名字對不上。
它不會被說成「算式執行失敗」——把參數改了名卻忘了改算式，是很容易犯而且看不出來的錯，
說成算式壞了會讓人去讀錯的地方。

執行計算時：

```jsonc
{
  "symbol": "BTCUSDT",
  "candleCount": 12,          // 我要幾格有值，不是要拿幾根
  "parameters":      [ { "name": "期數", "kind": "lookbackCount", "defaultValue": 20 } ],
  "parameterValues": [ { "name": "期數", "value": 50 } ]   // 這一次改成 50
}
```

**要拿幾根不必也不能填**——它是算出來的：`要看的格數 + 最大回看根數 − 1`。
上面這個例子會去拿 61 根，算出 12 個值。沒有任何回看根數參數時就是 12 根，不多不少。

沒給值的參數用它宣告的預設值；給了一個沒有宣告的名字則整次拒絕——
安靜忽略會讓人以為調的那一格有作用，而它什麼都沒做。

> **部署提醒**：本功能新增一張資料表，部署前請執行 `make migrate`。
