# 背景作業（Background Jobs）

## 配置規則

- 背景 job 集中放一個 `job/` 目錄，**一個 job 一個檔案**、`Job` 後綴。
- 每個 job 實作同一個 `IBackgroundJob` 介面。
- Job manager **只依賴 `IBackgroundJob` 的集合**，統一啟動；不認識任何具體 job。
- job 的註冊與組裝放**組裝根**，入口只負責呼叫「全部啟動」。
- 每個 job 的執行間隔由**設定 / 環境變數**控制；設 `0` 或負值即停用該 job。
- 提供**全域總開關**，關閉時所有背景 job 都不啟動。

## 管線輪次記錄（run tracking）

多步驟管線的每次步驟執行（不論由 job 或手動觸發）都應落地一筆 run 記錄：

- 狀態流轉：`running` → `succeeded` / `failed` / `noData`
- 記錄觸發來源（`job` / `manual`）
- 業務紀錄以 run id 關聯所屬輪次
- 同輪下游步驟以「上游 run id」串接上一步
- **run 寫入失敗即該步驟整體失敗**
- 服務啟動時把殘留的 `running` 掃成 `failed`（`interrupted by restart`）
