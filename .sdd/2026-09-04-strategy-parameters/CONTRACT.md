# 策略自己的旋鈕 — Contract Verification Matrix

**Oracle:** `PRD.md` 第 3 節的 Gherkin 驗收條件（23 條）
**Oracle 紀錄:** 實作前寫於 scratchpad `oracle-strategy-parameters.md`（獨立性關卡的證據）
**判定方式:** 靜態一致性稽核——測試對照 oracle、程式碼對照 oracle，兩邊**各自獨立**判定；
不以「測試跑綠」作為結論。

---

## 1. Clauses

| ID | 條款 | Oracle（實作前寫下） | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-01.1 | 帶著參數的策略被完整記住 | 兩個參數**連同種類與預設值**都被記住 | `entities/strategy.go` 的一對多 ＋ `strategy_repository.go:57`（`Create` 連帶存入） | `strategy_repository_test.go:保留策略帶著的旋鈕` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.2 | 讀回來時參數原樣還在 | 名稱、種類、預設值原樣回來 | `strategy_repository.go:145`（`Preload`） | 同上 ＋ `列出每一支策略與它的旋鈕` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.3 | 一個參數都不宣告照常運作 | 建立成功；計算與沒有這項功能時完全相同 | `strategy_parameters_domain.go:45`（空的一份是合法的）；進入點形狀未改 | `一個參數都不宣告是合法的一份`、`一支不讀任何參數的算式一如既往`，加上**既有 13 個套件全數維持綠燈** | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.4 | 參數名稱不得重複 | 拒絕，說明不得重複 | `strategy_parameters_domain.go:60` | `同一支策略內名稱不得重複` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.5 | 參數名稱不得為空白 | 拒絕，說明不得為空白 | `strategy_parameters_domain.go:186` | `名稱不得為空白` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.6 | 前後空白不予保留 | 記住的是去掉空白之後的名字 | `strategy_parameters_domain.go:184`（`TrimSpace`） | `名稱前後的空白不予保留` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.7 | 回看根數必須大於零 | 拒絕，說明必須大於零 | `strategy_parameters_domain.go:224` | `回看根數必須大於零`、`回看根數必須是整數` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.8 | 參數種類只有兩種 | 拒絕，說明只有那兩種 | `strategy_parameter_kind_domain.go:38` | `種類只有那兩種` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.9 | 數值不限正負與小數 | 建立成功 | `strategy_parameters_domain.go:222`（非回看根數即不判斷） | `數值不限正負與小數——系統本來就不解讀它` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.1 | 要看的每一格都有值 | 12 格 ＋ 回看 20 → 拿 **31** 根 | `indicator_calculation_domain.go:87` | `要看 12 格、最大回看 20 → 拿 31 根` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.2 | 沒有回看根數就是那一段的根數 | 拿 **12** 根 | 同上（`max(0, L−1)`） | `一個回看根數都沒有 → 就是要看的那幾格，不多不少` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.3 | 好幾個回看根數只看最大的 | 拿 **111** 根 | `strategy_parameters_domain.go:110` | `好幾個回看根數只看最大的` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.4 | 只看一格也拿滿回看所需 | 拿 **100** 根 | `indicator_calculation_domain.go:87` | `只看一格也拿滿回看所需` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.5 | 超過上限整次拒絕 | 整次拒絕；不回傳部分結果 | `indicator_calculation_domain.go:96`（對象是**推導後**的根數） | `上限判斷的對象是真的要餵進去的根數` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.1 | 回看根數取出來是可以數根數的整數 | 取到 20，且是整數 | `yaegi_indicator_script_proxy.go:161`（注入的函式回 `int`） | `一支算式以名字取用它的參數`（真的跑一段算式，用它去切片） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.2 | 這一次給的值蓋過預設值 | 取到 2.5 | `strategy_parameters_domain.go:76`（`Applying`） | 同上 ＋ `給了值就用給的` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.3 | 取用沒宣告的名字就是這次失敗 | 這一次計算失敗，說明是哪一個名字 | `yaegi_indicator_script_proxy.go:120`（記錄器優先） | `取用沒有人宣告的參數就怪那個名字`、`兩種讀法都要擋` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.4 | 失敗說的是名字對不上，不是算式壞了 | 指出哪一個名字；**沒有**說算式有問題 | 同上（自己的哨兵錯誤 `ErrIndicatorParameterNotDeclared`） | 同上（**明確斷言 `NotErrorIs` 算式執行失敗**） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.1 | 給了值就用給的 | 用 50 | `strategy_parameters_domain.go:88` | `給了值就用給的` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.2 | 沒給值就用宣告的預設值 | 用 20 | 同上（沒給就不覆蓋） | `沒給值就用宣告的預設值` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.3 | 給了沒宣告的名稱整次拒絕 | 整次拒絕，說明那不是這支策略的參數 | `strategy_parameters_domain.go:80` | `給了沒有宣告的名稱就整次拒絕` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.4 | 給的回看根數不合法整次拒絕 | 整次拒絕，說明必須大於零 | `strategy_parameters_domain.go:93`（套值後再驗一次） | `給的回看根數不合法就整次拒絕` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.5 | 給了沒人取用的值不是錯誤 | 照常執行 | **靠不做那件事達成**——沒有任何地方去比對算式用了哪些名字 | `沒有人取用的參數值不是錯誤` | asserts-oracle | produces-oracle | ✅ conforms |

---

## 2. Business Rules / NFR

| ID | 條款 | 覆蓋情形 |
|---|---|---|
| BR-1 | 一支策略記著它有哪些參數 | AC-01.1／01.2 ✅ |
| BR-2 | 種類只有兩種 | AC-01.8 ✅ |
| BR-3 | 名稱不得重複、不得為空白、前後空白不予保留 | AC-01.4／01.5／01.6 ✅ |
| BR-4 | 要拿幾根 ＝ 那一段的根數 ＋ 最大回看根數 − 1 | AC-02.\*，**含 `L=0` 與 `L=1` 兩個邊界** ✅ |
| BR-5 | 沒給值用預設值；給了沒宣告的就拒絕 | AC-04.2／04.3 ✅ |
| BR-6 | 算式取用沒宣告的名字＝這次失敗 | AC-03.3／03.4 ✅ |
| BR-7 | 零個參數的策略行為完全不變 | AC-01.3 ✅ |
| BR-8 | 參數數量不設上限 | 程式碼無任何數量判斷 ✅ |
| NFR-1 | 參數不改變計算的成本結構 | 要拿的根數仍受單次查詢上限約束（AC-02.5）✅ |

---

## 3. Orphans

| 行為 | 對應條款 | 判定 |
|---|---|---|
| 參數名稱長度上限 64 字 | PRD 未載明 | ⚠️ 未載明，但與策略名稱 128 字的既有規則同一類；已有測試 |
| 改寫策略時整份取代旗下的參數 | PRD 未載明（它只說「記住」與「讀回」） | ⚠️ 未載明，但**留著舊的會讓最大回看根數從一個看不見的參數算出來**。已補測試並以突變確認 |
| 建立時 `Create` 連帶存入參數 | 同上 | ⚠️ 同上 |

三項都**不落在 Out of Scope 清單內**，不構成範圍蔓延。建議日後補進 PRD。

---

## 4. Summary

```
✅ 23 conforms · 🔴 0 violations · 🟠 0 mis-asserted · 🟡 0 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 3 orphans
Conformance: 100%（23/23）
```

**實作過程中修掉的一個真錯誤（不是稽核發現的，是既有測試發現的）：**

`要拿幾根 = N + L − 1` 這條式子**只在 L ≥ 1 時成立**。沒有任何回看根數參數時 L=0，
它會算成 `N − 1`——**少拿一根**。ARCH §6 原本還宣稱「同一條式子涵蓋兩種情形，不需要分支」，
那句話是錯的，已就地更正並標明原文錯在哪。正確的式子是 `N + max(0, L−1)`。

**這次稽核補上的一處：**

| # | 問題 | 處置 |
|---|---|---|
| 1 | AC-01.1／01.2「參數被記住、讀回來原樣還在」**完全沒有測試** ——`Preload` 與整份取代這兩段程式碼一行都沒被驗證過 | 補上四條資料庫測試：存下讀回、列出時帶著、改寫時整份取代、可以把旗下參數全部拿掉 |

以突變測試確認：拿掉 `Preload`（兩處）、改寫時不動參數、只新增不刪除——四種寫法全部被擋下。

**覆蓋率（實測，`go tool cover`）：** 本切片新增的每一個函式都是 **100%**，
唯一的例外是 `StrategyParameter.TableName`（0%），與 `KCandle`／`TradingSymbol` 的同名方法一致——
它們只在需要資料庫的測試中被 GORM 呼叫，屬於既有狀況而非本切片造成。

**突變測試：** 14 個突變逐一確認會被對應測試擋下，無存活者。

**Ceiling:** 這是靜態一致性稽核——逐條比對測試斷言與程式碼路徑對規格的預期結果，
不執行自己發明的情境。AC-01.1／01.2 需要資料庫，**只對 `go_trading_test` 驗證**；
已確認共用開發資料庫 `go_trading` 上沒有出現這張新表。
