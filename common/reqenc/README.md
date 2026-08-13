# Request encryption

Shared server-side implementation of request-encryption protocol `v1`.

Implemented:

- `RSA-OAEP-256` client AES key unwrapping.
- `AES-256-GCM` request decryption with authenticated AAD.
- `DISABLED`, `OPTIONAL`, and `REQUIRED` modes.
- Redis-backed short-lived sessions and nonce replay protection.
- Server-side AES session-key wrapping with a 32-byte KEK.
- JSON, form, and query request restoration before go-zero handlers parse input.
- Encryption configuration and session bootstrap HTTP handlers.

Not implemented by the global middleware:

- Path rewriting. go-zero has normally selected a route before a global middleware runs. Add an
  explicit encrypted route instead.
- Multipart bodies and WebSocket messages.
- Response encryption and request signing.

## Construction

```go
cfg := reqenc.Config{
    Scope:               "app-api",
    Mode:                reqenc.ModeDisabled,
    RSAKid:              "app-api-2026-01",
    RSAPrivateKeyPath:   "/run/secrets/request-encryption/current.pem",
    SessionWrapKey:      os.Getenv("REQUEST_ENCRYPTION_SESSION_WRAP_KEY"),
    SessionTTLSeconds:   960,
    RotateBeforeSeconds: 180,
}

store := reqenc.NewRedisStore(rds, cfg.Scope)
service, err := reqenc.New(cfg, store)
```

`SessionWrapKey` must be exactly 32 bytes and must come from a secret manager. It must not be
committed to the repository.

## Route registration

Mode semantics:

- `DISABLED`: every route is plaintext and encrypted requests are rejected.
- `OPTIONAL`: regular registered routes must be encrypted; other routes stay plaintext.
- `REQUIRED`: regular and `RequiredOnly` routes must be encrypted.

Use `RequiredOnly` catch-all rules to cover all supported API methods in `REQUIRED` mode, and
explicit `Exempt` rules for the encryption bootstrap endpoints. `OPTIONS`, multipart uploads, and
WebSocket traffic should remain outside the encrypted rules.

Put the optional-mode route prefixes in `Config.ProtectedPrefixes` and return them to clients from
the encryption-config endpoint as `protectedPrefixes`, so the server and clients use one source of
truth.

```go
registry := reqenc.NewRegistry(
    reqenc.Rule{
        Method: http.MethodPost,
        Path: "/app/user/login",
        Location: reqenc.LocationJSON,
    },
)

server.Use(reqenc.NewMiddleware(service, registry).Handle)
```

Register the middleware before request logging, RBAC, rate limiting, or any middleware that reads
the request body or parameters.

The two bootstrap handlers must be registered as `Exempt` when a `RequiredOnly` catch-all is used:

```go
service.ConfigHandler()
service.SessionHandler()
```

Every API must use a distinct `Scope`, Redis key namespace, and preferably a distinct RSA key.
Start deployments in `DISABLED`, switch to `OPTIONAL` after clients encrypt selected routes, and
switch to `REQUIRED` after clients encrypt every supported route.
