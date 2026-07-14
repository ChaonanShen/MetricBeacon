# 三图归位（Chart Trio UI Fit）计划

status: completed
createdAt: 2026-07-14
completedAt: 2026-07-14
sourceCommit: 18037fc
visualBaseline: `current_ui.png`

## 1. 目标

只修正 Workbench 中 CPU 使用率、内存可用率、系统负载三张时序图的展示 UI，使 mock
数据在 Grafana App 页面中以稳定、可读、可响应的方式呈现。

完成后的直接效果应是：

- 三张图在宽屏下一行等宽排列，在较窄窗口中自然换为两列或一列；
- 卡片标题、状态、PromQL、绘图区和图例各自占据明确区域，不再相互挤压或覆盖；
- 图表宽度跟随卡片实际宽度，不再使用固定 `420px`；
- Y 轴不再出现 `undefined`，CPU/内存显示百分比单位，系统负载显示普通短数值；
- 图例只显示清晰的实例名，不重复拼接整组 labels；
- 页面和单张卡片都不产生横向溢出。

## 2. 范围边界

### 2.1 本计划包含

- `apps/grafana-plugin/frontend/src/workbench/Workbench.tsx` 中图表区的布局与参数传递；
- 新的、仅供 Workbench 使用的 ChartCard 展示组件（若实施时抽出文件，则放在同一
  `workbench` 目录）；
- `apps/grafana-plugin/frontend/src/workbench/mapper.ts` 中与展示有关的 DataFrame 字段配置；
- 与上述展示行为直接相关的单元测试和 mock browser E2E 断言。

### 2.2 本计划不包含

- 不修改 AI Core、assistant-mcp、Grafana Plugin Backend；
- 不修改 OpenAPI、JSON Schema、TaskEvent 或持久化结构；
- 不修改 `data/mock-scenarios/node_exporter_overview` 中的 mock 数值；
- 不调整分析输入、任务创建、SSE、路由恢复、助手文本等现有工作台行为；
- 不引入新的图表库、CSS 框架或运行时依赖；
- 不重做整个页面视觉设计，只处理三张图及其直接容器。

## 3. 当前问题与根因

### 3.1 `Card` 被当作普通容器使用

当前 `ChartCard` 将 `h3`、`p`、`details` 和 `TimeSeries` 直接放进 Grafana `Card`。该组件
面向 Heading、Meta、Figure、Actions 等预设槽位，有自己的网格布局语义，并不是普通的
纵向内容容器。当前用法使标题、状态、PromQL 和图表被分配到不合适的网格位置，形成截图中
标题逐字折行、状态与绘图区重叠、内容横向串出卡片的现象。

修正原则：不继续依赖 `Card` 的隐式槽位布局。图表卡片改用 Grafana 的 `Box`、`Stack`、
`Text`、`Grid` 等基础布局组件组合成明确的纵向结构。

### 3.2 图表宽度固定为 `420px`

外层网格是响应式的，但 `TimeSeries` 使用固定宽度。卡片变窄时图表不会同步收缩，卡片变宽时
又不能充分利用空间。图例文本较长时，实际内容宽度还会进一步超过卡片边界。

修正原则：以图表容器的 `contentRect.width` 为唯一宽度来源，通过原生 `ResizeObserver`
监听。首次获得大于零的尺寸后再渲染 `TimeSeries`，窗口变化或网格换列时同步更新宽度。

### 3.3 DataFrame 缺少展示配置

当前 mapper 只写入时间、数值和 labels，没有为数值字段提供 unit、displayName 及适合当前
Grafana theme 的 display processor。独立使用 `TimeSeries` 时，这些展示信息不会像标准 Panel
渲染链路那样自动补齐，截图中的 Y 轴因此出现 `undefined`；默认图例还把字段名和全部 labels
拼在一起，形成很长的重复文本。

修正原则：

- mapper 保持数据转换职责，但接收 chart 的 `unit`，写入数值字段 `config.unit`；
- 数值字段设置 `config.displayName = series.name`，同时保留原始 labels，图例显示实例名，
  tooltip 仍可使用标签信息；
- ChartCard 在拿到 theme 后为 frame 字段补充 Grafana `getDisplayProcessor`，保证坐标轴、
  legend 和 tooltip 使用一致的格式化逻辑；
- `percent` 沿用 mock chart 声明的 0–100 百分比语义；`short` 用于系统负载，不在前端按标题
  硬编码单位。

### 3.4 状态信息取值与排版不够稳健

当前代码直接执行 `String(chart.status)`。这会把缺失值变成可见的 `undefined`，而且 chart 的
`proposed/ready` 更偏向领域状态，用户在卡片上更关心查询结果是否已加载。

修正原则：使用 execution 状态形成有限的 UI 文案，不输出任意字符串：

- 尚无 execution：`查询中`；
- execution 为 `success`：`已加载`；
- execution 为 `failed`：`加载失败`；
- 未识别状态：使用中性的 `状态未知`，绝不显示 `undefined`。

状态以小型 Badge 放在标题右侧；标题区域允许正常换行，但不能被压缩成单字一行。

### 3.5 缺少显式空态和失败态

当前只有 `frame && timeRange` 时才绘图，其余情况留下空白卡片，无法区分加载、空数据和
时间范围缺失。

修正原则：绘图区始终保留稳定高度，并覆盖四种状态：加载中、查询失败、无 series、可绘制。
这只解释现有 mock/事件数据，不改变任何请求或重试逻辑。

## 4. 目标 UI 结构

单张卡片采用固定层级，所有内容均在卡片内部纵向排列：

```text
图表网格
└── 图表卡片（min-width: 0，overflow: hidden）
    ├── Header
    │   ├── 标题
    │   └── 状态 Badge
    ├── PromQL details（默认收起）
    │   └── 可换行的 code/pre
    └── Plot 区（宽度由 ResizeObserver 测量）
        ├── Y 轴 + 两条 mock series + X 轴
        └── 底部紧凑图例
```

布局约束：

- 外层使用响应式 Grid，建议最小卡片内容宽度约 `400px`；
- 宽视口优先三列等宽，中等视口自动两列，窄视口一列；
- 卡片和 plot wrapper 必须设置 `min-width: 0`，卡片内容不得反向撑大 Grid column；
- Plot 建议保持约 `260px` 高度，三张卡片在同一行时视觉高度一致；
- 图例放在底部并使用 list 模式，允许两个实例分行，但不能越过卡片边界；
- PromQL 默认收起；展开后使用 `white-space: pre-wrap` 和 `overflow-wrap: anywhere`；
- 颜色、边框、背景、间距均使用 Grafana theme token 或 Grafana Layout 组件，不写死只适合
  当前暗色主题的颜色。

## 5. 实施步骤

### P0 — 固化基线与可验证现象

1. 保留 `current_ui.png` 作为人工比对基线，不把截图内容当作数据契约。
2. 用现有 mock 场景确认三张 chart 均包含两条 series、每条七个点，unit 分别为
   `percent`、`percent`、`short`。
3. 记录当前需消除的可见字符串与几何问题：`undefined`、长 labels 图例、标题异常折行、
   图表超出卡片、页面水平滚动。

完成条件：后续测试可以分别覆盖“数据仍然存在”和“展示已经修正”，避免通过隐藏图表来消除
溢出。

### P1 — 隔离图表卡片布局

1. 将 `ChartCard` 改为接收完整 `chart` 和可选 `execution`，由组件内部计算 frame、timeRange
   和展示状态，减少 Workbench map 中的松散参数组合。
2. 移除当前作为内容容器的 Grafana `Card`。
3. 使用 `Box` 构造有背景、边框、圆角、padding 的语义化 `article`；内部使用纵向 `Stack`。
4. Header 使用横向 Stack：标题占剩余空间，Badge 保持自身宽度；为标题父项设置
   `min-width: 0`。
5. 保留现有 `data-testid="timeseries-panel"`，新增稳定的 plot wrapper test id，避免 E2E
   依赖 Grafana 内部 uPlot DOM 类名。

完成条件：即使暂时不渲染 TimeSeries，标题、状态和 PromQL 也必须在卡片内按纵向顺序显示。

### P2 — 让三张图按容器自适应

1. 为 plot wrapper 建立 ref 和 width state。
2. 在 effect 中创建原生 `ResizeObserver`，读取 `entry.contentRect.width`，四舍五入后仅在值
   变化时更新 state，避免无意义重复渲染。
3. effect cleanup 中 disconnect observer，防止任务刷新或路由切换后遗留监听器。
4. width 为零时渲染固定高度占位，不创建零宽 uPlot；width 有效后传给 `TimeSeries`。
5. 删除 `width={420}`，保留统一的 plot height；Grid 换列和浏览器 resize 都由 observer 驱动。
6. 外层图表区改为明确的响应式 Grid；卡片宽度由 Grid 决定，不由图表反向决定。

完成条件：在 1440px、1024px、768px 三类视口下页面无水平滚动，且 plot 的右边界不超过
所属卡片的 content box。

### P3 — 补齐单位、字段名和 Grafana display processor

1. 将 `ChartWireToDataFrame(series)` 调整为
   `ChartWireToDataFrame(series, unit)`；时间和值的对齐算法保持不变。
2. 为每个数值字段写入：
   - `config.unit = unit`；
   - `config.displayName = item.name`；
   - 仅在需要时设置合理 decimals，优先遵循 unit，不按中文标题分支。
3. 继续保留 `labels: item.labels`，不丢失 mock 标签元数据。
4. ChartCard 使用 `useTheme2` 和 `getDisplayProcessor` 对 frame 字段做展示增强；用 `useMemo`
   保证只有 frame、theme 或 unit 改变时重建。
5. 明确 legend 为底部 list 模式，无 calcs；三张图均应显示 `node-a:9100` 与
   `node-b:9100`，不再追加 `{instance=..., job=...}`。

完成条件：

- CPU 与内存 Y 轴刻度带百分比语义且不出现 `undefined`；
- 系统负载能显示约 0.5–1.7 的数值范围；
- hover/legend 数值与坐标轴格式一致；
- 三张图仍各有两条非空折线。

### P4 — 整理状态、PromQL 与空态

1. 用受控映射生成 `查询中`、`已加载`、`加载失败`、`状态未知`，不再
   `String(chart.status)`。
2. PromQL details 仍默认收起，summary 与 title/status 分离；展开后的表达式可任意换行，
   不撑宽卡片。
3. 渲染分支按优先级处理：execution failed → 时间范围缺失 → 无 series → 正常绘图；尚未收到
   execution 时显示加载占位。
4. 为非正常状态提供可读文本，保持 plot 区最小高度，避免事件抵达过程中卡片大幅跳动。
5. 不把 mock status `success`、chart status `proposed` 直接裸露为主要 UI 文案；原始值仍保留在
   state 中，不改 reducer。

完成条件：任意缺失字段都不会把 JavaScript 的 `undefined`、`null` 或 `[object Object]`
输出到页面。

### P5 — 补充测试

#### Mapper 单元测试

扩展 `mapper.test.ts`：

- 继续断言 timestamp/value/labels 对齐，防止 UI 修正破坏 mock 曲线；
- 断言 percent/short unit 被写入每个数值字段；
- 断言 displayName 为实例名，而 labels 仍完整保留；
- 覆盖空 series，确保返回合法空 frame，不在渲染阶段抛错。

#### Browser E2E

增强 `tests/e2e/mock/browser-e2e.spec.ts`：

- 仍断言三张标题、三张非空图和刷新恢复；
- 断言页面不存在可见文本 `undefined`；
- 断言两种实例图例在三张图中均出现，且不显示冗长 labels 拼接结果；
- 通过稳定的 panel/plot test id 比较 bounding box，确保 plot 位于所属卡片内部；
- 断言 `document.documentElement.scrollWidth <= clientWidth`；
- 至少在宽视口和窄视口各执行一次 resize 后检查，证明 ResizeObserver 和 Grid 换列生效；
- 展开一个 PromQL，断言表达式仍位于卡片边界内。

不新增基于 Grafana 内部 class name 的断言；这些 class 不属于本项目的稳定接口。

### P6 — 验证与人工复核

按以下顺序执行：

```bash
make test-frontend
cd apps/grafana-plugin/frontend && npm run build
make e2e-mock
make check
```

最后在真实 Grafana mock 页面复核并生成一张新的完成态截图，逐项对比：

1. 三张标题完整可读；
2. 状态 Badge 不挤压标题；
3. Y 轴单位正确且无 `undefined`；
4. 两条曲线的数据形状仍对应 fixture；
5. 图例简洁、完整、未出界；
6. 宽屏三列，窄屏换列，无水平滚动；
7. PromQL 收起和展开均不破坏卡片宽度；
8. 刷新恢复后布局与首次加载一致。

## 6. 预计文件变更

| 文件 | 变更 | 原因 |
| --- | --- | --- |
| `apps/grafana-plugin/frontend/src/workbench/Workbench.tsx` | 调整图表 Grid 与 ChartCard 入参 | 只保留页面编排职责 |
| `apps/grafana-plugin/frontend/src/workbench/ChartCard.tsx`（新增） | 卡片结构、状态、测宽、TimeSeries 展示 | 将三图 UI 修正隔离在单一组件 |
| `apps/grafana-plugin/frontend/src/workbench/mapper.ts` | 接收 unit，补字段展示配置 | 修正坐标轴与图例输入 |
| `apps/grafana-plugin/frontend/src/workbench/mapper.test.ts` | 增加 unit/displayName/空数据覆盖 | 保护纯映射逻辑 |
| `tests/e2e/mock/browser-e2e.spec.ts` | 增加无 undefined、无溢出、响应式断言 | 覆盖截图暴露的真实浏览器问题 |

原则上不修改 `package.json`、lockfile、后端、contracts 和 mock fixtures。若实施时发现仅靠现有
Grafana 13.1.0 API 无法完成 display processor 或布局，必须先记录具体编译/运行证据，再决定
是否扩大范围，不能顺手升级依赖。

## 7. 验收标准

全部条件同时满足才算完成：

- 三张 mock 图均可见，各有两条 series，时间和值未被 UI 改造改变；
- CPU/内存使用百分比显示，系统负载使用普通数值显示；
- 页面不存在可见 `undefined`；
- 标题、状态、PromQL、坐标轴、曲线、图例互不覆盖；
- 卡片、plot、legend 均不越过自身 Grid column，页面无水平滚动；
- 宽屏保持三列，空间不足时自动降为两列/一列；
- PromQL 展开后长表达式可换行，不撑宽页面；
- 暗色主题下颜色来自 Grafana theme，且实现未写死阻碍浅色主题；
- 页面提交、SSE 更新、刷新恢复等已有行为保持不变；
- `make test-frontend`、frontend build、`make e2e-mock`、`make check` 全部通过。

## 8. 风险与控制

- **ResizeObserver 抖动**：宽度取整并只在变化时 setState，cleanup 时断开 observer。
- **Grafana TimeSeries 为显式尺寸组件**：零宽时不渲染，避免初始化错误或不可恢复的空图。
- **display processor 与 theme 绑定**：在组件层创建，不把 theme 引入纯 mapper。
- **图例名称过度简化**：只把 displayName 设为 fixture 已有的 series.name，labels 仍保留用于
  tooltip 和未来交互。
- **响应式断言易脆弱**：检查边界和水平滚动，不绑定 Grafana 内部 CSS 类或精确像素颜色。
- **范围膨胀**：任何后端、contract 或 fixture 变更均视为超出本计划，除非先证明输入数据违反
  现有契约；当前检查未发现这种情况。

## 9. 实施顺序与提交建议

建议按一个集中变更完成，不拆散到后端或 mock 提交中：

1. ChartCard 结构与响应式测宽；
2. mapper unit/displayName 与 display processor；
3. 状态、空态、PromQL 收尾；
4. unit test 与 browser E2E；
5. 全量验证和新截图复核。

如果需要拆成两个提交，第一提交只包含 UI/mapper，第二提交只包含测试；不得把无关格式化或
仓库其他清理混入其中。

## 10. 完成记录

实施已按小步提交完成：

1. `f0ca394 docs: add chart trio UI fit plan`
2. `e35908b fix(ui): add chart display metadata`
3. `12d1271 fix(ui): make chart cards responsive`
4. `5de075d test(ui): cover responsive chart rendering`

实际变更保持在图表展示范围内：ChartCard 改为显式的 Grafana 基础布局，Grid 和
`ResizeObserver` 驱动实际绘图宽度；mapper 为字段写入 unit 和简洁 displayName；组件层按当前
theme 补全数值 display processor；浏览器测试覆盖三图、单位/图例、无 `undefined`、宽窄视口、
PromQL 展开、边界与刷新恢复。

验证证据：

- `make test-frontend` 通过（5 个 Vitest 文件、10 项测试及 TypeScript 检查）；
- `npm run build` 通过；
- `make check` 通过；
- 使用隔离 Compose 的 13000/18080/18081 端口运行完整 mock API E2E 和 Playwright browser E2E，
  均通过；
- 宽屏页面截图人工复核确认：标题、状态、坐标轴、曲线和图例均在各自卡片内，CPU/内存以百分比
  显示，系统负载以数值显示，未出现 `undefined`。

默认 `make e2e-mock` 首次尝试未能占用 `127.0.0.1:8081`，因为用户环境已有 SSH 转发和现有
`mini-torchbearing-*` 服务在使用该端口。为避免影响这些服务，未停止或修改它们；替代验证使用
同一份 Compose 服务定义，仅临时重映射宿主机端口，并在结束后清理了该隔离项目的容器、网络和
volume。
