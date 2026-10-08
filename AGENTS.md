# Mission

Mantener y evolucionar el microservicio de autenticación de **WhatsApp IA Commerce**, implementado en Go. Respetar los contratos HTTP, la arquitectura hexagonal y las garantías de seguridad de cuentas y sesiones.

El servicio ofrece registro, login, renovación y revocación de sesiones; persiste en PostgreSQL con GORM y migraciones SQL versionadas; firma access tokens JWT con RS256 y publica la clave pública mediante JWKS; e integra el onboarding interno con el backend.

Consultar `README.md` y `.env.example` para conocer configuración y rutas. No leer, copiar ni mostrar valores secretos de `.env`.

# Architecture

El servicio sigue arquitectura hexagonal:

- `cmd/auth-api`: composición de dependencias, migraciones y ciclo de vida HTTP.
- `internal/domain`: entidades, tipos y reglas del dominio.
- `internal/ports`: interfaces que definen los límites con infraestructura.
- `internal/usecase`: flujos de registro, login, sesiones y onboarding.
- `internal/adapters/transport/http`: endpoints, validación de solicitudes, cookies y CORS.
- `internal/adapters/persistence`: repositorios y mapeo a PostgreSQL/GORM.
- `internal/adapters/cryptography`: hash de contraseñas y firma/JWKS.
- `internal/adapters/backend`: cliente HTTP para onboarding en el backend.
- `internal/config`: carga y validación de configuración.
- `migrations`: cambios explícitos y versionados del esquema.

Mantener el dominio y los casos de uso independientes de HTTP, PostgreSQL y proveedores externos. Inyectar dependencias mediante los puertos existentes; no introducir acceso directo a infraestructura desde el dominio.

# Autonomous Actions

El agente puede:

- Leer código y documentación del microservicio, buscar referencias y ejecutar pruebas o compilaciones locales.
- Modificar código y pruebas dentro del microservicio y del alcance de la tarea.
- Añadir o actualizar pruebas para cambios de comportamiento; preferir pruebas unitarias en el paquete afectado y pruebas de integración cuando cambie la persistencia o su interacción con PostgreSQL.
- Actualizar `README.md` o `.env.example` cuando cambien rutas, contratos o configuración. `.env.example` debe contener solo valores de ejemplo no sensibles.

Al implementar cambios:

- Seguir los paquetes, interfaces, nombres y manejo de errores existentes.
- Mantener la lógica de negocio en `internal/usecase` y las reglas puras en `internal/domain`.
- Usar `context.Context` para operaciones de persistencia y llamadas HTTP.
- Cambiar el esquema únicamente mediante migraciones SQL `up`/`down`; no usar `AutoMigrate`.
- Mantener coordinados los contratos de autenticación con el backend y el frontend que consumen este servicio.

Preservar estas garantías:

- Hash de contraseñas con Argon2id; nunca almacenar ni registrar contraseñas en texto claro.
- Access tokens firmados con RS256. Mantener la compatibilidad de `issuer`, `audience` y `kid` con los validadores del backend y el endpoint JWKS.
- Refresh tokens opacos para el cliente; persistir solo sus hashes, rotarlos en cada renovación y revocar la sesión ante detección de reutilización.
- Cookie de refresh `HttpOnly`, `Secure` por defecto y `SameSite=Lax`. No exponer el refresh token en cuerpos de respuesta ni en logs.
- CORS limitado a orígenes explícitos; no aceptar `*` cuando se permiten credenciales.
- Llamada de onboarding protegida con el token interno y una clave de idempotencia estable.
- No asumir que el identificador de usuario de autenticación es el identificador de usuario del backend.

# Actions Requiring Approval

Solicitar aprobación humana antes de:

- Cambiar contratos de autenticación, claims JWT, expiraciones, política de contraseñas, rotación o revocación de sesiones, o reglas de CORS.
- Modificar o eliminar datos o esquemas existentes, o aplicar cambios destructivos de base de datos.
- Cambiar la integración de confianza con el backend, sus credenciales o el protocolo de onboarding.
- Agregar dependencias o alterar la arquitectura y los límites entre servicios.
- Usar credenciales, acceder a servicios externos, desplegar o publicar cambios.

# Forbidden Actions

- Exponer secretos, contraseñas, tokens privados o claves de firma.
- Incluir secretos en código, logs, documentación o `.env.example`.
- Desactivar controles de seguridad o quitar pruebas para conseguir una ejecución exitosa.
- Borrar bases de datos, ejecutar migraciones destructivas o cambiar datos fuera del alcance aprobado.
- Modificar archivos fuera del servicio o realizar cambios no relacionados con la tarea.

# Validation

- Ejecutar `go test ./...` para validar los paquetes del servicio.
- Para cambios relevantes de integración o persistencia, verificar también las pruebas de integración; requieren Docker.
- Ejecutar `go build ./cmd/auth-api` cuando los cambios afecten el arranque, la composición de dependencias o la compilación del servicio.
- Informar los comandos ejecutados y sus resultados. No afirmar que una validación pasó si no se ejecutó.
- Si una prueba no puede ejecutarse por falta de un requisito externo, comunicarlo explícitamente.

# Escalation

Detenerse y solicitar una decisión humana si el cambio requiere una acción de aprobación, altera un contrato compartido sin una decisión coordinada o presenta una ambigüedad que afecta la seguridad, las sesiones o la compatibilidad.

Al escalar:

1. Explicar el bloqueo y por qué no se puede continuar autónomamente.
2. Presentar alternativas y sus consecuencias cuando existan.
3. Recomendar una opción si hay información suficiente.
4. Esperar la decisión antes de continuar.
