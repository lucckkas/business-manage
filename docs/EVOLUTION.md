# Evolución del ERP

## Módulos siguientes

Los paquetes de contactos y compras definen contratos internos de extensión y no exponen rutas simuladas.

| Etapa | Capacidad | Integración necesaria |
| --- | --- | --- |
| 1 · Base actual | Catálogo editable, insumos, políticas de stock, ofertas, descuentos, venta al contado, caja y ajustes | Cotización autoritativa y transacción única con instantáneas y auditoría |
| 2 · Contactos y compras | Clientes/proveedores, documentos de compra y recepción | `contacts.Directory`, `purchasing.Receiving`; recepción y movimientos de inventario atómicos |
| 3 · Crédito y devoluciones | Saldos por cobrar/pagar, pagos parciales y reversos | Libros de movimientos, claves de idempotencia y trazabilidad al documento original |
| 4 · Gestión ampliada | Varias bodegas, informes y costos | Traslados atómicos, índices y modelo de costos definido antes de estados de resultados |
| 5 · Localización | Documentos tributarios y contabilidad | Adaptadores por país, outbox, validación de requisitos y reglas de contabilización |

Un contacto identifica a la contraparte y sus roles de cliente/proveedor. Una orden de compra contiene proveedor, líneas con cantidades y costo unitario exactos y estado. La recepción coordina compra, inventario y auditoría; cuando exista crédito, también el saldo por pagar. Los endpoints nuevos se incorporarán a OpenAPI al implementarse, con permisos y pruebas de transacción.

La contabilidad formal requiere plan de cuentas, períodos, asientos balanceados y cierres; el módulo de caja actual registra ingresos de ventas y no pretende sustituir estos libros. La integración tributaria se diseña para el país elegido, separada del documento comercial interno.

## Crecimiento técnico

Primero medir p95 de peticiones, tiempo de transacción, errores `busy`, tamaño de base y duración de respaldos con datos y hardware reales. La optimización inicial es índices y consultas, paginación y ventanas de informes. La capacidad depende de frecuencia y duración de escrituras, no solo de usuarios registrados.

Cuando las escrituras requieran mayor concurrencia, implementar repositorios PostgreSQL con el mismo contrato de negocio y adaptar `WithinTx`. Conservar enteros, reglas de redondeo e identificadores. Cambiar consultas, restricciones, correlativos y bloqueos según el motor; el cambio no consiste en modificar una cadena de conexión. Las mismas pruebas de negocio deberán ejecutarse sobre ambos adaptadores.

## Migración SQLite → PostgreSQL

1. Preparar el esquema PostgreSQL y probar la carga en una copia del respaldo.
2. Programar mantenimiento y detener todas las escrituras. Generar y verificar un respaldo.
3. Exportar tablas por identificador y relaciones; cargar en orden de dependencias dentro de una transacción.
4. Mantener los correlativos internos y posicionar las secuencias por encima del máximo migrado.
5. Comparar recuentos y referencias; verificar cada stock contra ingresos, consumos, conteos y compensaciones y cada ingreso de caja contra el neto de ventas, los egresos por separado y el saldo contra ingresos menos egresos. Conservar composiciones, promociones, instantáneas y permisos. Comparar escalas exactas (cantidades tres decimales; inventario seis), nunca conversiones de coma flotante.
6. Ejecutar pruebas de lectura, autenticación y una operación de prueba reversible en el entorno migrado.
7. Activar el adaptador PostgreSQL y supervisar errores, latencia y saldos. Conservar SQLite detenido durante la verificación.

Antes de reabrir escrituras se puede volver a la instalación anterior. Después de aceptar escrituras en el nuevo motor, una vuelta requiere reconciliar esas operaciones; no se vuelve a una copia anterior silenciosamente.

PostgreSQL permite posteriormente varias instancias de aplicación. Las tareas programadas deberán adquirir un bloqueo compartido y los recursos externos requerirán almacenamiento accesible a esas instancias. La extracción de servicios se evaluará cuando un módulo necesite despliegue o capacidad independiente. Varias sedes que trabajen sin conexión necesitan otro diseño de sincronización y resolución de conflictos.
