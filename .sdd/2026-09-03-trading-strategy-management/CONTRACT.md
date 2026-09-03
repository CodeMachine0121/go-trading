# 交易策略管理 — Contract Verification

**Contract source:** `.sdd/2026-09-03-trading-strategy-management/PRD.md`（Section 3 Acceptance Criteria 為 oracle）
**Design map:** `ARCH.md`（Traceability 表用來定位程式碼，非契約本身）
**Glossary:** `.sdd/UL-MAP.md`
**Ceiling:** 這是**靜態一致性稽核**——逐條把「測試斷言的東西」與「程式碼實際產出的東西」分別對照規格推導出的 oracle，
不以測試全綠作為判準，也不執行自己發明的情境。已跑的僅是對照條款所映射的那一支測試作為佐證。

---

## 判定摘要

| 判定 | 數量 |
| :--- | ---: |
| ✅ conforms | 53 |
| 🟡 partial（程式碼對，但沒有自動化測試釘住） | 1 |
| 🔴 violation | 0 |
| 🟠 mis-asserted | 0 |
| ❌ gap | 0 |
| ❔ unclear | 0 |
| ⚠️ orphan | 0 |

**Conformance: 53 / 54 = 98.1%**

初次稽核為 45 ✅ / 5 🟠 / 4 🟡。九條「測試綠但沒真的釘住 PRD 承諾」已於稽核後補強，
每一條都經反向驗證（破壞行為 → 對應測試轉紅 → 還原），詳見文末〈稽核後的補強〉。

---

## Clauses

程式碼位置縮寫：
`dom` = `internal/domain/models/domains/strategy_domain.go`、
`ent` = `internal/domain/models/entities/strategy.go`、
`svc` = `internal/domain/service/strategy_service.go`、
`repo` = `internal/infrastructure/persistence/strategy_repository.go`、
`ctl` = `internal/controller/strategy_controller.go`

測試位置縮寫：
`Tdom` = `internal/domain/models/domains/tests/strategy_domain_test.go`、
`Tent` = `internal/domain/models/entities/tests/strategy_test.go`、
`Tapp` = `internal/application/tests/strategy_application_test.go`、
`Tctl` = `internal/controller/tests/strategy_controller_test.go`、
`Trepo` = `internal/infrastructure/persistence/tests/strategy_repository_test.go`

### US-01 把一套算法留下來

| ID | Clause | Oracle（只由規格推導） | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01 | 建立一支完整的策略 | 建立成功並回覆識別碼；再讀回來時五項內容與存入時完全一致 | `dom:41`、`svc:32`、`repo:33` | `Tapp:64`、`Tctl:84` | asserts-oracle | produces-oracle | ✅ |
| AC-02 | 系統一支策略都沒有時建立第一支 | 建立成功——不需要先有任何東西 | `repo:33` | `Trepo:26`（每次由空表開始） | asserts-oracle | produces-oracle | ✅ |
| AC-03 | 未指定彙總刻度時視為五分鐘 | 建立成功，該策略的彙總刻度為五分鐘 | `dom:57`→`NewAggregationIntervalDomain` | `Tdom:44` | asserts-oracle | produces-oracle | ✅ |
| AC-04 | 未指定指標值種類時視為一個數字 | 建立成功，該策略的指標值種類為一個數字 | `dom:62`→`NewIndicatorResultTypeDomain` | `Tdom:44` | asserts-oracle | produces-oracle | ✅ |
| AC-05 | 策略名稱前後的空白不予保留 | 建立成功，記住的名稱是去掉前後空白後的那一個 | `dom:44` | `Tdom:86`、`Tapp:87` | asserts-oracle | produces-oracle | ✅ |
| AC-06 | 策略名稱恰好 128 個字 | 建立成功 | `dom:50` | `Tdom:86` | asserts-oracle | produces-oracle | ✅ |
| AC-07 | 策略名稱超過 128 個字 | 拒絕，說明長度上限為 128 個字；不留下任何策略 | `dom:50` | `Tdom:123`、`Tapp:160` | asserts-oracle | produces-oracle | ✅ |
| AC-08 | 策略名稱為空白 | 拒絕，說明必須給策略取一個名稱；不留下任何策略 | `dom:45` | `Tdom:123`、`Tapp:160` | asserts-oracle | produces-oracle | ✅ |
| AC-09 | 策略名稱只有空白字元 | 同上——只有空白等同沒取 | `dom:44` | `Tdom:123` | asserts-oracle | produces-oracle | ✅ |
| AC-10 | 指標算式為空白 | 拒絕，說明策略必須帶一段指標算式；不留下任何策略 | `dom:54` | `Tdom:123`、`Tapp:160` | asserts-oracle | produces-oracle | ✅ |

> AC-07～AC-10 的「系統不留下任何策略」由 `Tapp:160` 釘住：該測試對 repository **完全不設任何預期**，
> 任何一次落地呼叫都會讓測試失敗。

### US-02 用名字認出是哪一支

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-11 | 不同名稱的兩支策略都留得住 | 建立成功 | `ent:20`（唯一索引） | `Trepo:92`（三個相異名稱皆成功） | asserts-oracle | produces-oracle | ✅ |
| AC-12 | 名稱與既有策略完全相同 | 拒絕並說明該名稱已被使用；**既有那一支完全不受影響** | `repo:71`、`ent:20` | `Trepo:38` | asserts-oracle | produces-oracle | ✅ |
| AC-13 | 只差前後空白的名稱視為重複 | 拒絕——前後空白不計入，兩者是同一個名稱 | `dom:44`（去空白）+ `ent:20`（索引） | `Tapp:105`（兩種寫法到達儲存層時是同一個名稱）+ `Trepo:38`（同一個名稱兩次即衝突） | asserts-oracle | produces-oracle | ✅ |
| AC-14 | 名稱只要有實質差異就不算重複 | 建立成功 | `ent:20` | `Trepo:92` | asserts-oracle | produces-oracle | ✅ |
| AC-15 | 大小寫不同視為不同名稱 | 兩支都建立成功 | `ent:20`（一般唯一索引，區分大小寫） | `Trepo:60` | asserts-oracle | produces-oracle | ✅ |
| AC-16 | 策略刪除後名稱空出來 | 同名可以再建一支，建立成功 | `repo:116`（真刪除） | `Trepo:72` | asserts-oracle | produces-oracle | ✅ |

### US-03 只認得系統真的組得出來的彙總刻度

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-17 | 指定一小時 | 建立成功，彙總刻度為一小時 | `dom:57` | `Tdom:30`、`Tdom:123`(accepts-every-interval) | asserts-oracle | produces-oracle | ✅ |
| AC-18 | 指定一天 | 建立成功，彙總刻度為一天 | `dom:57` | `Tdom:123`(accepts-every-interval) | asserts-oracle | produces-oracle | ✅ |
| AC-19 | 未指定彙總刻度 | 建立成功，彙總刻度為五分鐘 | `dom:57` | `Tdom:44` | asserts-oracle | produces-oracle | ✅ |
| AC-20 | 指定七分鐘 | 拒絕，說明只接受那五種；不留下任何策略 | `dom:57` | `Tdom:145`、`Tctl:134`、`Tapp:160` | asserts-oracle | produces-oracle | ✅ |
| AC-21 | 指定一週 | 同上 | `dom:57` | `Tdom:145` | asserts-oracle | produces-oracle | ✅ |
| AC-22 | 修改時把彙總刻度改成不認得的值 | 修改被拒絕；**該策略維持原本的一小時** | `svc:78`（驗證先於落地） | `Tapp:212`（Update 未被呼叫） | asserts-oracle | produces-oracle | ✅ |

### US-04 記住這支策略算出來的值長什麼樣

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-23 | 指定一串數字 | 建立成功，種類為一串數字 | `dom:62` | `Tdom:30` | asserts-oracle | produces-oracle | ✅ |
| AC-24 | 未指定指標值種類 | 建立成功，種類為一個數字 | `dom:62` | `Tdom:44` | asserts-oracle | produces-oracle | ✅ |
| AC-25 | 指定四種以外的種類 | 拒絕，說明可選的四種；不留下任何策略 | `dom:62` | `Tdom:145`、`Tapp:160` | asserts-oracle | produces-oracle | ✅ |
| AC-26 | 修改時把種類改成四種以外的值 | 修改被拒絕；該策略維持原本的種類 | `svc:78` | `Tapp:212` | asserts-oracle | produces-oracle | ✅ |

### US-05 記住這支策略要吃幾根 K 線

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-27 | 一般的計算根數（20） | 建立成功 | `dom:68` | `Tdom:30`、`Tdom:201` | asserts-oracle | produces-oracle | ✅ |
| AC-28 | 計算根數為一根 | 建立成功 | `dom:68` | `Tdom:201` | asserts-oracle | produces-oracle | ✅ |
| AC-29 | 計算根數恰好等於上限（1000） | 建立成功 | `dom:72` | `Tdom:201` | asserts-oracle | produces-oracle | ✅ |
| AC-30 | 計算根數超過上限（1001） | 拒絕，說明超過單次可用的最大根數；不留下任何策略 | `dom:72` | `Tdom:145`、`Tapp:160` | asserts-oracle | produces-oracle | ✅ |
| AC-31 | 計算根數為零 | 拒絕，說明必須大於零；不留下任何策略 | `dom:68` | `Tdom:145`、`Tapp:160` | asserts-oracle | produces-oracle | ✅ |
| AC-32 | 計算根數為負數 | 同上 | `dom:68` | `Tdom:145` | asserts-oracle | produces-oracle | ✅ |
| AC-33 | 根數的限制與彙總刻度多粗無關 | 288 根 × 五分鐘與 24 根 × 一小時都建立成功 | `dom:68`（兩項各自獨立驗證） | `Tdom:214` | asserts-oracle | produces-oracle | ✅ |

### US-06 把一支策略叫回來

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-34 | 讀取一支存在的策略 | 回覆名稱、算式、種類、刻度、根數、建立時間與最後修改時間 | `svc:50`、`ent:36` | `Tapp:227`（七項全驗）、`Tent:11` | asserts-oracle | produces-oracle | ✅ |
| AC-35 | 讀取一個從未存在過的識別碼 | 回覆找不到該策略 | `repo:85` | `Trepo:85`、`Tapp:245`、`Tctl:216`(404) | asserts-oracle | produces-oracle | ✅ |
| AC-36 | 讀取一支已被刪除的策略 | 回覆找不到該策略 | `repo:85`、`repo:116` | `Trepo:216` | asserts-oracle | produces-oracle | ✅ |

### US-07 看看手上留了哪些策略

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-37 | 清單依名稱由小到大，與建立先後無關 | 依序回覆「二十根均線」「六十根均線」 | `repo:104` | `Trepo:92`（存入順序既非期望序也非其反序） | asserts-oracle | produces-oracle | ✅ |
| AC-38 | 一支策略都沒有 | 回覆空的清單，不視為錯誤 | `svc:61`、`repo:100` | `Trepo:114`、`Tapp:274`、`Tctl:182`（`[]` 而非 `null`） | asserts-oracle | produces-oracle | ✅ |
| AC-39 | 清單上每一支都帶著完整內容 | 每一支都帶自己的名稱、算式、種類、刻度、根數與兩個時間 | `ent:36` | `Tapp:257`（逐支七項全驗） | asserts-oracle | produces-oracle | ✅ |

### US-08 改掉一支策略

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-40 | 只改計算根數 | 修改成功；再讀取時根數為 60；**最後修改時間已更新** | `repo:47` | `Trepo:123`、`Trepo:147`（UpdatedAt 必須往前走） | asserts-oracle | produces-oracle | ✅ |
| AC-41 | 五項內容一次全改 | 五項都是新的值 | `repo:17`、`repo:47` | `Trepo:123` | asserts-oracle | produces-oracle | ✅ |
| AC-42 | 改回自己原本的名稱 | 修改成功——不會與自己撞名 | `ent:20`（撞到的是自己那一列） | `Trepo:166` | asserts-oracle | produces-oracle | ✅ |
| AC-43 | 改成另一支策略的名稱 | 拒絕並說明名稱已被使用；**兩支的內容都完全沒有改變** | `repo:71` | `Trepo:181` | asserts-oracle | produces-oracle | ✅ |
| AC-44 | 把名稱改成空白 | 修改被拒絕；名稱維持原狀 | `svc:78`→`dom:45` | `Tapp:212` | asserts-oracle | produces-oracle | ✅ |
| AC-45 | 把計算根數改成零 | 修改被拒絕；根數維持原狀 | `svc:78`→`dom:68` | `Tapp:212` | asserts-oracle | produces-oracle | ✅ |
| AC-46 | 修改一個從未存在過的識別碼 | 回覆找不到；**不會因此建立一支新的策略** | `repo:47`→`repo:85` | `Trepo:196` | asserts-oracle | produces-oracle | ✅ |
| AC-47 | 建立時間不因修改而改變 | 建立時間與修改前完全相同；最後修改時間不早於建立時間 | `repo:17`（只寫五個欄位） | `Trepo:147` | asserts-oracle | produces-oracle | ✅ |

### US-09 把不要的策略丟掉

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-48 | 刪除一支存在的策略 | 刪除成功；之後讀取回覆找不到；清單中不再出現 | `repo:116` | `Trepo:207` | asserts-oracle | produces-oracle | ✅ |
| AC-49 | 刪除一個從未存在過的識別碼 | 回覆找不到該策略 | `repo:116`（0 列受影響） | `Trepo:237`、`Tctl:323`(404) | asserts-oracle | produces-oracle | ✅ |
| AC-50 | 重複刪除同一支策略 | 回覆找不到該策略 | `repo:116` | `Trepo:230` | asserts-oracle | produces-oracle | ✅ |
| AC-51 | 刪除不波及其他策略 | 另一支仍讀得到，內容完全沒有改變 | `repo:116`（依主鍵刪一列） | `Trepo:222` | asserts-oracle | produces-oracle | ✅ |

### US-10 半成品也存得住

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-52 | 指標算式根本無法解讀 | 建立成功 | `dom:54`（只看有沒有，不看內容） | `Tdom:239`、`Tapp:125` | asserts-oracle | produces-oracle | ✅ |
| AC-53 | 指標算式與宣告的指標值種類對不上 | 建立成功 | `dom:41`（完全不比對兩者） | `Tdom:252` | asserts-oracle | produces-oracle | ✅ |
| AC-54 | 建立策略不會去讀 K 線，也不會算出任何指標值 | 系統沒有讀取任何 K 線、沒有算出任何指標值 | `svc:14`、`svc:20`——`StrategyService` 的建構子**不接受**任何 K 線來源 | 無（見下方說明） | no-test | produces-oracle | 🟡 partial |

> **AC-54 為何維持 partial。** 這一條由**編譯器**保證而非由測試保證：`StrategyService` 手上沒有
> `IKCandleRepository`，也沒有任何腳本執行器，所以「去讀 K 線」這件事寫不出來。
> 一個斷言「沒有呼叫某個不存在的相依」的測試，斷言的是自己憑空造出來的東西，
> 比型別簽章更弱也更容易腐爛。刻意留為 partial，並在 `ARCH.md` 記為「由依賴圖保證，不是靠自律」。

---

## Business Rules（PRD Section 4）

| ID | Rule | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- |
| BR-01 | 策略是食譜，不是結果——算出來的值不存在策略身上 | `ent:14`（欄位中沒有任何指標值） | `Tent:11` | ✅ |
| BR-02 | 名稱是身分：去空白後非空、≤128 字、不與別支相同；大小寫不同即不同；改回自己不算重複 | `dom:44`、`ent:20` | `Tdom:86`、`Trepo:38`、`Trepo:49`、`Trepo:60`、`Trepo:166` | ✅ |
| BR-03 | 未指定就套預設，與既有的彙總查詢、指標計算完全一致 | `dom:57`、`dom:62`（呼叫既有的兩個建構子，不另列清單） | `Tdom:44` | ✅ |
| BR-04 | 只認得五種彙總刻度 | `dom:57`→`selectableAggregationIntervals` | `Tdom:123`、`Tdom:145` | ✅ |
| BR-05 | 計算根數沿用單次查詢筆數上限，與刻度無關 | `dependencies.go`→`KCandleQueryMaxResults`→`svc:20`→`dom:72` | `Tdom:201`、`Tdom:214` | ✅ |
| BR-06 | 驗證是全有全無——任何一項不合法即整次拒絕，不留半成品、被改的策略維持原狀 | `dom:41`、`svc:32`、`svc:78` | `Tapp:160` | ✅ |
| BR-07 | 建立與修改的規則一字不差地相同 | `dto.StrategyWriteDto` 以 `ID` 區分，兩者共用 `dom:41` | `Tapp:160`（同一張表格同時跑建立與修改） | ✅ |
| BR-08 | 識別碼與建立時間不可更換；最後修改時間自動更新 | `repo:17`、`repo:47` | `Trepo:147` | ✅ |
| BR-09 | 刪除是真刪除，名稱隨之空出 | `repo:116` | `Trepo:61`、`Trepo:207` | ✅ |
| BR-10 | 存不等於跑——建立與修改都不執行也不檢查指標算式 | `dom:54` | `Tdom:239`、`Tdom:252` | ✅ |

**Edge cases（Section 4）**

| Edge case | Test | Status |
| :--- | :--- | :--- |
| 名稱長度以去除前後空白後計算 | `Tdom:86`（「　128 字　」合法） | ✅ |
| 修改時新舊名稱相同：撞到的是自己就放行 | `Trepo:166` | ✅ |
| 兩次建立同時送出同一個名稱，只有一支留得下來 | 由唯一索引保證（`ent:20`）；`Trepo:38` 驗單執行緒下的等價行為 | 🟢 by design |
| 一支都沒有時列出所有策略回空清單 | `Trepo:114`、`Tctl:182` | ✅ |

---

## Non-Functional（PRD Section 6）

| ID | Requirement | Evidence | Status |
| :--- | :--- | :--- | :--- |
| NFR-01 | 讀全部一次回傳，不分頁不快取 | `repo:100`（單次 `Find`） | ✅ |
| NFR-02 | 建立與修改不觸發任何 K 線讀取或指標計算 | `svc:14`——服務未持有任何 K 線相依 | 🟡 見 AC-54 |
| NFR-03 | 指標算式只被當成文字保存，不執行、不解讀 | `dom:54`、`ent:22`（`type:text`） | ✅ |
| NFR-04 | 既有的 K 線查詢、彙總查詢、交易標的清單與指標計算行為完全不變 | 既有測試全數通過；`aggregation_interval_domain` 與 `indicator_result_type_domain` 的重構在服務邊界維持相同哨兵與訊息（`k_candle_series_query_domain_test.go:112`、`indicator_calculation_domain_test.go:199`） | ✅ |

---

## Orphans

| Behavior | Clause? | 判定 |
| :--- | :--- | :--- |
| `GET/PUT/DELETE /strategies/:id` 對非正整數識別碼回 400（`ctl:115`） | PRD 未寫成 scenario | **非違規**：屬於「找不到與不合法要分得開」（Section 5）的實作面延伸，且不觸及任何 Out of Scope 項目。已由 `Tctl:227` 覆蓋。建議日後補一條 scenario。 |
| 儲存層無法連線時回 502（`ctl:128` 最後一段） | PRD 未寫成 scenario | **非違規**：沿用專案既有的錯誤對映慣例（`k_candle_controller.go:189`）。已由 `Tctl:154`、`Trepo:245` 覆蓋。 |
| `writeFailureOf`（`repo:71`） | 內部細節 | 非公開行為，不計入。 |

**Out of Scope 檢查**：PRD 列出的 12 項全部確認**未被實作**——
沒有任何程式碼會拿策略去算、不碰指標計算（`IndicatorCalculationService` 本切片零改動）、
策略不持有交易標的、不存執行紀錄、無版本歷史／軟刪除／擁有者／分類／分頁，
建立與修改也不檢查算式。**無 scope creep。**

---

## 稽核後的補強

初次稽核判為 5 條 🟠（測試綠但斷言比 oracle 弱）與 4 條 🟡（無測試）。
以下九條已補強，每一條都反向驗證過（破壞行為 → 該測試轉紅 → 還原）：

| ID | 原判定 | 缺什麼 | 補強 | 反向驗證 |
| :--- | :--- | :--- | :--- | :--- |
| AC-12 | 🟠 shallow | 只驗「被拒絕」，沒驗「既有那一支完全不受影響」 | `Trepo:38` 追加清單長度必須仍為 1 | 讓衝突不再被翻譯 → 紅 |
| AC-13 | 🟡 partial | 去空白與索引各自有測試，合起來沒有 | 見下方〈第二次 review 後的修正〉——初次補的 `Trepo:49` 是**無效測試**，已改由 `Tapp:105` 接手 | 不去空白 → 紅 |
| AC-18 | 🟡 partial | `1d` 從未被策略層測過 | 新增 `Tdom:123`：五種刻度逐一驗收 | 任一刻度被拒 → 紅 |
| AC-39 | 🟠 shallow | 清單只驗了名稱與一個根數 | `Tapp:257` 逐支驗七個欄位 | 清單掉欄位 → 紅 |
| AC-40 | 🟠 shallow | 只驗 UpdatedAt 不早於 CreatedAt，沒驗「已更新」 | `Trepo:147` 改為必須晚於原本的 UpdatedAt | UpdatedAt 設為只在建立時寫 → 紅 |
| AC-43 | 🟠 shallow | 沒驗「兩支都維持原狀」 | `Trepo:181` 追加兩支名稱皆未變 | 改名部分套用 → 紅 |
| AC-46 | 🟠 shallow | 沒驗「不會因此建立一支新的」 | `Trepo:196` 追加清單必須為空 | 改為找不到就建立 → 紅 |
| AC-53 | 🟡 partial | 只測過「算式無法解讀」，沒測過「形狀與宣告不符」 | 新增 `Tdom:252` | 存檔時比對形狀 → 紅 |
| AC-54 | 🟡 partial | 無法以測試表達 | **維持 partial**，理由見上 | — |

---

## 第二次 review 後的修正

PR 開出後又對整條分支（含前一次的修正 commit）跑了一次 review，找到五項，四項已修：

| # | 問題 | 處置 |
| :--- | :--- | :--- |
| 1 | **`Trepo:49` 是無效測試。** 它在測試裡自己 `strings.TrimSpace`，等於把同一個字面值存了兩次，與它上面那個測試完全一樣。去掉 `StrategyDomain` 的去空白它照樣綠。而 CONTRACT 曾把它當成 AC-13 的證據——**等於報了一個假的通過**。 | 刪除該測試（那一層證明不了，去空白住在 domain）。AC-13 改由兩半接手：`Tapp:105` 證「兩種寫法到達儲存層時是同一個名稱」，`Trepo:38` 證「同一個名稱兩次即衝突」。 |
| 2 | 識別碼落在 `[2^63, 2^64)` 時通過驗證、打到資料庫、以編碼錯誤回 **502** 而非 404 | `readID` 加上 `id > math.MaxInt64` 一併拒絕；測試補上 `9223372036854775808` |
| 3 | **PRD §4 流程圖把「存不存在」排在「內容合不合法」之前，實作相反。** 於是 `PUT /strategies/999999` 帶壞內容回 400「計算根數必須大於零」——正是 §5「找不到與不合法要分得開」禁止的混為一談 | 依規格修**程式碼**：`UpdateStrategy` 先確認策略在不在，再判內容 |
| 4 | 404 的訊息是內部英文哨兵字樣（`strategy not found`），而 400/409 都是給使用者看的中文 | repository 一律包成「找不到識別碼為 N 的策略」 |
| 5 | 儲存測試在未設 `TEST_POSTGRES_DSN` 時**靜默跳過**，而名稱衝突→409 這條路徑沒有別處覆蓋 | **未處理**——專案已備有 `make test-storage` 與 README 章節；要不要建 CI／讓缺 DSN 時直接失敗屬於開發流程決策，留給人決定 |

四項修正皆經反向驗證（破壞行為 → 對應測試轉紅 → 還原）。

## Verdict

**53 / 54 條符合（98.1%）。0 violation、0 gap、0 mis-asserted、0 orphan 違規。**
唯一未達 ✅ 的 AC-54 屬「由型別保證、測試無從加強」，為刻意接受的判定。
