# ⚖️ balanced

> Lightning channel liquidity balancer

## Generating grpc files

```
protoc -I rpc/ rpc/rpc.proto --go_out=plugins=grpc:rpc
```