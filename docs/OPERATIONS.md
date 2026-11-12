# Operación, respaldos y recuperación

## Directorios

```text
data/
  config.json
  erp.db
  erp.db-wal / erp.db-shm  (mientras SQLite los necesita)
  certificates/
  logs/
data-backups/
data.lock
```

La configuración contiene dirección/puerto, TLS, carpeta de respaldos y retención. La carpeta de respaldos debe estar fuera de `data/`; puede residir en otra unidad. Los respaldos contienen datos empresariales y claves de certificados y requieren los mismos permisos de acceso que los datos originales.

## Respaldo

Una vez configurada la empresa, el proceso comprueba cada minuto si el último respaldo tiene al menos 24 horas. Si el servidor estuvo apagado, recupera la tarea pendiente al volver a operar. Conserva las últimas siete copias por defecto. El administrador también puede solicitar una desde la interfaz.

```bash
./erp backup --data-dir ./data
```

Este comando funciona con el servidor activo. Utiliza SQLite Online Backup API, verifica la integridad de la copia y crea un ZIP con base, configuración, certificados y manifiesto SHA-256 de cada archivo. El archivo definitivo aparece únicamente al completarse el respaldo.

Una copia en el mismo disco permite recuperar errores de operación; una copia en otra unidad permite recuperar la pérdida del disco. Configura `backupDirectory` hacia esa ubicación cuando esté disponible. La copia de un archivo `erp.db` abierto por sí solo no sustituye el respaldo consistente, porque puede haber transacciones en WAL.

## Restauración

1. Detén el servicio o proceso portable.
2. Ejecuta el comando como la cuenta del servicio o un administrador con acceso al directorio.

```bash
./erp restore --data-dir ./data --file ./data-backups/erp-FECHA.zip
```

3. El comando genera un respaldo de seguridad de la instalación actual, si existe; valida las entradas permitidas, checksums, configuración, versión, integridad SQLite y referencias del respaldo recibido.
4. Revoca las sesiones, suspende trabajos de impresión pendientes/enviándose y desactiva los periféricos hasta verificarlos de nuevo. Intercambia el directorio completo y conserva el anterior como `data.previous-FECHA-ID`.
5. Inicia el servicio, entra nuevamente y verifica últimas ventas, caja y existencias.

Una restauración inválida no reemplaza el directorio activo. El bloqueo del sistema operativo impide restaurar mientras el proceso servidor usa la instalación. El formato inicial admite respaldos de hasta 9 GiB y exige la misma versión de esquema; para una copia antigua utiliza su ejecutable compatible y luego realiza actualizaciones sucesivas.

El destino debe estar vacío o corresponder a una instalación ERP. La copia seleccionada se conserva temporalmente antes de generar el respaldo de seguridad, por lo que se puede restaurar incluso la más antigua de las siete copias retenidas.

En una restauración a otro equipo, configura una dirección de escucha utilizable allí y confía en la autoridad local restaurada. Conserva el directorio anterior hasta terminar la verificación.

## Actualizaciones y migraciones

Detén el servicio, ejecuta un respaldo, conserva el paquete anterior y reemplaza el ejecutable por la nueva versión. Los datos permanecen fuera del programa. El arranque comprueba la versión y, si hay migraciones, respalda y aplica el cambio transaccionalmente antes de servir solicitudes.

Si el cambio de esquema ya se confirmó, volver al binario anterior requiere restaurar el respaldo compatible. No se ejecutan migraciones descendentes automáticas. Los instaladores de servicio rechazan actualizar mientras el servicio esté activo.

La versión **0.4.0 usa esquema 4**. Al abrir esquemas 1, 2 o 3 genera un respaldo previo y aplica las migraciones consecutivas antes de aceptar usuarios. La conversión 1 → 2 mantiene exactamente las cantidades al pasar de milésimas a millonésimas. El paso 2 → 3 añade códigos, detalles de auditoría y periféricos sin modificar saldos, ventas, correlativos ni idempotencia. El paso 3 → 4 amplía caja con egresos y permisos, conservando ingresos y trabajos existentes. En instalaciones nuevas los periféricos empiezan desactivados; al actualizar se conserva su configuración. Verifica el resultado con `doctor`, últimas ventas e inventario.

Para volver a una versión anterior, detén 0.3 y utiliza su ejecutable anterior junto con el respaldo de esquema compatible. Para recuperar una copia antigua y continuar con 0.3, restaura primero con 0.1/0.2 y después actualiza. No restaures un archivo `erp.db` aislado sobre una instalación activa.

Consulta [Periféricos y recuperación de tickets](AUDIT_AND_DEVICES.md) para estados y permisos USB. Los fallos de transporte se registran como `printer_delivery_failed`, vinculados al trabajo. Un resultado incierto requiere comprobar el papel antes de solicitar una copia. El arranque nunca reenvía automáticamente un trabajo que estaba enviándose.

## Diagnóstico y recuperación de acceso

El certificado del servidor dura un año. Para renovarlo o cambiar nombres/IP, detén el servicio y ejecuta `./erp certificates --data-dir ./data --hosts erp.local,192.168.1.20`. Se genera un respaldo previo para una empresa configurada y se conserva la autoridad local, por lo que los clientes mantienen su confianza. La autoridad dura diez años; su reemplazo exige una actualización planificada de la confianza en clientes.

```bash
./erp doctor --data-dir ./data
```

Puede ejecutarse con el servidor activo. Verifica integridad y claves foráneas e informa versión, WAL, durabilidad, configuración inicial y necesidad de respaldo. La API de salud es `/api/v1/health`; el diagnóstico autenticado muestra peticiones, tiempo activo y memoria administrada por Go.

Los registros JSON se encuentran en `data/logs/`, con rotación a 10 MiB o cambio de fecha y un máximo de diez archivos. Correlaciona problemas con `X-Request-ID`. Los fallos de respaldo se registran como `backup_failed` y se vuelven a intentar en la siguiente comprobación.

Para recuperar una contraseña, detén el servidor y ejecuta:

```bash
./erp reset-password --data-dir ./data --username administrador
```

El comando lee la nueva contraseña desde stdin, no desde argumentos. Para elegir menos de doce caracteres o una contraseña vacía, añade `--allow-weak-password`; para una vacía envía una línea vacía. Sin esa aceptación se rechaza. Esta opción no omite el máximo de 128 caracteres. En una consola que muestre lo escrito, utiliza una entrada protegida o redirección desde un gestor de secretos. La operación revoca las sesiones del usuario. El acceso al directorio y a la consola del servidor permite realizar esta recuperación.

La instalación funciona sin servicio externo de correo, identidad, actualización o telemetría. La instalación de nuevas versiones se realiza mediante paquetes controlados por el administrador.
