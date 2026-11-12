# Auditoría, códigos de barras y periféricos de caja

## Auditoría que se puede consultar

En **Auditoría**, pulsa el nombre de una operación para abrir su detalle. Una edición muestra los campos antes y después; una venta conserva productos, cantidades, precios, ofertas, descuentos y total cobrado. Las referencias **Producto SKU** y **Venta #00042** abren el producto actual o su comprobante.

El buscador filtra la columna Operación por su nombre en español, sin distinguir acentos ni mayúsculas. Escribir `Ed` ofrece **Edición de producto** y **Edición del perfil**. El desplegable admite flechas, Enter, Escape y selección con ratón. Seleccionar una opción aplica ese tipo exacto; el botón de limpiar vuelve al historial completo. La paginación mantiene el filtro.

Los eventos nuevos guardan nombre del actor y referencia/descripción del momento. Las ediciones de productos y los comprobantes anteriores mantienen sus datos históricos. Los registros antiguos muestran únicamente la información que conservaron y avisan cuando sus referencias o nombres de usuario usan datos actuales. Un registro antiguo sin valores anteriores no permite reconstruirlos.

Los detalles de cuenta, empresa, permisos y promociones muestran campos comprensibles. Las contraseñas, hashes, tokens y datos privados no se incluyen en los eventos. La auditoría está reservada a administradores.

## Configurar el lector USB

Esta versión admite lectores que se presentan como **teclado USB HID** y terminan cada lectura con **Enter**. Configura ese modo con el manual del lector si es necesario. Se puede comprobar en un editor de texto: cada lectura debe escribir el código completo y terminar la línea. No necesita un controlador dentro del ERP.

En **Nueva venta**, cada lectura añade una unidad, o incrementa la línea existente, usando el código exacto. Los ceros iniciales se conservan. Los códigos externos y el generado por el ERP pueden apuntar al mismo producto. Se siguen aplicando los permisos, disponibilidad para venta, promociones y políticas de existencias habituales.

En **Productos**, la lectura localiza el producto y abre su detalle con las acciones disponibles. No registra ingresos, ajustes ni ventas automáticamente. Un código desconocido permite al administrador crear el producto con ese código o vincularlo a uno existente. Después de guardar, vuelve a escanear.

El campo de escaneo también permite escribir o pegar un código y pulsar Enter. **F2** coloca el foco en ese campo; desde otras secciones abre Nueva venta. Las lecturas rápidas fuera de formularios se dirigen a la venta por defecto. La captura global no interfiere con campos de texto ni modales. Para lectores lentos, códigos muy cortos o escritura manual, usa el campo de escaneo con foco.

No se incluyen lectores configurados como puerto serie ni lectura por cámara en esta entrega.

## Registrar, generar y exportar códigos

En el formulario de producto puedes añadir hasta treinta códigos, con un principal. Un mismo código pertenece a un solo producto. El SKU sigue siendo independiente.

- **CODE128:** códigos internos o externos alfanuméricos ASCII, hasta 64 caracteres.
- **EAN-8, EAN-13 y UPC-A:** longitud numérica y dígito de control verificados.
- **Generar código interno:** disponible en el detalle de un producto guardado. Asigna un CODE128 `RZ` seguido de doce dígitos. Repetir la acción consulta el mismo código; no crea otro ni reemplaza los del fabricante.

Desde el detalle del producto, descarga un código en **SVG** (vector) o **PNG**. Selecciona varios códigos para exportar un **ZIP de SVG**. Los archivos contienen el valor legible y márgenes blancos; el usuario elige el papel, programa, tamaño e impresora. No hay envío automático de estas imágenes a la térmica. Conserva los márgenes y comprueba una etiqueta impresa con tu lector antes de preparar un lote.

## Configurar Xprinter y cajón

Abre **Ajustes → Periféricos de caja** como administrador. La integración inicial es una impresora de tickets compatible con **ESC/POS**, conectada por USB al **mismo equipo que ejecuta el servidor ERP**. Los navegadores de otros equipos utilizan esa impresora a través del ERP. El cajón se conecta a la salida para cajón de la impresora.

La marca Xprinter tiene modelos de tickets y de etiquetas con diferentes comandos. El modelo exacto no está identificado en esta instalación: confirma compatibilidad ESC/POS, ancho de papel, corte disponible y compatibilidad eléctrica/conector del cajón con su documentación. El perfil admite papel de 58/80 mm, corte opcional, pin 2/5 y tiempos de pulso pares entre 2 y 510 ms. Los valores iniciales son 80 mm, sin corte, pin 2 y pulso de 100 ms/reposo de 500 ms; deben ajustarse al equipo antes de probarlo.

1. Instala o habilita la impresora en el sistema operativo del servidor.
2. Selecciónala y guarda el perfil. Impresión automática y cajón permanecen desactivados.
3. Pulsa **Imprimir ticket de prueba**. Comprueba texto en español, anchura y corte; después confirma físicamente el resultado en la pantalla.
4. Pulsa **Probar apertura del cajón** y confirma cuando haya abierto correctamente.
5. Activa **Imprimir automáticamente cada venta nueva** y, si corresponde, **Abrir el cajón al imprimir una venta nueva**. Guarda.

Cambiar el dispositivo o sus parámetros invalida las pruebas y desactiva ambos automatismos. Una prueba antigua no valida una configuración nueva ni una instalación restaurada.

En el controlador de la impresora, desactiva la apertura del cajón al principio/final de todos los trabajos. El ERP envía el pulso únicamente en una venta nueva o una apertura manual explícita. Comprueba que imprimir una copia deje el cajón cerrado.

### Windows

Instala el controlador del fabricante con una cola local del sistema y permite imprimir a la cuenta que ejecuta el ERP. El servicio portable instalado usa **LocalService**: una impresora visible solo en el perfil de tu usuario puede no estar disponible para esa cuenta. En modo de desarrollo, se utilizan las impresoras locales del usuario que inicia Go.

El adaptador usa la API RAW del spooler de Windows, sin ejecutar comandos de shell ni requerir herramientas de desarrollo. La cola debe aceptar datos ESC/POS; una impresora PDF o un dispositivo de etiquetas con otro lenguaje no sustituye ese perfil.

### Linux

Se utiliza un dispositivo de clase impresora USB, normalmente `/dev/usb/lp0`, con el controlador del kernel `usblp`. El adaptador no necesita CUPS ni bibliotecas C. Si el dispositivo no aparece, revisa el soporte del kernel y el modo USB de la impresora.

La cuenta del proceso necesita permiso de escritura. Para el servicio instalado es `raiz-erp`. Configura una regla udev específica para la impresora, usando su número de serie, para conservar permisos y una ruta estable. Ejemplo a adaptar tras consultar `udevadm info --attribute-walk --name=/dev/usb/lp0`:

```udev
SUBSYSTEM=="usbmisc", KERNEL=="lp[0-9]*", ATTRS{serial}=="SERIAL_DE_TU_IMPRESORA", GROUP="raiz-erp", MODE="0660", SYMLINK+="erp-printer-caja"
```

Recarga las reglas y vuelve a conectar la impresora. El ERP detecta tanto `/dev/usb/lp*` como alias `/dev/erp-printer-*` que apunten a esos dispositivos. Selecciona preferentemente el alias estable. No ejecutes el ERP como root para resolver permisos de impresión.

## Venta, reimpresión y recuperación

Una venta crea su trabajo en la misma transacción que inventario, caja y auditoría. El servidor envía el ticket después del commit. Con impresión automática activa, la interfaz devuelve el foco a caja y ofrece **Ver comprobante** en el estado del trabajo.

Estados:

| Estado | Significado |
| --- | --- |
| Pendiente | Venta registrada; trabajo en cola. |
| Enviando | El servidor está comunicándose con el dispositivo o spooler. |
| Enviado | El dispositivo o spooler aceptó el envío; no confirma que el papel haya salido. |
| Error | Falló el acceso o envío. Revisa conexión, permisos y registros. |
| Resultado incierto | Pudo salir contenido o el servidor se detuvo durante el envío. Revisa físicamente antes de pedir una copia. |
| Suspendido | Trabajo retenido por desactivación de impresión automática o restauración. |

Los trabajos se procesan en orden y se serializan. Un error de impresión no revierte una venta confirmada. Reintentar una venta cuya respuesta se perdió devuelve el mismo comprobante y trabajo, sin repetir cobros, consumos ni apertura. Los trabajos inciertos no se reenvían automáticamente.

Desde **Ventas → Ver comprobante → Reimprimir ticket** puedes solicitar una copia marcada como **COPIA**, sin comando de apertura. El modal se cierra cuando el servidor acepta la solicitud. Sin una impresora verificada, **Imprimir** utiliza el diálogo del navegador y cierra el comprobante al terminar o cancelar la vista previa.

La apertura manual está en Periféricos de caja, requiere administrador y motivo. Las solicitudes y sus resultados se auditan. Los reintentos usan idempotencia; no generes una nueva apertura para resolver un envío cuyo resultado aún no has comprobado.

Al reiniciar, un trabajo que estaba enviándose pasa a resultado incierto. Al restaurar un respaldo, los pendientes/enviándose pasan a suspendidos y los periféricos requieren nuevas verificaciones. El registro de un envío no es prueba de entrega física del ticket ni de apertura efectiva del cajón.

En Windows revisa también la cola del sistema tras una incidencia o restauración: un trabajo que Windows ya aceptó puede seguir pendiente allí. Suspender la cola del ERP no elimina trabajos que ya están en el spooler.

## Egresos y apertura del cajón

Desde **Caja → Registrar egreso** se registran gastos en efectivo con monto y motivo. El permiso se habilita por operador en Administración. Si la apertura automática está activa y verificada, se crea una única orden de cajón con el egreso; no se imprime un ticket de gasto. Consulta [Caja y acceso](CASH_AND_ACCESS.md).

Un egreso confirmado no se revierte por fallos del equipo. Su detalle muestra el resultado del envío. Comprueba físicamente el cajón si el resultado es incierto y utiliza, si hace falta, la apertura administrativa con motivo. Reintentar el registro no crea otra orden. La restauración suspende también las aperturas por egreso; revísalas antes de volver a activar automatismos.
