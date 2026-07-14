# 对话分组宽版图表画布执行计划

> status: completed
> createdAt: 2026-07-15
> implementationAuthorized: true
> decision: local Workbench UI refinement; no ADR required unless implementation expands a service or contract boundary
> dependsOn: `three_pane_workbench_execution_plan.md`、`chart_trio_ui_fit_plan.md`、`current_codebase_overview.md`

## 1. 目标

Workbench 分析画布按 Task/对话轮次组织图表。分组顺序与左侧对话一致，旧分析在上、最新分析追加到底部；同一
Task 的图表保持紧凑，Task 之间使用更明显的间距和浅分隔线。图表在桌面画布中最多两列，实际空间不足时降为
单列，避免当前 `minColumnWidth.xl=21` 允许三至四个窄列产生的“瘦长型”图表。

```text
分析请求 · 10:30 · 1 张图
最近 1 分钟 CPU 变化
┌──────────────────────────────────────────────┐
│                    CPU                       │
└──────────────────────────────────────────────┘


分析请求 · 10:32 · 3 张图
CPU、内存和系统负载
┌──────────────────────┐ ┌──────────────────────┐
│         CPU          │ │        Memory        │
└──────────────────────┘ └──────────────────────┘
┌──────────────────────────────────────────────┐
│                    Load                      │
└──────────────────────────────────────────────┘
```

## 2. 范围与非目标

本切片包含：

- 从现有 Task、User Message 和 runtime charts 派生画布分组；
- 组级标题、时间、图表数量、间距和分隔；
- 最多两列、奇数尾图跨整行、窄画布单列的响应式布局；
- 新 Task 首图定位、历史恢复默认选择和加载更早记录时的滚动位置保护；
- 前端单元测试、Mock browser E2E 和当前实现文档更新。

本切片不包含：AI Core、assistant-mcp、Plugin Backend、OpenAPI/Schema、SQLite、TaskEvent、图表数据/单位、
PromQL、查询参数、图表高度、分组折叠、虚拟列表、筛选或新依赖。自然语言输入计划中的范围/resolution 控件删除
不在本切片执行，避免两个 UI 计划混在同一提交。

## 3. 已锁定的行为

### 3.1 分组模型和顺序

Workbench 只从已有 state 派生内部 UI 模型，不复制后端 DTO：

```ts
type ChartGroup = {
  taskId: string;
  createdAt: string;
  prompt: string;
  charts: WorkbenchChart[];
};
```

- reducer 的 `taskOrder` 继续保持 newest-first，供 active Task、恢复和现有上下文逻辑使用；画布派生分组时反转为
  oldest-first，不修改全局状态语义。
- 只为已有至少一张 Chart 的 Task 渲染分组；查询中但尚无 Chart 的 Task 不创建空壳 section。
- `prompt` 优先用 `Task.inputMessageId` 查找 User Message；找不到时再按 `taskId + role=user` 查找，仍缺失时使用
  “分析请求”，不得显示 `undefined`。
- 组标题显示用户问题（最多两行，完整内容放入 title/可访问文本）、本地化创建时间和当前图表数量。
- 画布总计改为“X 轮分析 · Y 张图表”。
- Chart 在组内保持现有 durable event/replay 形成的顺序；不在 UI 按标题猜测 CPU/内存/load 顺序。

### 3.2 选择与滚动

- 现有 `selectedChartId` 和 ContextPane 关联保持不变；选中查找从 groups 展平，仍用 `taskId + chartId` 保证归属。
- 新 Task 的第一张 Chart 到达时，自动选中该图并将所属分组滚动到可见区域；同一 Task 后续 Chart 到达不得重复
  抢焦点或触发滚动。
- 用 ref 记录最后一次自动聚焦的 taskId，普通 Task 状态、execution 或窗口 resize 更新不得改变用户选择。
- 首次历史恢复等待当前已加载 Task 的有限 replay 完成后，默认选择最新非空分组的第一张图，并无动画定位一次；
  replay 并发抵达时不得逐组跳动。
- 加载更早 Task 前记录 ScrollContainer 的 `scrollTop/scrollHeight`；旧分组插入顶部后以高度差补偿 scrollTop，保持
  用户正在查看的内容位置。
- 图表选择按钮仍是唯一的手动选择入口；分组 header 不新增折叠或选择行为。

### 3.3 组内网格

- 每组使用独立 CSS Grid，组内 gap 使用 Grafana spacing token 2，组间使用 token 4，并在非首组顶部添加 weak
  border 与 padding。
- 默认一列。只有页面达到桌面三栏断点且组内容容器实际宽度至少约 736px（两张约 360px 图加间距）时启用两列。
- 使用 theme-aware Emotion/useStyles2 和 CSS container query 同时约束 viewport 与实际画布宽度；不得只按 viewport
  强制两列，因为三栏刚展开时中心画布仍可能过窄。
- 两列是硬上限；超宽屏不得恢复三列/四列。
- 当组内 Chart 数量为奇数时，最后一个 wrapper 使用 `grid-column: 1 / -1`。因此 1 张整行、2 张两列、3 张为
  `2 + 1 full-width`、4 张为 `2 × 2`；单列模式自然退化为逐张整行。
- ChartCard 的 plot 高度继续为 260px，现有 ResizeObserver 继续以卡片实际宽度驱动 TimeSeries；不增加固定宽度。
- 卡片、wrapper、grid 和 PromQL 内容继续保持 `min-width:0`/可换行，页面不得产生水平溢出。

## 4. 实施阶段

### G0：计划激活

- 用户授权执行后，将本计划和进度记录从 `draft-review` 改为 `active`。
- 更新 `docs/CLAUDE.md` 和 implementation README 的状态；不修改已完成的历史计划正文。
- 在 current overview 中注明本计划将取代 `chart_trio_ui_fit_plan.md` 的当前“三图自由多列”布局，但历史文件仍保留
  原提交证据。
- 验证：`git diff --check`。

### G1：分组派生与画布结构

- 在 Workbench 建立纯派生 helper，将 Task、Message、runtime charts 转为 oldest-first ChartGroup；保留扁平 selected
  Chart lookup。
- ChartCanvas 入参改为 groups，并渲染语义化 `section`、稳定的 `data-testid="chart-group"` 和 task 标识。
- 增加分组 header、总轮次/图表数和组间视觉层级。
- 为分组 helper 增加单元测试：顺序、归属、prompt fallback、空 Task、增量 Chart 和默认最新选择。
- 验证：`make test-frontend`。

### G2：响应式宽版布局与滚动

- 实现一列/两列 container-aware 样式和奇数尾图跨行。
- 用 ScrollContainer ref 实现新 Task 首图单次定位、恢复后一次定位和 prepend scroll compensation。
- 保留 ChartCard 260px plot、高度状态和 ContextPane 选择行为。
- 验证：前端 unit/typecheck/build 与定向浏览器 E2E。

### G3：端到端与文档收口

- Browser E2E 连续提交两轮请求，检查分组、正序、几何关系、选择、刷新 replay 和无横向溢出。
- 更新 current codebase overview、current code tree、本进度记录和必要的 runbook 说明。
- 将计划/进度改为 completed，记录实际命令证据。
- 验证：`make test-frontend`、frontend build、`make e2e-mock`、`make check`。

## 5. 测试与验收标准

### 5.1 单元测试

- 输入 newest-first Task 时输出 oldest-first 非空分组。
- 同一 taskId 的所有 Chart 只存在于同一 group，下一 Task 不会混入上一组。
- Message 映射使用 inputMessageId；历史缺失时 fallback 稳定。
- 新 Chart 加入已有 Task 只更新该组，不改变分组顺序。
- 默认选择指向最新非空分组，而不是页面顶部最旧分组。
- 同一 Task 多张图只产生一次自动聚焦意图。

### 5.2 Browser E2E

连续完成两轮分析后：

- 存在两个 `chart-group`，第一轮在第二轮上方，header 与各自 User Message 对应；
- 组内相邻 Chart 的距离小于两组之间的垂直距离；
- 桌面宽画布每组最多两个不同 x 坐标，不再出现三列/四列窄图；
- 三张图时第三张位于下一行，其宽度接近前两列合计宽度；
- 页面窄屏或中心容器不足 736px 时所有图表单列；
- 每个 plot 的 bounding box 位于所属 Card 内，页面 `scrollWidth <= clientWidth`；
- 新一轮首图出现后该组可见、被选中图和 ContextPane 均属于最新 Task；
- 用户手动选择旧图后，普通 execution/status 更新不把选择切回最新图；
- 刷新和 SSE replay 后分组正序、归属和最新默认选择不变；
- 加载更早记录后当前可见内容的 viewport 位置基本不变；
- 展开 PromQL 不撑破 group grid。

## 6. 预计提交

1. `docs: plan grouped chart canvas`
2. `refactor(frontend): group and widen analysis charts`
3. `test(frontend): verify grouped chart layout`
4. `docs: complete grouped chart canvas`

每个提交只包含一个独立可验证切片；不 push、不建 PR、不 amend 或改写历史。
