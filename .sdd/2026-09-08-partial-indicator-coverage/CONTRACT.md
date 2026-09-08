# Contract Traceability Matrix — 指標計算湊不滿也算得出來

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/models/domains/`、`internal/domain/service/`、`internal/controller/`
Oracle: Acceptance Criteria（22 個情境）＋ Core Business Rules（6 條）＋ Non-Functional（1 條）＝ 29 clauses

> 第一輪稽核找出 1 個 mis-asserted、1 個 partial 與 2 個 orphan，全部已處理：
> 補上兩個測試，並把兩個 orphan 收進 PRD（新增 US-04.5 兩個情境與一條 Edge Case）。本表為處理後的狀態。

> **審核天花板**：這是一次**靜態**契約稽核。它拿 PRD 的預期結果分別去對照測試斷言與程式路徑，
> **不執行自己發明的情境**。少數 clause 有跑過它自己對應的那一個測試作為佐證，
> 但判定一律來自與 oracle 的比對，不是來自紅綠。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01.1 | 可用根數湊得滿 | 算出 100 個值；回報實際採用 119、計算根數 119 | `indicator_calculation_domain.go:227` | `indicator_calculation_domain_test.go:443`（exactly as many as were asked for）＋`indicator_calculation_service_test.go:197` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.2 | 可用根數湊不滿，以可用根數執行 | 算式拿到最靠近截止時間的 50 根、由早到晚；回報採用 50、計算根數 119 | `indicator_calculation_domain.go:227` | `indicator_calculation_domain_test.go:443`（fewer than were asked for, so all of them）＋`indicator_calculation_service_test.go:176` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.3 | 可用根數剛好等於最少可算根數 | 算出 1 個值；回報採用 20 | `indicator_calculation_domain.go:220-221` | `indicator_calculation_domain_test.go:559` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.4 | 可用根數比最少可算根數少一根 | 整次拒絕，說出可用 19、最少 20；不回任何部分結果 | `indicator_calculation_domain.go:222` | `indicator_calculation_domain_test.go:510`（one bucket short of the declared look-back） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.5 | 沒有宣告回看根數時，一根就算得出來 | 算出 1 個值；回報採用 1、計算根數 100 | `indicator_calculation_domain.go:220`（`max(1, …)`） | `indicator_calculation_domain_test.go:443`（a single bucket against a wide request） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.6 | 一根都湊不出來 | 整次拒絕，說出可用 0、最少 1；不回任何部分結果 | `indicator_calculation_domain.go:222` | `indicator_calculation_domain_test.go:510`（no candles at all, with nothing declared） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.7 | 還在走的那一格照樣不算進可用根數 | 算式拿到的每一根都來自走完的刻度區間；10:00 那一格不在其中 | `indicator_calculation_domain.go:161`（`ReadCutoff`） | `indicator_calculation_domain_test.go:237`＋`indicator_calculation_service_test.go:422` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.1 | 湊得滿時兩個根數相同 | 回報計算根數 119、實際採用 119 | `indicator_calculation_service.go:109-110` | `indicator_calculation_service_test.go:197` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.2 | 湊不滿時兩個根數不同 | 回報計算根數 119、實際採用 50 | `indicator_calculation_service.go:109-110` | `indicator_calculation_service_test.go:176`＋`indicator_calculation_application_test.go:114` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.3 | 沒有回看根數時計算根數不多花 | 回報計算根數 100 | `indicator_calculation_domain.go:86`（`max(0, lookback-1)`） | `indicator_calculation_domain_test.go:640`（no look-back costs nothing extra） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.4 | 兩個根數與每一根的起始時間一起回報 | 回報 50 根各自的起始時間、由早到晚；個數等於實際採用根數 | `indicator_calculation_service.go:98`（與 `inputKCandleVos` 同源） | `indicator_calculation_service_test.go:176`（`assert.Len(OpenTimes, 1)`）＋`:476` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.1 | 湊不出最少可算根數是一種認得出來的拒絕 | 呼叫端辨認得出這一種；取得 19 與 20 兩個數字；不必解讀說明文字 | `indicator_calculation_errors.go:59,78,99`＋`indicator_calculation_controller.go:74` | `indicator_calculation_domain_test.go:510`（`CandleCoverageShortfall` 取值）＋`indicator_calculation_controller_test.go`（reports a stretch too thin，斷言 `availableCandleCount`/`minimumCandleCount`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.2 | 與超過上限那一種分得開 | 呼叫端辨認得出是「超過上限」；不被當成湊不出最少可算根數 | `indicator_calculation_controller.go:61` vs `:74`（兩個獨立哨兵、兩條分流） | `indicator_calculation_controller_test.go`（keeps asking for too much apart，斷言 `field` 在且 `availableCandleCount` **不**在） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.3 | 算式本身跑不動仍是算式的問題 | 呼叫端辨認得出是「算式的問題」；不被當成任何一種根數不足 | `indicator_calculation_controller.go:101` | `indicator_calculation_controller_test.go`（reports a script that cannot run as unprocessable）— 以 422 對 400 做區分 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.1 | 沒宣告卻在算式裡寫死期數 | 算式拿到那幾根；整次以「算式的問題」被拒絕；不被說成根數不足 | `indicator_calculation_domain.go:220`（只讀宣告）＋`indicator_calculation_controller.go:101` | `indicator_calculation_service_test.go`（an algorithm needing more than it declared fails as an algorithm）——斷言算式收到那三根、錯誤是算式失敗、且 `NotErrorIs` 湊不出最少可算根數 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.2 | 宣告與算式一致時最少可算根數就是對的 | 算出 1 個值 | `indicator_calculation_domain.go:220-221` | `indicator_calculation_domain_test.go:559` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.3 | 宣告了多個回看根數時取最大的那一個 | 最少可算根數為 60 | `indicator_calculation_domain.go:220`（`MaximumLookbackCount`） | `indicator_calculation_domain_test.go:573`（several declared look-backs take the hungriest，可用 59 → 拒絕並說出 60） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.1 | 計算根數超過單次可用的最大根數 | 整次拒絕，說出需要幾根與最大根數；提示縮短區間或改用粗一點的刻度 | `indicator_calculation_domain.go:94`＋`indicator_calculation_errors.go:39` | `indicator_calculation_domain_test.go:136`（more candles than a single call allows）＋`:825` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.2 | 計算根數剛好等於單次可用的最大根數 | 算得出指標值，不被拒絕 | `indicator_calculation_domain.go:94`（`>` 而非 `>=`） | `indicator_calculation_domain_test.go`（`TestTheCeilingAcceptsExactlyItsOwnLimit`）——上限那條線被拒絕的一側原本就有，這是被接受的一側 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.3 | 超過上限的判斷不因可用根數而改變 | 以「超過上限」被拒絕；不被說成湊不出最少可算根數 | `indicator_calculation_domain.go:94`（建構時就攔下，讀取從未發生） | `indicator_calculation_controller_test.go`（keeps asking for too much apart，未設任何 repository 期望即證明沒讀過） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 實際採用根數 = 計算根數與可用根數取較小者 | 兩者取小 | `indicator_calculation_domain.go:227` | `indicator_calculation_domain_test.go:443`（五個案例橫跨兩側） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 最少可算根數 = 最大回看根數；一個都沒宣告時為一根 | 取最大；地板為一 | `indicator_calculation_domain.go:220` | `indicator_calculation_domain_test.go:573`（兩案例：取最大、只宣告數值時為一） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 只認宣告出來的回看根數，算式額外要幾根由算式自己把關 | 系統不猜 | `indicator_calculation_domain.go:220` | `indicator_calculation_domain_test.go:725` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 兩種根數不足的出路正好相反，所以必須分得開 | 兩個獨立、分辨得出的拒絕 | `indicator_calculation_errors.go:59` vs `:12`；`indicator_calculation_controller.go:61,74` | `indicator_calculation_controller_test.go`（keeps asking for too much apart） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 還在走的那一格永遠不算進可用根數 | 排除 | `indicator_calculation_domain.go:161` | `indicator_calculation_domain_test.go:237` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 沒有任何 K 線的刻度區間不算一格，也不算進最少可算根數 | 六十小時中一小時無成交、回看 60 → 可用 59 → 拒絕 | `indicator_calculation_domain.go:204`（`KCandleSeriesDomain.Buckets()` 不補洞）＋`:221` | `indicator_calculation_domain_test.go`（`TestTheFloorCountsOnlyBucketsThatHoldSomething`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.5.1 | 助手拿到的結果帶著兩個根數 | 結果同時帶計算根數 119 與實際採用根數 50 | `indicator_calculation_assistant_query.go:120`（原樣轉交結果） | `indicator_calculation_service_test.go:176`（結果形狀）；助手端原樣轉交，無獨立斷言 | shallow | produces-oracle | 🟠 mis-asserted |
| AC-04.5.2 | 助手被告知湊不滿不是拒絕 | 說明寫出以手上有的計算，並要求助手講出讀數以較少行情算出 | `indicator_calculation_assistant_query.go:68`（`Description()`） | **無**——這段敘述沒有任何測試讀它 | no-test | produces-oracle | 🟡 partial |
| NFR-1 | 回報項目只增不減：既有呼叫端不看新增的計算根數也照樣運作 | 舊呼叫端不壞 | `indicator_calculation_result_dto.go`（新增欄位，無欄位被移除或改名） | `indicator_calculation_assistant_query.go` 原樣轉交結果，其既有測試未改動仍綠 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| — | 第一輪的兩個 orphan 都已收進契約：空刻度區間那條成為 `BR-6`，助手那段敘述成為 `US-04.5`（`AC-04.5.1`／`AC-04.5.2`） | 已解決 |

沒有任何一項落在 **Out of Scope** 上，因此本次沒有範圍外的違規。

## Summary

- Conforms: 27/29 clauses ✅（93%）
- Violations: 無
- Mis-asserted: `AC-04.5.1`（結果的形狀在服務層驗過了，助手端只是原樣轉交，沒有自己的斷言）
- Partial: `AC-04.5.2`（給助手讀的那段敘述沒有測試）
- Gaps: 無
- Unclear: 無
- Orphans: 0

### 為什麼留下那兩項

兩者都落在**「一段給 AI 讀的散文」**上。`AC-04.5.1` 的程式路徑是一行 `json.Marshal(resultDto)`，
它沒有自己的邏輯可以錯——真正會錯的是結果的形狀，而那已經在服務層被斷言。
`AC-04.5.2` 要驗的是一段敘述的**內容**：能寫的測試只有「這串字裡有這幾個字」，
它會在任何一次措辭改善時變紅，卻抓不到唯一真正的失效（敘述寫得對、助手仍然沒照做）。
兩者都刻意不補，並記在這裡，而不是假裝已覆蓋。
