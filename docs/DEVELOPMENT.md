# Desarrollo y contratos

## Preparar y compilar

Go 1.27.3 y Node 24+, con dependencias fijadas. En un clon limpio:

```bash
npm ci --prefix web
node scripts/build.mjs
```

El script genera el contrato y sus tipos, compila la interfaz, verifica módulos y produce ejecutables y paquetes Linux/Windows x64 con `CGO_ENABLED=0`, rutas de compilación eliminadas y versión fijada. Los archivos empaquetados tienen metadatos temporales normalizados. Se incluye documentación, contrato y licencias de dependencias de runtime. `ERP_VERSION=0.4.0 node scripts/build.mjs` selecciona una versión; en PowerShell configura `$env:ERP_VERSION` antes de ejecutar. `--current` construye únicamente el sistema actual.

No se incorporan `node_modules`, datos, respaldos ni certificados de desarrollo al paquete. La salida del frontend se ignora en Git y debe generarse antes de ejecutar `go test ./...` o compilar el servidor desde un clon limpio.

Para desarrollar sin instalador ni servicio, prepara una vez los recursos y usa dos terminales desde la raíz del proyecto:

```bash
npm ci --prefix web
npm --prefix web run build
```

Terminal 1, servidor y SQLite locales (reinicia al cambiar Go):

```bash
go run ./cmd/erp serve --data-dir ./data-dev
```

Terminal 2, interfaz con recarga automática:

```bash
npm --prefix web run dev
```

Completa el alta con el enlace del backend en su primer inicio. Abre después `http://127.0.0.1:5173` para trabajar con Vite: escucha en loopback y reenvía `/api` al backend de puerto 8080. Puedes entrar desde varios navegadores con el usuario creado; cada navegador obtiene su propia sesión. Usa siempre el mismo host en cada navegador para conservar su cookie. El proxy conserva `Host` y `Origin` para que el backend valide el origen de cada escritura. Los datos quedan en `data-dev/`, separados de producción, y persisten al reiniciar. La producción utiliza los recursos incorporados, sin Vite. También puedes ejecutar el binario portable ya compilado en la terminal 1.

Si el alta funciona desde el backend pero iniciar sesión por Vite muestra «No tienes permiso para realizar esta operación», reinicia Vite con la configuración actual. La forma abreviada de proxy como una cadena activa `changeOrigin` y cambia `Host`, provocando el rechazo de la validación de origen. La configuración explícita usa `changeOrigin: false`; conserva las comprobaciones de CSRF y origen del servidor. No hace falta recrear la empresa ni el usuario.

## Contrato público

`api/openapi.mjs` y sus extensiones `api/commercial.mjs` y `api/peripherals.mjs` son la fuente del contrato. `node api/openapi.mjs` genera `api/openapi.json`; `npm --prefix web run generate` genera `web/src/api/schema.ts` mediante openapi-typescript. No edites los tipos generados manualmente. El administrador puede descargar el contrato servido por el ejecutable desde Administración.

La API utiliza `/api/v1`, cookie `erp_session`, token `X-CSRF-Token` en operaciones autenticadas de escritura y `Idempotency-Key` al confirmar ventas y registrar egresos de caja. Los errores incluyen `error.code`, `error.message` y `error.requestId`. Las respuestas 409 representan conflictos de negocio; 503, contención recuperable. Un cliente debe repetir una venta incierta con la misma clave y cuerpo.

El primer alta requiere `Authorization: Setup <token>` y conexión loopback. El inicio de sesión devuelve usuario, empresa y token CSRF; la cookie de sesión no se expone a JavaScript. Las listas admiten `search`, `page` y `pageSize`.

| Ampliación comercial | Rutas bajo `/api/v1` |
| --- | --- |
| Producto individual y edición con versión | `GET/PATCH /products/{id}` |
| Productos vendibles o de stock propio | `GET /products?sellable=true`, `GET /products?stockMode=own` |
| Conteo auditado con motivo y seis decimales | `POST /inventory/adjustments` |
| Promociones | `GET/POST /promotions`, `GET/PATCH /promotions/{id}` |
| Cotización con descuentos/regalos | `POST /sales/quote` |
| Confirmación con `quoteFingerprint` e idempotencia | `POST /sales` |
| Perfil y contraseña | `PATCH /auth/me`, `POST /auth/change-password` |
| Empresa, sin cambiar moneda | `PATCH /company` |
| Permisos por operador | `PATCH /users/{id}` con `canApplyManualDiscounts` o `canRecordCashExpenses` (uno por petición) |
| Caja, filtro por tipo y detalle | `GET /cash/movements?kind=sale|expense`, `GET /cash/movements/{id}` |
| Egreso con idempotencia | `POST /cash/expenses` con `amountMinor` positivo y `reason` |
| Código exacto y generación interna estable | `GET /products/lookup?barcode=...`, `POST /products/{id}/barcodes/generate` |
| Auditoría, catálogo de operaciones y detalle | `GET /audit?search=Ed&action=product.update`, `GET /audit/operations`, `GET /audit/{id}` |
| Perfil de impresión y dispositivos del servidor | `GET/PUT /pos/settings`, `GET /pos/devices` |
| Prueba y confirmación física | `POST /pos/tests`, `POST /pos/verify` |
| Trabajos (listado administrador; detalle de venta para operadores) | `GET /pos/jobs`, `GET /pos/jobs/{id}` |
| Copia y apertura manual con idempotencia | `POST /sales/{id}/print`, `POST /pos/drawer/open` |

Precios y montos fijos usan unidades monetarias menores; los porcentajes son cadenas con dos decimales como máximo. Al editar una promoción se envía su configuración completa con versión actual. El cálculo puro vive en `internal/pricing`; aplicación coordina lecturas consistentes y transacciones. La huella no reserva stock: saldos y permisos se reevalúan al confirmar. Una venta ya confirmada puede recuperarse con su cuerpo y clave originales sin volver a cotizar.

## Validación

```bash
npm --prefix web run build
npm --prefix web run test:proxy
go vet ./...
go test -race -v ./...
node scripts/smoke.mjs
```

El detector de carreras necesita un compilador C en la máquina de desarrollo; los ejecutables de distribución no lo necesitan.

La prueba portable copia únicamente el ejecutable a otra carpeta y lo inicia con PATH vacío. Verifica HTTPS con su autoridad local, recursos incorporados, venta, respaldo con el servidor activo, restauración, revocación de sesiones y reinicio. Se ejecuta nativamente en cada sistema de la matriz de integración continua.

Las pruebas comerciales emplean SQLite real en directorios temporales. Verifican exactitud, composiciones compartidas, políticas, promociones, permisos, huellas desactualizadas, concurrencia, idempotencia y reversión completa. El motor puro se prueba con cantidades grandes sin expandir unidades. Transporte verifica también perfil, empresa y contraseña con revocación. Operación verifica WAL, respaldo/restauración y migraciones reales de esquemas 1, 2 y 3 con reintentos y trabajos antiguos. Los casos de caja comprueban correlativos concurrentes, permisos frescos, saldo negativo, auditoría, idempotencia y reversión conjunta del cajón.

Para aceptación de navegador inicia un servidor con un directorio **nuevo y exclusivo de pruebas** y ejecuta:

```bash
npx --prefix web playwright install chromium
ERP_TEST_DATA_DIR=/ruta/absoluta/prueba/data npm --prefix web run test:e2e
```

El servidor debe usar el puerto 8080 o se debe definir `ERP_BASE_URL`. La suite lee el token del directorio nuevo, crea la empresa y ejecuta secuencialmente el flujo base y la ampliación: controles decimales, insumos, regalos, descuentos, historial, conteos, permisos y cuenta. Bloquea orígenes externos y genera capturas de escritorio/móvil. Conserva trazas ante fallos. `ERP_BROWSER_EXECUTABLE=/ruta/a/chromium` permite un navegador ya disponible. Cada ejecución completa requiere una instalación de pruebas nueva. La suite incluye códigos externos/generados, exportaciones SVG/PNG/ZIP, lectura contextual, auditoría con diferencias y cierre después de imprimir. El flujo de ajustes de impresión utiliza respuestas simuladas; los casos Go prueban una cola SQLite real con un transporte inyectado, sin enviar datos a equipos físicos.

## Agregar un módulo

Define modelos y puertos en su paquete de módulo; implementa adaptadores SQL en la infraestructura y un caso de uso en aplicación. Agrega una migración consecutiva y actualiza la versión de esquema. Expón el contrato HTTP y regenera los tipos; añade la interfaz cuando su comportamiento esté disponible. Las operaciones que cambien inventario o dinero deben incluir auditoría y compartir la unidad de trabajo.

Los efectos críticos son síncronos. Si una integración externa necesita envío garantizado, agrega un outbox persistente en la misma transacción y un procesador con reintentos e idempotencia; no uses goroutines sin persistencia para entregar documentos o alterar saldos.

## Periféricos y simulación

`internal/modules/pos` define perfiles, trabajos y renderizado ESC/POS puro. `internal/adapters/printing` implementa la enumeración y envío RAW de Windows y dispositivos USB de Linux, con archivos separados por sistema y sin CGO. El caso de uso recibe un `pos.Transport`, lo que permite probar envíos completos, parciales e interrumpidos sin una impresora conectada.

La cola se confirma con la venta; el trabajador no mantiene una transacción abierta mientras habla con el equipo. Las transiciones y resultados se auditan. Una escritura posiblemente parcial queda incierta y no se reenvía. El perfil exige prueba enviada y confirmación física por administrador para activar automatismos; el cliente no puede habilitar sus propios indicadores de verificación.

No añadas efectos de impresión en el navegador al recibir una venta: un reintento o una pestaña cerrada no debe controlar si el servidor duplica trabajo. El navegador consulta `printJobId` y estados. SVG/PNG/ZIP de códigos se generan localmente con dependencias incorporadas y respetan la CSP sin abrir acceso a imágenes externas.

Las altas y cambios de contraseña aceptan `allowWeakPassword: true` como aceptación expresa para menos de doce caracteres, incluido vacío. Las cadenas no se recortan; el máximo es 128 caracteres Unicode. La recomendación no impone complejidad adicional. La autenticación continúa comparando Argon2id, y los cambios revocan sesiones.

El navegador conserva el egreso pendiente en localStorage por identificador de usuario antes del primer envío. Tras una respuesta incierta congela cuerpo y clave para reintentar, incluso después de recargar o cambiar de cuenta. El saldo proyectado se muestra solo antes del primer envío para no volver a restar un egreso ya confirmado cuya respuesta se perdió.

## Navegación de catálogo y existencias

La sección **Productos** concentra catálogo y operaciones de existencias. La navegación anterior `#/inventory` se normaliza a `#/products` con `history.replaceState`, conservando el acceso desde marcadores y el comportamiento de Atrás. El listado usa `/products`; los endpoints `/inventory/stocks`, `/inventory/receipts` y `/inventory/adjustments` conservan sus contratos y permisos. No hay migración de datos asociada a esta simplificación.

Playwright comprueba el enlace antiguo, el acceso rápido de existencias, el escáner/F2 por rol y las acciones de descuento total alineadas en escritorio y móviles de 390 y 320 px. Verifica además que quitar el descuento total conserva descuentos por línea, regalos y motivo, y que un carrito enviado mantiene bloqueadas estas acciones.
