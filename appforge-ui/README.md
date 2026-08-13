# APK 渠道动态打包平台 UI

Vue 3 + TypeScript + Vite + Element Plus 管理端。

## 功能范围

- 系统：用户、角色、菜单权限、租户、租户域名、系统配置、登录日志、操作日志
- 平台：应用、版本、APK 签名配置、推广渠道、构建任务、渠道统计
- 路由和按钮权限来自 `/admin/system/auth/profile` 返回的 `menus + perms`

菜单的 `component` 必须对应 `src/views` 下的文件，例如：

- `platform/applications`
- `platform/versions`
- `platform/signing-configs`
- `platform/channels`
- `platform/build-tasks`
- `platform/channel-stats`

## 开发命令

```bash
npm install
npm run dev
npm run type-check
npm run build
```

