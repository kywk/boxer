# Boxer Frontend

Vue 3 + Vue Flow + TypeScript 視覺化流程編輯器。

## 開發

```bash
npm install
npm run dev      # → http://localhost:5173
npm run build    # 生產建置
```

## 使用方式

1. 從左側 Node Palette 拖拉節點到畫布
2. 連接節點（自動驗證連線合法性）
3. 點擊節點在右側 ConfigPanel 編輯屬性
4. 點擊 **▶ Test** 開啟測試面板
   - 輸入 Mock Params 和 Mock Upstreams（JSON）
   - 點擊 Run 執行流程
   - 查看 Response 和 Trace
5. 點擊 **Export IR** 匯出 IR JSON（複製到剪貼簿）
6. 點擊 **Import IR** 貼入 IR JSON 載入流程
