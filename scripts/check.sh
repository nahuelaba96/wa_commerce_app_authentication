#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

run_validation() {
    local name="$1"
    shift

    printf '\n==> %s\n' "$name"
    printf 'Comando:'
    printf ' %q' "$@"
    printf '\n'

    if "$@"; then
        printf 'Resultado: OK\n'
    else
        local status=$?
        printf 'Resultado: FALLÓ (exit code %d)\n' "$status" >&2
        exit "$status"
    fi
}

run_validation "Build" go build ./cmd/auth-api

run_validation \
    "Tests unitarios e integración PostgreSQL con Testcontainers" \
    go test ./...

run_validation "Análisis estático" go vet ./...

printf '\n==> Formato Go\n'
unformatted="$(find . -type f -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -l)"
if [[ -n "$unformatted" ]]; then
    printf 'Archivos Go sin formato:\n%s\n' "$unformatted" >&2
    exit 1
fi
printf 'Resultado: OK\n'

printf '\nTodas las validaciones configuradas pasaron.\n'