# Alert 通知接口

`common/alert` 只定义告警领域对象和传输无关的 `Notifier` 接口。业务服务不应直接
依赖 Kafka Topic、WebSocket、HTTP 或具体通知厂商。

```go
type Notifier interface {
    Notify(context.Context, Alert) error
}
```

业务代码通过 `alert.Notify(ctx, notifier, value)` 发送。该入口会先校验告警身份、
状态、内容、来源和时间，再调用注入的实现。

当前实现位于 `common/alert/adminnotify`，负责将领域告警转换为现有 Admin
Notification 事件。新增邮件、短信、IM 等通道时，实现 `alert.Notifier` 并在服务
初始化时注入即可，不需要修改告警生产逻辑。

多个通道可通过 `alert.NewMultiNotifier(...)` 组合。组合器会尝试所有实现并使用
`errors.Join` 返回全部失败；各实现必须使用稳定的 `Alert.ID` 作为幂等键，避免部分
通道成功后整体重试产生重复通知。
