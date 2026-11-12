# Productos, insumos, ofertas y ajustes

## Catálogo y existencias

**Productos** reúne catálogo y existencias en una única pantalla: nombre, SKU, unidad, precio y saldo disponible. Desde allí el administrador crea y edita productos, ingresa existencias y registra ajustes con motivo. Los operadores conservan sus consultas y lectura de códigos. Los enlaces antiguos `#/inventory` se redirigen a `#/products` y el acceso rápido **Ingresar existencias** abre esta misma sección.

## Cantidades y edición

Los botones y las flechas arriba/abajo de los controles numéricos cambian el valor en uno: `1,25 → 2,25`. Puedes escribir coma o punto decimal. Ventas y factores admiten tres decimales; los conteos e ingresos de inventario, seis. La precisión es independiente del paso del botón.

Desde **Productos → Editar** el administrador modifica nombre, SKU, precio, disponibilidad para venta, composición y control de stock. El identificador se conserva y los comprobantes existentes mantienen sus datos. Un cambio de unidad con historial o uso como insumo se rechaza. Dos ediciones simultáneas requieren recargar cuando la versión ya cambió.

Para un centro de copiado:

1. Crea **Hoja A4** y, si corresponde, tóner, desmarcando **Disponible para venta**. Ingresa sus existencias.
2. Crea **Copia color**, disponible para venta, con **Consumir insumos**. Añade una hoja y la cantidad de tóner necesaria.
3. Crea **Copia B/N** apuntando a la misma hoja y a su consumo de tóner.
4. Vende ambas copias. El servidor suma los consumos, incluidos los regalos, antes de modificar cada insumo una sola vez.

Las composiciones admiten varios insumos de stock propio y un nivel. No admiten productos compuestos como componentes. La disponibilidad mostrada se deriva de los insumos que bloquean faltantes; el carrito verifica además sus consumos compartidos. Los compuestos no reciben existencias propias.

## Políticas de inventario

| Política | Resultado al vender más del saldo |
| --- | --- |
| Bloquear faltantes | Rechaza la venta completa. Predeterminada. |
| Permitir negativos | Conserva todo el consumo y el saldo negativo. |
| Mantener mínimo cero | Conserva la salida completa y registra una compensación por el faltante. Saldo final cero. |
| Sin seguimiento | Vende sin comprobar ni cambiar existencias. |

Cada insumo aplica su propia política. Activar mínimo cero con saldo negativo genera una corrección auditada. Volver a controlar un producto sin seguimiento exige indicar las existencias actuales y un motivo. Cambiar de stock propio a composición con saldo exige ajustarlo a cero con motivo.

**Productos → Ajustar** establece el saldo contado y exige motivo. Genera un movimiento por la diferencia, sin borrar movimientos anteriores. **Ingresar** añade mercadería; úsalo para reposición. Un cero controlado se distingue de **Sin seguimiento**.

## Ofertas

El administrador crea, edita y pausa reglas en **Descuentos y ofertas**. Cada regla tiene nombre, prioridad, grupos de productos, vigencia y acumulación:

- **Descuento:** porcentaje o monto sobre productos seleccionados o sobre la venta. Los montos por producto pueden ser por unidad o por línea.
- **Lleva X, paga Y:** introduce en caja todas las unidades entregadas. El descuento bonifica las más baratas del grupo. El grupo debe compartir unidad de medida.
- **Compra N y recibe J adicionales gratis:** introduce primero la compra. La caja muestra los regalos elegibles y el vendedor confirma producto y cantidad con **Agregar regalo**. Un regalo consume inventario/insumos y no genera nuevos regalos.

Las fechas se introducen en la zona horaria de la empresa y se guardan en UTC. El inicio está incluido; el fin, excluido. **Sin fecha final** mantiene una regla permanente, que puede tener inicio. La vigencia se evalúa tanto al cotizar como al confirmar.

Las reglas se ordenan por prioridad descendente y luego por identificador estable. Las reglas que se superponen solo se acumulan cuando ambas admiten acumulación; una no acumulable bloquea otras reglas automáticas sobre sus líneas. Las reglas automáticas nunca descuentan más del importe disponible. Los descuentos manuales autorizados pueden añadirse igualmente.

## Descuentos y confirmación

Un administrador puede descontar siempre. Un operador necesita **Administración → Permitir descuentos**, permiso cuyo cambio se audita. En caja hay descuento por producto y **Descuento sobre el total**. Elige porcentaje o monto; el monto de producto usa **A toda la línea** por defecto y permite **Por cada unidad**. El mismo diálogo pide el motivo inmediatamente después del porcentaje o monto y antes de aplicar. Ese motivo es común a todos los descuentos manuales de la venta; se precarga al añadir o editar otro y queda resumido junto al desglose del carrito. Cancelar conserva los valores previos.

Con un descuento total aplicado, **Editar descuento total** y **Quitar** aparecen en una fila, con la acción de quitar a la derecha. Al quitarlo, la cotización se actualiza conservando promociones, descuentos por producto y el motivo común. Las acciones quedan deshabilitadas mientras se recupera una venta pendiente de confirmación.

El orden es: ofertas automáticas, descuentos manuales de línea y descuento manual total. El servidor calcula y distribuye cada importe exacto entre líneas. Rechaza un descuento manual que deje un importe negativo. Guarda el valor solicitado, importe aplicado, alcance, motivo y responsable.

La cotización muestra subtotal, ofertas/regalos, descuentos, total final y faltantes. **Confirmar venta** necesita una cotización vigente; un cambio de precio, regla, composición o permiso exige revisar. Caja e informes usan el cobro final. El comprobante incluye el desglose; motivos y responsables se conservan en el detalle interno y la auditoría.

Si se pierde la respuesta, usa **Reintentar confirmación**. El carrito permanece congelado con descuentos y regalos para recuperar el mismo documento sin repetir consumos ni cobros, incluso si una oferta expiró después. El navegador guarda este estado por usuario en la misma pestaña; cerrar la pestaña o borrar sus datos elimina esa copia local.

## Cuenta y empresa

En **Ajustes** cada usuario cambia su nombre visible o contraseña. El cambio de contraseña requiere la actual, cierra todas sus sesiones y devuelve al inicio de sesión. **Cambiar de usuario** cierra la sesión actual y conserva el carrito de cada usuario en la misma pestaña.

El administrador también cambia nombre de empresa y zona horaria. La moneda permanece fija. Los comprobantes guardados mantienen la configuración de empresa con que se emitieron.

Se mantiene una empresa, moneda y bodega, composiciones de un nivel y beneficios dentro de una venta. Packs de precio cerrado, tramos, cupones y acumulación entre ventas quedan fuera.

## Egresos y contraseñas opcionales

Consulta [Caja y acceso](CASH_AND_ACCESS.md) para registrar gastos en efectivo, habilitar el permiso por operador y elegir una contraseña corta o vacía con advertencia.
