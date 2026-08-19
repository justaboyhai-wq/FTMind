# FMind Skill / ClawHub 发布说明

当前 Skill 包位于 `frontend/public/fmind-skill`，版本为 `2.0.0`，入口文件是
`SKILL.md`。它同时覆盖普通知识库 API 和 FMind/MemoryProxy 外部记忆网关，
但明确隔离用户 API Key、一次性 Setup Key 与 Agent 运行期 Key。

发布前必须由发布者在本机完成 ClawHub 登录：

```powershell
npm i -g clawhub
clawhub login
.
\scripts\publish-fmind-skill.ps1 -AllowPublish -Version 2.0.0
```

脚本不会读取或写入 FMind 密钥，也不会自动登录或从浏览器推导地址。若当前
账号没有发布权限，必须先在 ClawHub 完成授权后再执行。发布完成后，前端配置的
链接应保持为：

`https://clawhub.ai/justaboyhai-wq/skills/fmind`

如果实际发布账号或 slug 发生变化，应同步修改
`frontend/src/config/integrations.ts`，并重新构建前端。
