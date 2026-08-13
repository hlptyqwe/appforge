# Common MQ

Directory layout:

```text
common/mq/
├── kafka/   # system-wide production implementation
└── redis/   # legacy compatibility adapter; no business service uses it
```

The system-wide message bus is Kafka and is exposed through the small API in
`kafka/kafka.go`. Business services should depend only on `mq.Publisher` and
`mq.Subscriber`; they must not call Redis Pub/Sub directly.

## Delivery modes

- `MustNewSubscriber`: durable competing consumer. Use a stable group ID for
  workers. A new group starts from the first retained record.
- `MustNewBroadcastSubscriber`: live fan-out consumer for WebSocket/API
  gateways. Each simultaneously running service instance must use a distinct
  group ID. A new group starts at the end of the topic and an existing group resumes
  from its committed offset.
- Handler failures are retried and then written to `<topic>.dlq`. The source
  offset is committed only after successful handling or successful DLQ write.

## Required topics

Auto topic creation is intentionally disabled. Provision the topics below and
their `.dlq` counterparts before starting services:

- `system.scheduled-tasks`
- `trade.domain-events`
- `admin.notifications`
- `appforge.chat.app.events`
- `appforge.chat.admin.events`

Production topics should use at least three replicas and `min.insync.replicas=2`.
Partition counts depend on expected concurrency; preserve ordering-sensitive
events by publishing a stable key when keyed publishing is added to the public
API.
