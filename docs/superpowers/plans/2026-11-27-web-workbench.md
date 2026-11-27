# Web 工作台实施计划

**Goal:** 交付地图、对话与工具记录共存的单程序 Web 工作台。

**Architecture:** `web/src` React 组件复用 Go API，`web/assets.go` 嵌入 `web/dist`，HTTP 对静态资源与 API 分流。纯函数承担几何显示及参数处理。

**Tech Stack:** React、TypeScript、Vite、MapLibre GL JS、Vitest、Testing Library、Go embed。

- [x] 写 `web/src/lib.test.ts`：半径圆闭环、围栏请求/退化输入、结构化 API 错误；运行 `npm test` 确认缺失实现，再实现 `lib.ts` 和 API 客户端。
- [x] 写 `web/src/App.test.tsx`：演示状态、确定性检索、结果来源、Agent 禁用、请求失败与取消；先失败再实现 App、MapPanel、ResultsPanel 和样式。
- [x] 写 Go 工作台资源测试：`GET /`、静态资源、未知路径与未知 API 404、POST 不作为静态页面；再实现嵌入与路由。
- [x] 构建前端产物并嵌入；更新开发/单程序使用说明、CI 构建前端与测试。
- [x] 执行前端测试/build、Go race/vet/build、真实浏览器桌面/移动端交互验证；修复实际问题，记录外部底图或模型未验证项。

沿用已有开发分支，不修改用户配置或提交。产物随源码保留，`npm ci && npm run build` 可重建。

## 验证记录

- 前端组件与空间参数测试通过，涵盖取消迟到响应、Agent 参数/耗时同步、日期变更线及 WebGL 降级视图。
- npm 生产构建、格式检查与依赖审计通过。MapLibre 6 worker 明确单独构建并嵌入。
- Go race/vet/build 通过，静态资源测试覆盖所有生成的 JS/CSS 与 worker 文件。
- 实际浏览器验证 1440×900 桌面和 390×844 移动视口、围栏查询、列表定位、执行记录展开、底图开关及 GeoJSON 下载；导出文件解析为两个 demo POI。临时视口已恢复。
- 独立审查指出日期变更线连线、非法纬度定位和降级视图中心不同步，三项均用先失败后通过的测试修正。
- 未接入真实模型提供商；本轮未配置 TEST_DATABASE_URL，PostGIS 集成测试跳过；CI 提供独立数据库服务。
