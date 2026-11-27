# PulseNav · PulseGeo

PulseNav 是空间导航与寻址 Agent 项目，PulseGeo 是其规划中的确定性空间计算核心，为上层 AI Agent（如 BizPulse 商业选址项目）提供 POI 检索、地理编码和空间几何计算能力。

LLM 负责语义理解与意图拆解，PulseGeo 负责可验证的空间计算。例如，上层 Agent 将“寻找商圈附近适合开店的位置”拆解为明确的中心点、搜索半径、围栏和 POI 条件，再调用空间服务获取结果。

> 当前已提供首期 Go Agent MVP，下方区分已实现能力与后续目标。亿级数据规模与毫秒级响应仍是需要通过基准测试验证的目标。

## 当前可运行的版本

- 基于 CloudWeGo Eino 的单 Agent，支持 OpenAI 兼容的 Chat Completions 工具调用协议。
- 四个受控工具：球面距离计算、简单围栏点面判断、WGS84 → GCJ02、POI 复合查询。
- POI 查询提供真实 PostGIS 仓储与显式开启的合成演示仓储，返回 GeoJSON FeatureCollection。
- HTTP API、输入校验、请求取消、超时、Agent 并发与工具调用限额、优雅退出。
- 单元测试、本地模型协议闭环测试、可配置的真实 PostGIS 集成测试。

尚未实现：地址地理编码、Redis / BigCache、多边形孔洞和 MultiPolygon、游标分页、持久会话、多 Agent、指标与追踪、任意代码执行沙箱。首期没有动态代码执行需求，因此不引入沙箱。

Agent 负责选择已注册的工具，不允许执行任意 SQL、Shell 或模型生成的代码。模型文本回答仍可能有误，`tool_results` 中的结构化结果用于核对实际计算；没有发生工具调用时，不应把文本中的数值视为计算证据。

## 快速开始

需要 Go 1.25 或更新版本；PostGIS 模式额外需要数据库。先在无数据库、无模型密钥的情况下启动：

```bash
go mod download
POI_MODE=demo go run ./cmd/pulsegeo
```

默认监听 `127.0.0.1:8080`。演示数据包含上海人民广场附近的三个合成 POI，不能用于真实商业决策。

```bash
curl http://127.0.0.1:8080/healthz

curl http://127.0.0.1:8080/v1/spatial/distance \
  -H 'Content-Type: application/json' \
  -d '{"from":[0,0],"to":[1,0]}'

curl http://127.0.0.1:8080/v1/poi/search \
  -H 'Content-Type: application/json' \
  -d '{"center":[121.4737,31.2304],"radius_meters":1000,"category":"cafe","limit":10}'
```

### 启用 Agent

在启动服务的终端配置支持工具调用的模型，再启动服务。请将占位内容替换为实际值：

```bash
export OPENAI_API_KEY='your-api-key'
export OPENAI_MODEL='your-tool-calling-model'
# 可选：兼容服务的 API 基地址，通常以 /v1 结尾。
# export OPENAI_BASE_URL='https://your-provider.example/v1'
POI_MODE=demo go run ./cmd/pulsegeo
```

在另一个终端请求：

```bash
curl http://127.0.0.1:8080/v1/agent/run \
  -H 'Content-Type: application/json' \
  -d '{"message":"使用 WGS84 坐标 [121.4737,31.2304]，查找周围 1000 米的 cafe，最多返回 5 个，并说明数据来源。"}'
```

返回包含 `answer` 和 `tool_results`。每次请求是独立会话，不保留上一轮聊天历史。模型密钥与模型名称必须同时配置；同时留空时 Agent API 返回 503，其他 API 仍正常工作。配置文件示例见 [.env.example](.env.example)，程序不会自动加载 `.env`。

### 使用 PostGIS

本地开发可使用仓库中的 Compose 配置，账号仅用于本地开发：

```bash
docker compose up -d --wait postgis
export DATABASE_URL='postgres://pulsegeo:pulsegeo_local@127.0.0.1:5432/pulsegeo?sslmode=disable'
POI_MODE=postgis go run ./cmd/pulsegeo
```

Compose 在首次初始化空数据卷时执行 `migrations/`。已有数据库需由部署者执行迁移，例如：

```bash
docker compose exec -T postgis psql -U pulsegeo -d pulsegeo < migrations/001_poi.sql
```

服务不会自动创建表，也不会导入合成 POI；新数据库返回空结果。实际数据写入 `poi` 表，位置类型为 `geography(Point,4326)`。数据库故障会显式报错，不会回退到演示数据。部署时迁移与服务账号应分离，查询服务仅需读取 POI 的权限。

### API 与资源限制

| 方法及路径 | 请求主体 | 返回 |
| --- | --- | --- |
| `GET /healthz` | 无 | 进程存活状态、POI 来源、Agent 是否启用 |
| `POST /v1/spatial/distance` | `from`、`to` 两个 WGS84 坐标 | 米数及计算方法 |
| `POST /v1/spatial/contains` | `point`、GeoJSON `polygon` | 是否命中，包含边界 |
| `POST /v1/spatial/convert` | WGS84 `point` | 明确标注 GCJ02 的坐标 DTO |
| `POST /v1/poi/search` | `center` + `radius_meters` 和/或 `polygon`，可选 `category`、`limit` | GeoJSON FeatureCollection |
| `POST /v1/agent/run` | `message` | 模型回答及成功工具调用结果 |

所有 POST 请求需要 `Content-Type: application/json`，不接受未知字段。`polygon` 首期仅支持无孔洞、非自交的闭环 Polygon，最多 256 个顶点，不支持跨日期变更线及纬度绝对值大于 85° 的围栏。

POI 查询半径最大 50km，结果默认 20 条、最多 100 条，分类为精确匹配。圆形查询按距离及 ID 排序，仅围栏查询按 ID 排序。演示仓储使用球面距离，PostGIS 使用椭球距离，近距离阈值验收以 PostGIS 为准。

HTTP body 最多 64KiB，Agent 消息最多 16000 字节；每进程最多 8 个并发 Agent 请求，满载返回 429。单请求最多执行 8 次工具调用，Eino 图最多 12 步（并非 12 轮对话），总超时 30 秒，单次 POI 查询最多 3 秒。工具参数无效、工具预算耗尽或模型失败会终止本次 Agent 运行并返回脱敏的 503；确定性 API 参数错误返回 400，超时返回 504。

`/healthz` 是存活检查，不探测数据库和模型提供商，也不保证业务表已就绪。当前服务面向本地开发，尚无身份认证；对外部署前需接入网关鉴权与调用方配额。

### 验证

```bash
go test -race ./...
go vet ./...
go build ./...
```

默认测试通过本地 HTTP 服务模拟模型协议，不访问真实模型服务、不消耗模型额度。真实 PostGIS 测试需要显式配置独立测试数据库；测试在事务中创建独立 schema，结束后回滚：

```bash
TEST_DATABASE_URL='postgres://pulsegeo:pulsegeo_local@127.0.0.1:5432/pulsegeo?sslmode=disable' \
  go test -count=1 -v ./internal/postgres
```

未配置 `TEST_DATABASE_URL` 时该集成测试会跳过，不能将普通测试通过视为数据库验证通过。测试账号需要创建 schema 及使用 PostGIS 的权限。

## 项目目标

- 提供支持超时、取消和资源限制的高并发空间检索接口。
- 通过 PostgreSQL + PostGIS 实现可索引、可解释的空间查询。
- 通过本地缓存与 Redis 降低地理编码的重复计算和回源压力。
- 提供纯 Go 空间算法，处理无需访问数据库的轻量计算。
- 使用明确的坐标、边界和错误语义，为 Agent 提供确定性的结果。

## 技术栈与架构

采用模块化单体起步，通过依赖注入和小接口隔离业务逻辑与基础设施，后续根据性能瓶颈决定是否拆分服务。

| 层次 | 技术与职责 |
| --- | --- |
| 服务与并发 | Go、`context.Context`、有界并发与连接池 |
| 主存储 | PostgreSQL + PostGIS，使用 GiST 空间索引 |
| 多级缓存 | 本地 BigCache + Redis，缓存组件通过接口接入 |
| 空间交换格式 | 对外接口及模块交换边界采用 GeoJSON |
| 内部计算 | 强类型坐标与几何结构，避免反复 JSON 编解码 |
| 运行保障 | 限流、超时、结构化日志、指标与链路追踪 |

请求链路：Agent / API → 输入校验与坐标归一化 → 业务服务 → 缓存 / PostGIS / 地理编码数据源 → GeoJSON 结果。

## 统一空间约定

### 坐标系与单位

- 持久化和内部计算统一使用 WGS84，数据库 SRID 为 4326。
- GeoJSON 坐标顺序固定为 `[longitude, latitude]`，不使用 `[latitude, longitude]`。
- 距离和半径统一使用米，经纬度使用十进制度。
- 接入边界必须明确输入坐标系。GCJ02 数据在归一化后再参与空间检索，输出转换按调用方要求执行。
- WGS84 → GCJ02 使用独立的算法封装，注明适用区域和近似误差；区域外按约定保持原坐标。反向转换如需提供，应注明迭代策略与误差容限。
- 校验经纬度范围、非有限数值、半径上限、几何复杂度和结果数量上限。

GeoJSON 输出遵循 WGS84 约定；需要 GCJ02 的调用方通过明确标注坐标系的独立 DTO 获取结果，避免产生语义不明的 GeoJSON。

### 几何关系

- 中心点与半径必须成对提供；圆形与多边形条件同时存在时，默认取交集。
- 围栏查询默认包含边界点。数据库使用 `ST_Covers`，纯 Go 工具保持相同边界语义。
- API 接收 GeoJSON Polygon / MultiPolygon 时，需要校验闭环、孔洞、顶点数量及几何有效性；无效几何返回参数错误，不静默修复。
- 跨日期变更线和极区几何需要专门处理，未实现前应明确拒绝此类围栏，不能按普通平面多边形计算后返回结果。
- 纯 Go 几何工具采用渐进交付方式，首期可只支持普通区域的简单多边形；尚不支持的几何类型必须显式报错。

## 核心模块

### 一、POI 检索引擎

支持中心点、半径、多边形围栏与业务属性的复合筛选。POI 至少包含 ID、名称、分类、位置和可扩展属性；搜索请求包含空间条件、属性条件、分页及数量限制。

目标业务接口：

```go
type POISearcher interface {
    SearchPOI(ctx context.Context, req *SearchRequest) ([]*POI, error)
}
```

核心实现约束：

1. 使用 `ST_DWithin` 执行半径过滤。推荐将位置存为 `geography(Point, 4326)` 并建立匹配的 GiST 索引，距离参数直接使用米。
2. 如果存储类型采用 geometry，并在查询中转换为 geography，需要建立匹配该表达式的索引，并使用查询计划验证实际命中情况。
3. 多边形查询使用统一 SRID 的 geometry 执行 `ST_Covers`，根据实际查询形态配置索引。避免对索引列进行未匹配索引的变换。
4. Geohash 仅作为可选粗筛手段，必须覆盖搜索区域涉及的全部网格并执行精确过滤，不能只搜索中心点所在网格。
5. 距离排序使用稳定的次级排序键，例如 POI ID；较深分页优先考虑游标分页。
6. SQL 参数化执行，属性筛选与排序字段使用白名单，不能直接拼接用户输入。
7. 调用链将 `ctx` 传递到数据库驱动；服务默认超时不得延长调用方已有的更短截止时间。

`ST_DWithin` 能利用空间索引缩小候选集，再执行精确关系判断。是否达到预期性能，需要通过 `EXPLAIN (ANALYZE, BUFFERS)` 和代表性数据验证，不能仅凭建有索引判断。

### 二、智能地理编码服务

支持非结构化地址到坐标的正向地理编码，以及坐标到地址的逆向地理编码。

```go
type Geocoder interface {
    Geocode(ctx context.Context, req *GeocodeRequest) (*GeocodeResult, error)
    ReverseGeocode(ctx context.Context, req *ReverseGeocodeRequest) (*ReverseGeocodeResult, error)
}
```

地理编码结果应携带规范地址、坐标、数据来源、匹配精度或置信信息。数据库回源依赖实际建设的地址库和匹配能力：PostGIS 提供空间计算能力，不会自动完成非结构化地址解析。

正向处理流程：地址归一化 → 缓存查询 → 地址库匹配 → 按配置调用外部地理编码服务 → 写入缓存。

逆向处理流程：坐标归一化 → 缓存查询 → 本地地址库空间检索 → 按配置调用外部服务 → 写入缓存。

外部服务是可选适配器，需要独立配置配额、超时和失败处理；没有有效数据源时应返回明确错误，不能伪造定位结果。

#### 多级缓存与双映射

缓存读取顺序为本地 BigCache → Redis → 数据源，Redis 命中后回填本地缓存。缓存失效和版本策略需要覆盖两个层级，本地缓存通常采用更短 TTL。

“双映射”指维护正向与逆向两类缓存键空间，不能假设二者互为逆函数：多个地址可以指向同一地点，同一坐标也可能有不同粒度的地址描述。

| 缓存类型 | 键的主要维度 |
| --- | --- |
| 正向 | 归一化地址、城市或区域约束、语言、数据源及版本 |
| 逆向 | 归一化坐标、坐标精度、结果粒度、语言、数据源及版本 |

坐标量化用于提升逆向缓存命中率时，必须定义空间误差范围，不能让相邻建筑误用同一个精确地址。正向命中结果不能无条件写入逆向缓存，反向亦然。

缓存保护策略：

- 穿透：对确定的“无结果”使用短 TTL 负缓存；输入无效、超时或数据库故障不能缓存为“无结果”。
- 击穿：同进程内通过 singleflight 合并同键回源；多副本仍需依赖回源限流，必要时引入跨实例协调。
- 雪崩：TTL 加入随机抖动，限制回源并发，并为缓存故障设置明确的降级策略。
- 取消：等待共享回源的调用方可独立取消；共享任务采用有上限的生命周期，避免首个请求取消导致其他等待者全部失败。

缓存中存放序列化数据或不可变值，避免共享可变指针引发并发读写问题。

### 三、空间计算与几何工具

提供不访问网络和数据库的纯 Go 函数，便于复用、测试和性能分析。

目标函数包括：

```go
func CalculateDistance(a, b Point) float64
func IsPointInPolygon(point Point, polygon Polygon) bool
func WGS84ToGCJ02(point Point) Point
```

上述纯函数面向已校验的输入；服务边界及几何构造函数负责返回参数错误。距离单位为米，点面判断包含边界。

- **Haversine 球面距离**：经纬度转换为弧度后计算球面中心角，使用明确记录的平均地球半径。中间值限制在合法浮点范围，处理重合点、日期变更线和接近对跖点的情况。该算法是球面近似；高精度需求交给椭球测地线算法或 PostGIS geography 计算。
- **Ray Casting 点面判断**：先判断点是否落在线段边界，再通过半开区间规则统计射线交点，避免顶点被重复计数，并正确处理水平边。含孔洞的多边形需分别处理外环与内环，孔洞内部不属于多边形。
- **坐标转换**：转换逻辑与业务服务隔离，记录算法来源、适用区域及误差限制，防止重复转换。

浮点容差需要明确单位和适用范围，不能用一个未经定义的常数覆盖所有尺度。关键算法使用中文注释解释公式、数值处理和边界规则。

## 并发与资源管理

- HTTP 请求本身已具备并发执行能力，不为每个数据库操作额外启动无界 Goroutine。
- Goroutine / Channel 用于有明确收益的并行子任务和有界流水线；任务队列必须有容量、取消和关闭规则。
- 数据库连接池、Redis 连接池、外部服务并发和请求速率分别设置上限。
- 所有 I/O 接口接收 `context.Context`，及时释放连接、计时器与响应体。
- 并行任务失败或请求取消后停止无用工作，避免 Goroutine 泄漏。
- 使用锁时明确被保护的数据与临界区，不在持锁期间执行数据库或网络请求。

## 领域错误

业务层返回稳定的领域错误，通过 `errors.Is` / `errors.As` 判断。底层驱动错误保留在内部错误链及日志中，对外只暴露稳定错误码和安全描述。

建议至少区分：

| 错误类别 | 语义 |
| --- | --- |
| InvalidArgument | 坐标、半径、几何或筛选条件无效 |
| UnsupportedGeometry | 当前版本尚不支持的几何类型或空间范围 |
| NotFound | 地理编码没有匹配结果 |
| RateLimited | 请求超过资源或供应商配额限制 |
| Unavailable | 数据库或必要的外部依赖不可用 |
| DeadlineExceeded / Canceled | 请求超时或被调用方取消 |

POI 搜索无匹配时返回空结果和 `nil` 错误。地理编码无匹配可返回 NotFound，二者语义应在 API 中固定。取消与超时必须保留 Go 标准 context 错误的可识别性。

## 目录结构

```text
cmd/pulsegeo/          # 服务入口、配置加载与依赖组装
internal/agent/       # Eino 编排、受控工具注册及调用记录
internal/httpapi/     # HTTP 路由、输入校验与错误映射
internal/config/      # 环境配置与启动校验
internal/domain/      # 领域模型、校验规则与领域错误
internal/poi/         # POI 检索业务与仓储接口
internal/spatialsvc/  # HTTP 与 Agent 共用的空间工具 DTO 和校验
internal/postgres/    # PostgreSQL / PostGIS 适配器
pkg/spatial/          # 距离、点面关系与坐标转换
migrations/           # 数据表、约束与空间索引
docs/superpowers/     # 本次设计与实施计划
```

后续按需增加 `api/` 协议定义、`internal/geocoding/`、`internal/cache/` 和 `internal/provider/`，不预建空模块。以下章节中涉及缓存、地理编码、复杂几何和观测的内容均为后续设计要求。

接口由使用方按需要定义，构造函数注入依赖，避免业务逻辑直接依赖全局数据库或缓存实例。`pkg/` 仅保留适合对外复用的工具。

## 性能目标与验收

“亿级 POI、毫秒级检索”需要在明确条件下验收。实施前确定数据量与分布、目标 QPS、P95/P99 延迟、查询半径、围栏顶点数、返回条数、缓存命中率及硬件配置，再分别记录冷缓存和热缓存结果。

重点关注：

- POI 查询延迟、候选集规模、实际执行计划和连接池等待时间。
- 本地及 Redis 缓存命中率、负缓存命中率和回源并发。
- 地理编码各数据源延迟、错误率、配额消耗与匹配质量。
- CPU、内存分配、GC、Goroutine 数量以及请求取消后的资源回收。

数据分区、只读副本或独立检索服务应由测量结果驱动，不作为起步阶段的默认复杂度。日志和指标避免记录完整地址或高精度坐标，也避免将其作为指标标签。

## 实施路线与验证

1. **空间工具**：实现坐标模型、输入校验、Haversine、简单多边形点面判断和 WGS84 → GCJ02；通过边界测试后再扩展孔洞与复杂几何。
2. **POI 检索**：建立 PostGIS 表结构与索引，实现半径、围栏和属性复合查询，并用真实 PostGIS 执行集成测试与查询计划检查。
3. **地理编码**：明确地址数据源，实现正反向业务、地址归一化和缓存键规范，再接入多级缓存与回源保护。
4. **服务化与压测**：补齐 API、配置、限流、观测和部署说明，根据基准数据设定可承诺的服务指标。

验证范围包括：

- 空间算法：重合点、极小距离、对跖点、经度跨界、顶点、水平边、边界和孔洞。
- 查询一致性：圆形及围栏边界上的 POI 不漏检，复合条件和稳定排序符合约定。
- 缓存行为：命中、失效、负缓存、同键并发、依赖故障与共享回源取消。
- 并发正确性：使用 Go race detector 检查共享数据访问，验证取消和超时不会遗留任务。
- 性能：在固定数据、硬件与并发条件下记录基准，避免用单次或热缓存结果代表整体性能。

后续实现应保持可编译、可测试；数据导入流程及已验证的性能结果将在实际建设后补充，首期不作吞吐量或延迟承诺。
