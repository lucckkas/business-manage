# Raíz ERP

Una base de ERP local, modular y portable para pequeñas empresas. Un servidor por negocio, hasta diez sesiones simultáneas como objetivo de validación y acceso mediante navegador. Los datos permanecen en la empresa y la operación no requiere Internet.

## Empezar con un paquete portable

1. Descomprime el paquete correspondiente a tu sistema en una carpeta con permiso de escritura.
2. Ejecuta `erp.exe` en Windows o `./erp` en Linux desde esa carpeta.
3. Abre **en el propio equipo servidor** el enlace de alta que muestra la consola. El código del enlace es de uso único.
4. Configura empresa, moneda, zona horaria y administrador. Después inicia sesión.
5. Crea un producto, ingresa existencias y confirma una venta. Inventario, caja y auditoría se actualizan juntos.

El primer arranque escucha en `http://127.0.0.1:8080`. Para compartirlo con otros equipos sigue [Instalación y HTTPS](INSTALLATION.md). El ejecutable incluye interfaz, base SQLite, migraciones y zonas horarias; no requiere Go, Node, Docker ni un servidor de base de datos en el equipo del negocio.

## Ejecutar desde el código

Requiere Go 1.27.3 y Node 24 o superior durante el desarrollo:

```bash
npm ci --prefix web
node scripts/build.mjs
./artifacts/erp-linux-amd64 serve --data-dir ./data
```

En Windows utiliza `artifacts\erp-windows-amd64.exe`. Los paquetes y `SHA256SUMS` quedan en `artifacts/`. La primera compilación necesita Internet para obtener dependencias; los paquetes resultantes funcionan sin conexión.

## Documentación

- [Arquitectura, límites y decisiones](ARCHITECTURE.md)
- [Modelo de datos e integridad](DATA.md)
- [Productos, insumos, ofertas y ajustes de cuenta](COMMERCIAL.md)
- [Egresos de caja y contraseñas opcionales](CASH_AND_ACCESS.md)
- [Auditoría, lector, códigos exportables, impresora y cajón](AUDIT_AND_DEVICES.md)
- [Instalación, HTTPS y servicios](INSTALLATION.md)
- [Respaldos, recuperación y actualizaciones](OPERATIONS.md)
- [Desarrollo, contratos y pruebas](DEVELOPMENT.md)
- [Crecimiento y módulos siguientes](EVOLUTION.md)
- [Alcance y evidencia de validación](VALIDATION.md)
- [Dependencias y distribución](THIRD_PARTY.md)

## Alcance de esta entrega

Funcionales: alta inicial, usuarios y permisos, catálogo con varios códigos de barras, lector USB HID, exportación SVG/PNG/ZIP, insumos compartidos, cuatro políticas de inventario, ingresos y ajustes auditados, promociones, descuentos manuales, cotización y venta al contado, comprobantes históricos, impresión ESC/POS y cajón desde el servidor, caja con egresos auditados y permisos por usuario, contraseñas opcionales con aceptación, informes, auditoría detallada, ajustes de cuenta/empresa, respaldos y comandos operativos. Una empresa, una bodega y una moneda por instalación. Ventas y factores admiten tres decimales; inventario, seis. Los periféricos permanecen desactivados hasta verificar físicamente sus pruebas.

Clientes/proveedores y compras tienen límites y contratos definidos para su siguiente implementación. Contabilidad de doble partida, documentos tributarios, cuentas por cobrar/pagar, devoluciones, impuestos y sincronización entre sedes pertenecen a etapas posteriores. Tampoco se incluyen packs de precio cerrado, tramos, cupones ni beneficios entre ventas. Los comprobantes actuales son registros de gestión interna.
