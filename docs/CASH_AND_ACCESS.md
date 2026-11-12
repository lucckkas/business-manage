# Egresos de caja y contraseñas opcionales

## Registrar gastos en efectivo

En **Caja → Registrar egreso**, escribe el monto que sale y el motivo, por ejemplo «Compra de papel higiénico». Los controles cambian de uno en uno y admiten decimales según la moneda de la empresa. Cada registro recibe una referencia como **Egreso #00001**, responsable y fecha.

Los administradores pueden registrar egresos. Para habilitar a un operador, marca **Administración → Egresos de caja** en su fila. La opción también está disponible al crear el usuario. Los cambios del permiso se auditan y se verifican de nuevo en el servidor al registrar un gasto.

Caja muestra **Ingresos**, **Egresos** y **Saldo registrado**. Las ventas siguen formando el total de facturación; los egresos afectan únicamente el efectivo registrado. No hay apertura ni saldo inicial en esta entrega. Si el gasto supera el saldo, aparece una advertencia y puedes guardarlo igualmente: revisa el dinero disponible físicamente.

Busca por referencia o motivo y filtra ingresos o egresos. Pulsa una venta para ver su comprobante o un egreso para consultar monto, motivo, responsable y saldo anterior/final. Auditoría conserva esos mismos datos históricos y enlaza al egreso. Los movimientos confirmados no se editan ni se eliminan.

## Si la respuesta se pierde

El navegador guarda el egreso pendiente por usuario antes de enviarlo. Si no puede confirmar la respuesta, conserva monto, motivo y clave; pulsa **Reintentar registro**. También puedes recargar o cambiar de cuenta y volver con el mismo usuario. No borres los datos del navegador mientras haya un registro pendiente.

El reintento recupera el mismo movimiento y la misma orden del cajón, incluso si después cambiaron los permisos. Una respuesta recibida correctamente permite empezar un egreso nuevo con otra clave.

Si el cajón está verificado y la apertura automática activada, el ERP envía una orden después de guardar. Un problema de conexión no elimina el gasto. El estado «Enviado» indica entrega al dispositivo o cola del sistema; comprueba físicamente la apertura. Ante resultado incierto no se reenvía automáticamente. La apertura administrativa separada sigue exigiendo motivo.

## Elegir contraseñas cortas o vacías

El alta inicial, la creación de usuarios y **Ajustes → Cambiar contraseña** recomiendan doce caracteres y admiten hasta 128. Si eliges menos de doce, marca **Usar esta contraseña aunque sea poco segura** para confirmar tu decisión.

Si dejas el campo vacío se muestra: **Cualquier persona con acceso al sistema que conozca este usuario podrá entrar sin contraseña**. Debes aceptar la advertencia antes de guardar. En el inicio de sesión introduce el usuario y deja la contraseña vacía.

Para cambiar una cuenta que ya no tiene contraseña, deja vacía **Contraseña actual**. Para quitar la contraseña, deja vacías la nueva y su confirmación y acepta la advertencia. Todos los cambios cierran las sesiones de esa cuenta y requieren entrar nuevamente.

La decisión se toma al establecer cada contraseña; no hay un ajuste global que cambie otras cuentas. El almacenamiento sigue utilizando Argon2id con sal aleatoria, también para cadenas vacías. Las contraseñas no se muestran en auditoría.

Para recuperación por consola, consulta [Operación](OPERATIONS.md): `reset-password --allow-weak-password` permite elegir explícitamente una contraseña corta o vacía.

## Motivos de descuentos

En el diálogo de descuento, completa porcentaje o monto y **Motivo del descuento** antes de aplicar. El motivo es común a todos los descuentos manuales de la venta; se precarga al editar o añadir otro. Cancelar no cambia el carrito. Su resumen aparece junto al desglose y se conserva en el detalle interno del comprobante y auditoría.
