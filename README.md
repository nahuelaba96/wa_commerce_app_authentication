# Servicio de autenticación

Microservicio HTTP en Go con arquitectura hexagonal. El dominio y los casos de uso están separados de PostgreSQL/GORM, la API HTTP, la firma JWT y el cliente interno del backend.

## Requisitos

- Go 1.27+
- PostgreSQL
- Docker para ejecutar las pruebas de integración PostgreSQL
- Clave RSA privada PEM para firma RS256
- Backend Spring Boot con el endpoint interno de onboarding habilitado

## Configuración

El servicio carga automáticamente el archivo `.env` del directorio de trabajo al arrancar. Copia `.env.example` como `.env` y completa sus valores. El archivo `.env` está excluido de Git; nunca guardes secretos reales en `.env.example`.

Las variables de entorno del proceso tienen prioridad sobre los valores de `.env`, para facilitar la configuración segura en contenedores y despliegues:

```sh
cp .env.example .env
# Edita .env y completa los valores requeridos
go run ./cmd/auth-api
```

Variables obligatorias:

| Variable | Descripción |
|---|---|
| `AUTH_DATABASE_URL` | URL PostgreSQL exclusiva de autenticación. |
| `BACKEND_INTERNAL_URL` | URL base interna del backend, por ejemplo `http://localhost:8080`. |
| `INTERNAL_SERVICE_TOKEN` | Secreto compartido para llamadas internas; proporcionarlo mediante un gestor de secretos. |
| `AUTH_PRIVATE_KEY_PATH` | Ruta al PEM RSA usado para firmar los access tokens. |
| `AUTH_ISSUER` | Claim `iss` esperado por los validadores. |
| `AUTH_AUDIENCE` | Claim `aud` esperado por las APIs. |

Configuración opcional:

| Variable | Valor predeterminado |
|---|---|
| `AUTH_LISTEN_ADDRESS` | `:8081` |
| `AUTH_KEY_ID` | `auth-rsa-1` |
| `AUTH_ALLOWED_ORIGINS` | `http://localhost:4200` (lista separada por comas; no admite `*`). |
| `AUTH_REFRESH_COOKIE_NAME` | `auth_refresh` |
| `AUTH_COOKIE_DOMAIN` | Cookie sin atributo `Domain`. |
| `AUTH_COOKIE_SECURE` | `true`; usar `false` solo en desarrollo local HTTP. |

Usa el mismo `AUTH_ISSUER` y `AUTH_AUDIENCE` en autenticación y backend; el valor por defecto del backend para audience es `whatsapp-commerce-api`.
El backend requiere además `AUTH_JWKS_URL` (por defecto `http://localhost:8081/.well-known/jwks.json`), `INTERNAL_SERVICE_TOKEN` y `BACKEND_CORS_ALLOWED_ORIGINS` (por defecto `http://localhost:4200`). Configura el origen del frontend como un origen explícito; no uses `*`.

Access tokens duran 15 minutos y refresh tokens 7 días. La cookie de refresh es `HttpOnly`, `Secure` por defecto y `SameSite=Lax`. El secreto privado no se genera ni almacena en el repositorio.

## Desarrollo local

Ejemplo de generación de clave en un directorio local privado, fuera del repositorio:

```sh
mkdir -p "$HOME/.config/whatsapp-ai-auth"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 \
  -out "$HOME/.config/whatsapp-ai-auth/signing-key.pem"
chmod 600 "$HOME/.config/whatsapp-ai-auth/signing-key.pem"
```

Configura `.env` con los valores requeridos y ejecuta desde la raíz de `whatsapp-ai-autentication`:

```sh
go run ./cmd/auth-api
```

El proceso aplica las migraciones SQL versionadas al arrancar. No usa `AutoMigrate`.

## Rutas

| Método | Ruta | Comportamiento |
|---|---|---|
| `POST` | `/api/auth/register` | Crea la cuenta y solicita onboarding idempotente. Devuelve `201` si termina o `202` si queda pendiente. |
| `POST` | `/api/auth/login` | Devuelve el access token y establece la cookie de refresh. |
| `POST` | `/api/auth/refresh` | Rota el refresh token de la cookie. |
| `POST` | `/api/auth/logout` | Revoca la sesión vigente y elimina la cookie. |
| `GET` | `/.well-known/jwks.json` | Publica la clave pública para validación local de JWT. |
| `GET` | `/health/live` | Liveness. |
| `GET` | `/health/ready` | Comprueba conectividad a PostgreSQL. |

El backend debe implementar `POST /internal/auth/onboarding`, protegerlo con `X-Internal-Service-Token` e imponer idempotencia por `Idempotency-Key`. El endpoint recibe `authUserId`, `email`, `personalName` y `merchantName`; responde `userId` y `merchantId`. Las solicitudes de onboarding pendientes se reintentan cada 30 segundos.

## Pruebas

Para ejecutar el conjunto completo de validaciones del servicio:

```sh
./scripts/check.sh
```

El script compila el servicio, ejecuta las pruebas unitarias y de integración, corre `go vet` y verifica el formato Go con `gofmt`. Las pruebas de integración PostgreSQL usan Testcontainers y requieren Docker en ejecución.

También se pueden ejecutar solo las pruebas Go:

```sh
go test ./...
```

El proyecto no debe registrar contraseñas, tokens ni claves. Antes de un despliegue, configurar claves persistentes y rotación de claves coordinada con el endpoint JWKS.
