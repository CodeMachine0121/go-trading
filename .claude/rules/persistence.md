# Persistence — Module & Schema Sync

## 一律 Code First

**schema 的單一來源永遠是程式碼**（`domain/models/entities/` 的 entity 定義），不是資料庫。

- 先寫 entity，再由 entity 產生資料庫結構。
- **禁止 Database First**：不從既有 DB 反向產生模型，不手動改 DB 再回頭補程式碼。
- entity 是**乾淨的 Data Model**：只有欄位與持久化標註，沒有業務邏輯（行為在 Domain Model，見 [architecture.md](architecture.md)）。

## Schema 同步一律交給 ORM

資料庫結構的同步（sync / migration）**一律使用 ORM library 的機制控管**，不手動維護 DDL。

- 由 ORM 的 migration / auto-migrate 機制產生與套用結構變更（EF Core Migrations、GORM `AutoMigrate`、Prisma Migrate、TypeORM Migrations…）。
- **不手寫 DDL 腳本**、不在 DB 端直接改結構。
- migration 檔（若該 ORM 產生）視為程式碼的一部分，隨 commit 進版控。

## 禁止手寫 SQL 字串

**資料存取一律走 ORM 的型別安全 API。** 程式碼中不得出現拼接的 SQL 字串。

- 效能瓶頸等真的必須下原生查詢時，需明確標註原因，並走 ORM 提供的參數化查詢介面，絕不字串拼接參數。

## 一個 entity 對應一個 repository

`XxxRecord` → `XxxRecordRepository`。**讀寫都在同一個 repository**，不另立切片型別（不拆 reader / writer）。

repository **介面**放 `domain/interface/`，**實作**放 infrastructure（見 [architecture.md](architecture.md)、[naming.md](naming.md)）。
