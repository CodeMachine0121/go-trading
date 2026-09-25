# Contract Traceability Matrix — 跳空穿過止損的成交價（backtest-gapped-stop-fill）

Contract: PRD.md（`.sdd/2026-09-25-backtest-gapped-stop-fill/PRD.md` v1.0）
Design map: ARCH.md（§7 Traceability）
Implementation: branch `fix/backtest-gapped-stop-fill`
Oracle: Acceptance Criteria（17 AC ＋ 7 BR ＋ 1 NFR）

**Path abbreviations**
- Impl：`pos` = `internal/domain/models/domains/backtest_position_domain.go`；`cpos` = `internal/domain/models/domains/contract_backtest_position_domain.go`
- Tests：`GAP` = `internal/domain/models/domains/tests/backtest_gapped_stop_fill_test.go`；`EXIT` = `.../tests/backtest_exit_simulation_test.go`；`CBT` = `.../tests/contract_backtest_domain_test.go`

## Clauses

### US-01 現貨止損在跳空時成交在開盤價

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 開盤跳過止損價 | 止損出場、成交 90、虧 1000、帳上 9000 | pos:81 | GAP `an open below the stop fills at the open` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 開盤正好等於止損價 | 成交 95、虧 500 | pos:81 | GAP `an open exactly at the stop…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 盤中才碰到止損價 | 成交 95、虧 500 | pos:81 | GAP `a stop reached only within the bar…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 開盤跳過止損價的那一格也碰到止盈 | 止損出場、成交 90 | pos:78-83（止損分支先於止盈） | GAP `a gapped stop on a bar also reaching the take profit…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 止盈跳空不動 | 止盈出場、成交 105、賺 500 | pos:86-90（未改） | GAP `a take profit gapped through…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 沒有設止損 | 沒有任何一筆止損出場，那一注照舊抱著 | pos:78（`HasStopLoss` 為假） | GAP `without a stop a gap down is simply held` | asserts-oracle（無平倉、止損出場 0、帳上 8900） | produces-oracle | ✅ conforms |

### US-02 下一格開盤成交也一樣

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-7 | 進場那一格不會跳過自己的止損 | 以 100 開倉、同一格以 98 止損 | pos:81；開盤成交走法未改 | GAP `the bar opening the position cannot gap through its own stop` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 之後那一格開盤跳過止損價 | 以 95 止損、虧 500 | pos:81 | GAP `a later bar opening below the stop fills at its open` | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 合約止損在跳空時成交在開盤價

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-9 | 做多開盤跳過止損價 | 成交 90、虧 1000 | cpos:119-124 | GAP `a long opening below its stop fills at the open` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 做空開盤跳過止損價 | 成交 110、虧 1000 | cpos:119-124 | GAP `a short opening above its stop fills at the open` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 做空盤中才碰到止損價 | 成交 105 | cpos:137-140 | GAP `a short reaching its stop only within the bar…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 跳空的止損照舊套滑點 | 成交 89.91 | cpos:122-124（`exitFillFor(bucket.Open)`） | GAP `a gapped stop still pays the slippage on the open` | asserts-oracle | produces-oracle | ✅ conforms |

### US-04 合約開盤同時越過止損與強平價

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-13 | 開盤越過止損、還沒越過強平價 | 止損、成交 93、虧 7000、帳上 3000 | cpos:119-124 | GAP `an open past the stop but short of liquidation…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 開盤標記價格越過強平價，即使止損比較近 | 強平、只輸 10000、帳上 0 | cpos:112-117 | GAP `a mark open past liquidation liquidates even though the stop is nearer`（另有做空一例） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 開盤成交價越過強平價、標記價格還沒 | 止損、成交 89、只輸 10000、帳上 0 | cpos:119-124 ＋ cpos:217（零下限） | GAP `a traded open past liquidation with the mark short of it…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 開盤兩個都沒越過，盤中強平價比較近 | 強平 | cpos:142-144 | GAP `an open past neither leaves the nearer liquidation price…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 開盤越過比較遠的止損與強平價 | 強平、只輸 10000 | cpos:112-117 | GAP `a mark open past both a farther stop and liquidation liquidates` | asserts-oracle | produces-oracle | ✅ conforms |

### Business rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | 跳空止損成交價：做多取低、做空取高 | 同 AC-1、AC-10 | pos:81、cpos:119-124 | GAP（AC-1、AC-9、AC-10） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 有沒有碰到止損的判定不變 | 低點（做空高點）碰到才算 | pos:78-79、cpos:127-129 | EXIT `TreatsTouchingTheLevelAsReachingIt`、CBT `TestContractBacktestStopAgainstLiquidation` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 合約不利側順序：標記開盤越過強平 → 成交開盤越過止損 → 盤中近者先 → 止盈 | 同 AC-13–AC-17 | cpos:112-151 | GAP US-04 表格、CBT `TestContractBacktestStopAgainstLiquidation` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 虧損超過保證金時最多只輸保證金 | 收回零 | cpos:217 | GAP AC-15、CBT `gapping far past the liquidation price` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 止盈成交價不變 | 同 AC-5 | pos:86-90、cpos:146-151 | GAP AC-5、EXIT `ClosesALongAtItsTarget` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 滑點大於止損距離時，止損在開盤那一刻觸發、以開盤價成交 | 以 100 的開盤價再扣 1% ＝ 99 成交、止損出場 | cpos:119-124 | GAP `a stop the entry slippage already put above the open…` | asserts-oracle | produces-oracle | ✅ conforms（首輪稽核為 🟡 partial，補測後轉正） |
| BR-7 | 現貨只有做多那一半 | 現貨出場永遠取低 | pos:81 | GAP 現貨表格 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 舊成績單不可比較、不提供切回開關 | 沒有切換選項；交付說明寫明 | 無新參數；README、postman 已改寫範例數字 | —（文件性質） | n/a | produces-oracle | ✅ conforms |

## Orphans

| Site | Behaviour | Classification |
|------|-----------|----------------|
| `CBT` 測試夾具的標記開盤改為可指定、預設取開盤價 | 測試資料形狀，不是產品行為 | 非產品行為，無需對齊 |

## Summary

✅ 25 conforms · 🔴 0 violations · 🟠 0 mis-asserted · 🟡 0 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 0 product orphans
Conformance: 100%

Ceiling: static conformance audit against the Acceptance Criteria — it judges test assertions and code paths against the spec's expected outcome; it does not execute invented scenarios.
