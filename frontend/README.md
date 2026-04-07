# Boxer Frontend

Vue 3 + Vue Flow + TypeScript 視覺化流程編輯器。

## 開發

```bash
npm install
npm run dev      # → http://localhost:5173
npm run build    # 生產建置
```

搭配 codegen service 使用時，需先啟動後端：

```bash
cd ../codegen
go run ./cmd/main.go serve
# → http://localhost:8080（前端 vite 自動 proxy /api/codegen）
```

## 使用方式

### 流程設計

1. 從左側 **Node Palette** 拖拉節點到畫布
2. 連接節點（自動驗證連線合法性，不合法的連線會被拒絕）
3. 點擊節點在右側 **ConfigPanel** 編輯屬性
   - 所有欄位依節點類型動態顯示
   - 支援 retry/fallback 開關、upstream provider 選擇等進階設定

### 測試執行

4. 點擊 **▶ Test** 開啟測試面板
   - 輸入 Mock Params（JSON，如 `{ "userId": "42" }`）
   - 輸入 Mock Upstreams（JSON，upstream name → mock response）
   - 點擊 **Run** 執行流程
   - 查看 Response（status code + body）和 Trace（每個節點的狀態/耗時）
   - 執行中的節點會在畫布上高亮顯示（綠色=成功、紅色=失敗、黃色脈衝=執行中）
   - 點擊節點可在 ConfigPanel 底部查看該節點的輸出

### 匯出 / 匯入

5. **Export IR** — 匯出 IR JSON 到剪貼簿
6. **Import IR** — 開啟 modal 貼入 IR JSON 載入流程（支援大段 JSON）

### 程式碼生成

7. **⚙ Go** — 呼叫 codegen service 生成 Go HTTP handler，可預覽並下載
8. **⚙ Lua** — 呼叫 codegen service 生成 Kong Plugin Lua，可預覽並下載

> ⚙ Go / ⚙ Lua 需要 codegen service 運行中（見上方「開發」章節）
