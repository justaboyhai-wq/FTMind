# Skills API

[返回目录](./README.md)

| 方法 | 路径      | 描述               |
| ---- | --------- | ------------------ |
| GET  | `/skills` | 获取预装 Skills 列表 |

## GET `/skills` - 获取预装 Skills 列表

获取系统中所有预装的智能体技能列表。

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/skills' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json'
```

## FMind 统一凭证边界

`/skills` 使用 FMind 用户 API Key（推荐环境变量名
`FMIND_USER_API_KEY`，`FMIND_API_KEY` 仅作为兼容别名）。该 Key 只识别用户，
不授予额外的租户、团队或记忆权限。外部 Agent 的 L0-L2 记忆和 Memory Wiki
必须通过 FMind/MemoryProxy 网关，并使用绑定专用的 Agent 运行期 Key；不得直接
访问 MemoryCore，也不得把 Agent Key 发送到普通 Knowledge API 或上游模型服务。

**响应**:

```json
{
    "data": [
        {
            "name": "web_search",
            "description": "搜索互联网获取最新信息"
        },
        {
            "name": "code_interpreter",
            "description": "执行代码并返回结果"
        },
        {
            "name": "image_generation",
            "description": "根据文本描述生成图片"
        }
    ],
    "skills_available": true,
    "success": true
}
```

当系统未配置 Skills 时，`skills_available` 返回 `false`，`data` 为空数组：

```json
{
    "data": [],
    "skills_available": false,
    "success": true
}
```
