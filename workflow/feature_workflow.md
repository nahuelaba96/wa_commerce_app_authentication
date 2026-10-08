# Feature Development Workflow

Este documento define el workflow para implementar features, corregir bugs o modificar pruebas en el microservicio de autenticación Go.

El workflow es:

DISCOVER → PLAN → HUMAN APPROVAL → BASELINE → IMPLEMENT → VALIDATE → HUMAN REVIEW

Si durante IMPLEMENT o VALIDATE aparece una decisión fuera del alcance aprobado, detenerse y ejecutar ESCALATION.

---

# 1. DISCOVER

Objetivo: comprender el requerimiento y el estado actual del servicio antes de proponer una solución.

Durante esta fase el agente puede leer e inspeccionar el proyecto, pero no modificar archivos.

Debe:

- Leer `AGENTS.md` y la documentación relevante, incluidos `README.md` y los documentos de `workflow/` relacionados con la tarea.
- Inspeccionar la estructura del servicio y los paquetes Go afectados.
- Revisar los casos de uso, dominio, puertos y adaptadores relacionados.
- Buscar implementaciones y pruebas existentes del comportamiento solicitado.
- Revisar contratos HTTP, claims JWT, configuración o esquema de base de datos afectados.
- Revisar `go.mod` y `go.sum` cuando la tarea involucre dependencias.
- Identificar impactos en el backend o frontend que consumen este servicio.
- Identificar requisitos externos para las pruebas, especialmente PostgreSQL y Docker.
- Identificar decisiones que requieran aprobación humana.

No asumir que el usuario, cuenta o identificador de este servicio es equivalente al del backend. No leer ni mostrar secretos de `.env`.

Al finalizar, debe contar con información suficiente para elaborar un PLAN.

---

# 2. PLAN

Antes de modificar código o pruebas, presentar un plan que incluya como mínimo:

## Feature / Problem

Descripción breve del problema o funcionalidad solicitada.

## Proposed Solution

Descripción de la solución propuesta, respetando la arquitectura hexagonal y los patrones existentes.

## Decisions

Decisiones técnicas relevantes y su justificación, en particular si afectan autenticación, sesiones, seguridad o compatibilidad entre servicios.

## Affected Components

Paquetes, migraciones, documentación y pruebas que probablemente serán modificados.

## Proposed Changes

Cambios concretos que se realizarán.

## Approved Scope

Lista explícita de archivos y cambios autorizados.

## Out of Scope

Cambios que no forman parte de la tarea.

## Tests and Validation

Pruebas y comandos de validación que deberán ejecutarse.

## Risks and Assumptions

Riesgos, supuestos, dependencias externas y decisiones que podrían requerir intervención humana.

---

# 3. HUMAN APPROVAL

Después de presentar el PLAN, detenerse y esperar aprobación humana explícita. La solicitud original de la feature no constituye por sí sola la aprobación del PLAN.

Ejemplos de aprobación válida:

- "Aprobado"
- "Aprobado el plan"
- "Procedé con ese plan"

Si el humano modifica el PLAN:

1. Actualizar el PLAN.
2. Presentarlo nuevamente.
3. Esperar una nueva aprobación explícita.

Si el humano rechaza el PLAN, detenerse y solicitar nuevas instrucciones o proponer una alternativa cuando corresponda.

---

# 4. BASELINE

Después de recibir aprobación y antes de modificar archivos, ejecutar las validaciones pertinentes para conocer el estado inicial del servicio.

Usar los comandos disponibles para el proyecto:

```sh
go test ./...
go build ./cmd/auth-api
```

Las pruebas de integración PostgreSQL pueden requerir Docker. No omitirlas silenciosamente: si no se pueden ejecutar, registrar el requisito que falta y el resultado del comando.

Registrar:

- comandos ejecutados y sus códigos de salida;
- resultado de cada validación;
- pruebas fallidas y errores existentes.

Identificar los fallos previos como **PRE-EXISTING FAILURES**; no atribuirlos a los cambios posteriores. Si el baseline no puede ejecutarse, informar el motivo antes de continuar.

---

# 5. IMPLEMENT

Una vez aprobado el PLAN y registrado el BASELINE, implementar únicamente los cambios definidos en el alcance aprobado.

Debe:

- Respetar `AGENTS.md` y la arquitectura hexagonal existente.
- Mantener las reglas de negocio en `internal/usecase` y `internal/domain`; mantener infraestructura en los adaptadores detrás de los puertos.
- Usar `context.Context` para operaciones de persistencia y llamadas HTTP.
- Crear o actualizar pruebas que cubran el comportamiento modificado.
- Formatear los archivos Go modificados con `gofmt`.
- Actualizar documentación de configuración o contratos cuando corresponda; nunca añadir secretos a `README.md` o `.env.example`.
- Evitar cambios no relacionados.

No debilitar las garantías de seguridad existentes: Argon2id para contraseñas; firma RS256 y compatibilidad de claims/JWKS; refresh tokens almacenados como hashes, rotación y detección de reutilización; cookies de refresh `HttpOnly`, `Secure` por defecto y `SameSite=Lax`; CORS con orígenes explícitos; token interno e idempotencia para onboarding.

Los cambios de esquema deben hacerse mediante migraciones SQL versionadas `up`/`down`; no usar GORM `AutoMigrate`.

---

# 6. SCOPE CONTROL

Verificar durante la implementación que los cambios permanecen dentro del alcance aprobado.

Detenerse y escalar si se descubre que hace falta:

- Cambiar un contrato HTTP compartido, claims JWT, expiraciones o políticas de autenticación.
- Cambiar reglas de CORS, credenciales internas o el protocolo de onboarding.
- Modificar el esquema o los datos existentes, o añadir/aplicar una migración no incluida en el plan aprobado.
- Agregar o actualizar dependencias.
- Alterar la arquitectura o los límites entre servicios.
- Modificar configuración sensible, acceder a credenciales o servicios externos, desplegar o hacer `git push`.
- Realizar cambios fuera del alcance aprobado o tomar una decisión técnica relevante no contemplada.

No realizar esos cambios automáticamente. Ejecutar el proceso de ESCALATION.

---

# 7. ESCALATION

Cuando una decisión exceda la autoridad o el alcance aprobado:

1. Detener la parte de la implementación afectada.
2. Explicar el problema y por qué no se puede continuar autónomamente.
3. Presentar alternativas con sus ventajas, desventajas, impacto y riesgos.
4. Recomendar una opción cuando haya información suficiente.
5. Solicitar una decisión humana explícita y esperar antes de continuar.

Si la decisión modifica el PLAN o el alcance aprobado, actualizarlos, presentarlos nuevamente y esperar su aprobación. Aprobar una escalación no autoriza automáticamente otros cambios relacionados.

---

# 8. VALIDATE

Después de implementar, ejecutar las validaciones pertinentes. Para cambios Go, como mínimo:

```sh
go test ./...
go build ./cmd/auth-api
```

Además:

- Formatear los archivos Go modificados con `gofmt` y verificar que no queden diferencias de formato.
- Para cambios de persistencia o integración, ejecutar las pruebas de integración aplicables con sus requisitos externos disponibles.
- Revisar los cambios para confirmar que solo incluyan archivos dentro del alcance aprobado, que las dependencias y configuración no hayan cambiado sin autorización y que no haya secretos.
- No ejecutar migraciones contra bases compartidas o productivas como parte de una validación local.

---

# 9. VALIDATION REPORT

Al terminar, informar un reporte con los comandos realmente ejecutados y sus resultados, por ejemplo:

```text
VALIDATION REPORT

Tests:
PASS - go test ./...

Build:
PASS - go build ./cmd/auth-api

Formatting:
PASS - gofmt sobre los archivos Go modificados

Scope verification:
PASS - cambios dentro del alcance aprobado

Final result:
PASS
```

No afirmar que una validación pasó si no se ejecutó y obtuvo resultado exitoso. Si no fue posible ejecutarla, indicar cuál, por qué, qué comando se intentó y el resultado obtenido. No inventar resultados.

---

# 10. HUMAN REVIEW

Presentar un resumen de los cambios y el reporte de validación para revisión humana. Destacar contratos, configuración, seguridad o migraciones afectados, además de cualquier validación pendiente o fallo previo. No desplegar ni publicar cambios sin la aprobación requerida.
