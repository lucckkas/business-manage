# Alcance y validación

## Aceptación automatizada

- Cantidades de venta y factores a tres decimales, inventario a seis; consumos exactos como `0,25 × 0,125 = 0.031250`.
- Controles numéricos con botones/flechas de paso uno y escritura con coma o punto.
- Composiciones compartidas, regalos que consumen insumos y una salida agrupada por recurso.
- Cuatro políticas de stock, compensaciones, conteos auditados y reactivación con saldo contado.
- Promociones de porcentaje/monto, lleva/paga y regalos adicionales; prioridad, acumulación, inicio incluido y fin excluido.
- Descuentos manuales por línea/unidad/total sobre ofertas no acumulables; permiso, motivo, actor y reparto exacto por restos mayores.
- Cantidades grandes sin expansión por unidad y promociones fijas limitadas al importe disponible.
- Cotizaciones desactualizadas y recuperación inmutable después de expirar ofertas.
- Edición de catálogo y empresa sin modificar comprobantes anteriores.
- Venta completa e idempotencia persistente tras reabrir SQLite.
- Diez escrituras concurrentes sin stock negativo ni ventas por encima de existencias.
- Diez reintentos simultáneos con una sola venta, salida e ingreso.
- Fallo inducido en auditoría que revierte documento, detalle, stock y caja.
- Restricciones de operador, desactivación de cuentas, revocación y cierre de sesión.
- Alta inicial desde loopback, token consumido, cookie segura y protección CSRF/origen.
- Diez sesiones autenticadas enviando ventas simultáneas; se registran duración, p95 y memoria del runtime Go.
- Respaldo consistente con WAL, restauración, conservación del directorio anterior y rechazo de archivos inválidos.
- Migraciones reales desde esquemas 1, 2 y 3 hasta 4 con respaldo previo, movimientos históricos exactos, correlativos, reintentos y trabajos de impresora conservados.
- Cambio de perfil, empresa y contraseña con verificación de la actual y revocación de todas las sesiones.
- Cambio de cuenta con carritos separados por usuario, incluidos descuentos y regalos pendientes.
- Flujo de navegador desde alta hasta venta y consulta de stock/caja, con orígenes externos bloqueados.
- Compilación sin CGO y paquetes con checksums para Linux y Windows.
- Auditoría con filtros en español, coincidencias parciales, diferencias, referencias y nombres de actor conservados.
- Códigos externos/generados únicos y estables, validación EAN/UPC y conservación de ceros iniciales.
- Escaneo contextual consecutivo, vinculación de códigos desconocidos y rechazo íntegro de lecturas demasiado largas.
- Exportaciones SVG/PNG/ZIP sin conexión y cierre del comprobante tras el diálogo de impresión.
- Cola de impresión atómica, reintentos idempotentes, serialización, copias sin pulso, fallos parciales y recuperación de envíos interrumpidos.
- Restauración que suspende trabajos y exige nuevas pruebas de periféricos; confirmación física protegida contra configuraciones obsoletas o indicadores enviados por el cliente.

- Egresos con montos exactos, correlativos concurrentes, permiso por operador, saldos negativos y auditoría; reintentos sin duplicar ni movimiento ni apertura.
- Inicio de sesión con contraseña corta o vacía, aceptación explícita, verificación de la actual y revocación de sesiones.
- Motivo visible dentro del diálogo de descuento y resumen persistente en el carrito.

## Límites de la evidencia

La prueba de diez sesiones usa transporte HTTP en memoria y SQLite real; demuestra coherencia y mide el servidor sin incluir latencia física de LAN. Las pruebas de navegador utilizan el servidor real en loopback. No equivalen a una certificación de rendimiento sobre el hardware del negocio.

La integración continua ejecuta pruebas nativas en Linux y Windows y aceptación de navegador en Linux. La generación cruzada de un `.exe` comprueba compilación; su funcionamiento como servicio debe verificarse en Windows. La instalación de servicios modifica el sistema y se verifica en un equipo o máquina virtual destinada a instalación, no en el entorno de desarrollo compartido.

## Comprobación de instalación antes de distribuir

1. En equipos Windows/Linux de prueba sin Go ni Node, verificar SHA-256 y descomprimir.
2. Arrancar, completar el alta local, confiar en la autoridad y conectarse desde otro equipo de la red.
3. Registrar producto, existencias y venta; comprobar el resultado y la impresión.
4. Reiniciar servidor y equipo. Verificar persistencia y arranque automático si se instaló servicio.
5. Respaldar, detener, restaurar en otra carpeta y comprobar datos y nuevas sesiones.
6. Desconectar Internet conservando la LAN y repetir el flujo.
7. Medir con diez usuarios y datos representativos del negocio antes de dimensionar producción.

## Verificación local del 30 de octubre de 2026 · versión 0.2

Ejecutada en Linux x64 con Go 1.27.3:

- `go vet ./...` y `go test -race ./...`: correctos.
- Compilación y empaquetado Linux y Windows x64: correctos; binario Linux estático y binario Windows PE64, ambos sin CGO.
- Playwright: dos escenarios completos sobre el ejecutable portable. Alta/venta/reintento tras respuesta perdida e insumos/ofertas/regalos/descuentos/edición histórica/conteo/permisos/cambio de cuenta/contraseña; correctos con orígenes externos bloqueados. Capturas de escritorio y móvil sin desbordamiento de página.
- Prueba portable con PATH vacío: HTTPS, interfaz incorporada, venta, respaldo en línea, restauración de la copia más antigua y reinicio con sesiones revocadas; correcta.
- Diez sesiones autenticadas con ventas concurrentes en SQLite real: 24,7 ms totales, p95 de 21,8 ms, heap Go de 65,0 MiB y memoria del runtime de 142,2 MiB en una ejecución local sin detector de carreras. Son medidas puntuales del entorno de desarrollo, no una capacidad garantizada para hardware de producción.
- Migración desde una base de esquema 1 con empresa, usuario, ingreso, venta, caja y auditoría: integridad, cantidades, historial, siguiente correlativo y reintento antiguo correctos.

No se instaló un servicio del sistema ni se ejecutó Windows en este entorno. El workflow incluye compilación, prueba portable y pruebas Go nativas en ambos sistemas. La guía anterior cubre la verificación final de instalación en equipos de destino.

## Verificación local del 9 de noviembre de 2026 · versión 0.3

- `go test -race ./...`, `go vet ./...`, generación de tipos y compilación TypeScript/Vite: correctos.
- Cuatro escenarios Playwright sobre el ejecutable portable: regresiones comerciales, insumos/descuentos/cuenta, lector/exportaciones/auditoría/impresión y configuración guiada de periféricos. Correctos con 15,8 segundos totales en la ejecución final. La exportación funciona con la CSP original, sin habilitar imágenes externas.
- Prueba portable Linux con PATH vacío: HTTPS, interfaz incorporada, venta, respaldos, restauración de la copia más antigua y reinicio correctos.
- Migraciones desde esquemas 1 y 2: respaldo previo, cantidades, historial, correlativos e idempotencia conservados. Restaurar suspende los trabajos pendientes e invalida las verificaciones físicas.
- Transporte de impresora simulado con SQLite real: una tarea automática por venta, copias sin apertura, idempotencia de apertura manual, exclusión entre trabajadores, fallos parciales sin reenvío y rollback de venta/tarea ante fallo de auditoría.
- Paquetes Linux y Windows x64 compilados sin CGO, con bibliotecas y licencias incorporadas. Los paquetes normalizan metadatos para reproducibilidad y publican sumas SHA-256.

La prueba visual de configuración utiliza un transporte simulado y el resto de casos no contacta con equipos físicos. No se ejecutó Windows ni se instaló un servicio en este entorno. La compatibilidad del modelo concreto de Xprinter, el lector HID y el cajón requiere las pruebas físicas descritas en [Auditoría y periféricos](AUDIT_AND_DEVICES.md).

## Verificación local del 11 de noviembre de 2026 · versión 0.4

- `go test -race ./...` y `go vet ./...`: correctos, incluidos los casos nuevos de caja, contraseñas, migración y restauración.
- Regresión del proxy de desarrollo: conservación de `Host`/`Origin` con localhost y 127.0.0.1; alta en el backend y acceso desde otra sesión de navegador por Vite, escritura con CSRF y cierre independiente de sesión: correctos. Los orígenes ajenos continúan rechazándose.
- Generación OpenAPI/tipos, TypeScript y compilación Vite: correctos. Formato de importes firmados verificado también con centavos negativos, incluido `USD −0,01`.
- Cinco escenarios Playwright completos sobre el ejecutable portable: correctos. El nuevo escenario registra un egreso que deja saldo negativo, pierde su respuesta y lo recupera tras recargar con el mismo cuerpo/clave; consulta el detalle y auditoría, habilita el permiso por usuario y prueba inicio/cambio de contraseña vacía y corta con revocación de otra sesión. La suite reserva una ventana de un minuto para respetar el límite de diez inicios de sesión por minuto; no se modifica el limitador de producción.
- Escritorio y móvil sin desbordamiento de página; capturas del diálogo de descuento, detalle de egreso y Caja móvil.
- Prueba portable Linux con PATH vacío: HTTPS, recursos incorporados, venta, egreso, respaldo, restauración de la copia más antigua, idempotencia persistente, reinicio y recuperación por consola con contraseña vacía: correctos.
- Migración real desde esquemas 1, 2 y 3: respaldo previo, saldos, correlativos, reintentos antiguos y trabajo ESC/POS pendiente conservados. Restaurar preserva el egreso y suspende su orden de cajón.
- Transporte simulado: pulso de cajón único sin ticket por egreso, reversión completa ante fallo tardío de auditoría, apertura desactivada, envío parcial e interrupción sin reenvío automático.
- Ejecutables Linux y Windows x64 compilados sin CGO y paquetes portables 0.4.0 con documentación, licencias y SHA-256.

No se ejecutó Windows ni se conectaron una Xprinter o cajón físicos en este entorno. La compilación cruzada y el transporte simulado no sustituyen la prueba física descrita en [Auditoría y periféricos](AUDIT_AND_DEVICES.md).

## Verificación de navegación y descuento total · 11 de noviembre de 2026

- TypeScript y compilación Vite: correctos; recursos de la interfaz incorporados al ejecutable de prueba.
- Cinco escenarios Playwright sobre el servidor real y datos temporales: correctos en la ejecución final (1,4 minutos, incluida la ventana del limitador de acceso).
- Una única entrada **Productos** con precio y existencias; acceso rápido para ingresos y enlaces antiguos `#/inventory` redirigidos mediante reemplazo, con Atrás sin pasos adicionales.
- Ingresos, ajustes, edición y escáner/F2 desde Productos; el operador consulta y escanea sin recibir controles de edición o movimientos de inventario.
- Aplicación, edición y eliminación de descuentos totales porcentuales y fijos, conservando promociones, descuentos por línea, regalos y motivo; acciones bloqueadas durante la recuperación de una venta enviada.
- Disposición de los botones de edición y eliminación verificada con medición de sus posiciones en escritorio y pantallas de 390 y 320 px, con separación y sin desbordamientos. Capturas de Productos y descuento total revisadas.

La simplificación conserva los endpoints y el esquema de datos existentes.
