# This is a renovate-friendly source of Docker images.
FROM python:3.13.5-slim-bullseye@sha256:631af3fee9d0b0a046855a62af745c1f94b75c5309be8802a0928cce3ac0f98d AS python
FROM otel/weaver:v0.15.3@sha256:a84032d6eb95b81972d19de61f6ddc394a26976c1c1697cf9318bef4b4106976 AS weaver
FROM avtodev/markdown-lint:v1@sha256:6aeedc2f49138ce7a1cd0adffc1b1c0321b841dc2102408967d9301c031949ee AS markdown
