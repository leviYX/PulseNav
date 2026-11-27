# PulseGeo Agent MVP

根据已确认的方案，使用 Eino 编排单 Agent，工具参数由 Go 校验，模型不接触数据库凭据、SQL 或 Shell。

首期交付：HTTP JSON API、OpenAI 兼容模型接入、四种工具（距离、简单围栏判断、WGS84→GCJ02、POI 检索）、PostGIS 仓储与独立演示仓储。无模型密钥时确定性 API 仍可启动，Agent API 明确返回不可用。

坐标统一 WGS84，经纬度以 longitude/latitude 表示，距离单位米。围栏首期仅支持无孔洞、非自交、未跨日期变更线的简单 Polygon，包含边界；所有无效输入显式返回领域错误。空间结果使用 GeoJSON FeatureCollection。

中心半径与围栏取交集，结果按距离及 ID 排序；仅围栏搜索按 ID 排序。限制半径（50km）、顶点（256）、结果数（100）、HTTP body（64KiB）、Agent 并发（8）与总超时（30s）。每个请求独立会话，不持久化聊天历史，工具执行次数通过 Eino 最大步骤限制。

默认监听回环地址，模型密钥从环境读取。PostGIS 使用 geography GiST 半径过滤及 geometry 表达式 GiST 围栏过滤。演示模式必须主动配置，不在数据库故障时自动回退。真实地理编码、缓存、任意代码执行、多 Agent、持久会话均不属于本次范围。

验证：空间边界与非法输入测试；两种仓储的组合筛选及取消语义；使用本地 OpenAI 兼容测试服务验证完整模型→工具→模型链路；HTTP 错误脱敏、并发与取消测试；race、vet、build；可配置的真实 PostGIS 集成测试。
