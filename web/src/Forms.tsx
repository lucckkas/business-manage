import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Printer, CheckCircle2 } from 'lucide-react';
import {
  api,
  money,
  date,
  quantity,
  type Product,
  type Sale,
  type Session,
  type Schema,
} from './api/client';
import { ErrorMessage, Field, Modal, useResource } from './ui';
import NumericInput from './NumericInput';
import ProductEditor from './ProductEditor';
import PasswordField from './PasswordField';
export type FormKind =
  | { type: 'product'; product?: Product; barcode?: string }
  | { type: 'stock'; product: Product }
  | { type: 'adjust'; product: Product }
  | { type: 'user' };
type Props = {
  kind: FormKind;
  session: Session;
  onClose: () => void;
  onSuccess: (message: string) => void;
};
export function EditForm(props: Props) {
  return props.kind.type === 'product' ? (
    <ProductEditor
      initial={props.kind.product}
      barcode={props.kind.barcode}
      session={props.session}
      onClose={props.onClose}
      onSuccess={props.onSuccess}
    />
  ) : (
    <SimpleForm {...props} />
  );
}
function SimpleForm({ kind, onClose, onSuccess }: Props) {
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [role, setRole] = useState('operator'),
    [canManual, setCanManual] = useState(false),
    [canExpense, setCanExpense] = useState(false);
  const title =
    kind.type === 'user'
      ? 'Nuevo usuario'
      : kind.type === 'adjust'
        ? 'Ajustar existencias'
        : 'Ingresar existencias';
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError('');
    const data = new FormData(event.currentTarget);
    try {
      if (kind.type === 'stock') {
        await api('/inventory/receipts', 'POST', {
          productId: kind.product.id,
          quantity: String(data.get('quantity')).replace(',', '.'),
          reason: data.get('reason'),
        });
        onSuccess('Existencias actualizadas.');
      } else if (kind.type === 'adjust') {
        await api('/inventory/adjustments', 'POST', {
          productId: kind.product.id,
          countedQuantity: String(data.get('quantity')).replace(',', '.'),
          reason: data.get('reason'),
        });
        onSuccess('Conteo registrado con su movimiento de ajuste.');
      } else if (kind.type === 'user') {
        const body: Schema['CreateUser'] = {
          name: String(data.get('name')),
          username: String(data.get('username')),
          password: String(data.get('password')),
          allowWeakPassword: data.get('allowWeakPassword') === 'on',
          canRecordCashExpenses: data.get('cashExpenses') === 'on',
          role: role as 'admin' | 'operator',
          canApplyManualDiscounts: data.get('manualDiscounts') === 'on',
        };
        await api('/users', 'POST', body);
        onSuccess('Usuario creado.');
      }
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title={title} onClose={onClose} busy={busy}>
      <form onSubmit={submit}>
        <ErrorMessage message={error} />
        {(kind.type === 'stock' || kind.type === 'adjust') && (
          <>
            <div className="selected-product">
              <span className="product-avatar">{kind.product.name.slice(0, 2).toUpperCase()}</span>
              <div>
                <strong>{kind.product.name}</strong>
                <small>
                  {kind.product.sku} · Actual: {quantity(kind.product.stock)} {kind.product.unit}
                </small>
              </div>
            </div>
            <Field
              label={
                kind.type === 'stock'
                  ? `Cantidad a ingresar (${kind.product.unit})`
                  : `Existencias contadas (${kind.product.unit})`
              }
              hint="Botones de uno en uno; admite hasta seis decimales escritos."
            >
              <NumericInput
                name="quantity"
                label={kind.type === 'stock' ? 'Cantidad a ingresar' : 'Existencias contadas'}
                precision={6}
                minimum={
                  kind.type === 'adjust' && kind.product.stockPolicy === 'negative'
                    ? '-1000000000'
                    : kind.type === 'stock'
                      ? '0.000001'
                      : '0'
                }
                defaultValue={kind.type === 'adjust' ? (kind.product.stock ?? '0') : ''}
                required
                autoFocus
              />
            </Field>
            <Field label={kind.type === 'stock' ? 'Motivo del ingreso' : 'Motivo del ajuste'}>
              <textarea name="reason" required minLength={3} maxLength={240} rows={3} />
            </Field>
          </>
        )}
        {kind.type === 'user' && (
          <>
            <Field label="Nombre completo">
              <input name="name" required minLength={2} maxLength={120} autoFocus />
            </Field>
            <Field label="Usuario">
              <input
                name="username"
                required
                minLength={3}
                maxLength={64}
                autoCapitalize="none"
                autoComplete="off"
              />
            </Field>
            <PasswordField name="password" label="Contraseña inicial" />
            <Field label="Rol">
              <select name="role" value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="operator">Operador · Consultas y ventas</option>
                <option value="admin">Administrador · Gestión completa</option>
              </select>
            </Field>
            <label className="checkbox-field">
              <input
                type="checkbox"
                name="manualDiscounts"
                disabled={role === 'admin'}
                checked={role === 'admin' || canManual}
                onChange={(event) => setCanManual(event.target.checked)}
              />
              Permitir descuentos manuales
              <small>
                Los administradores siempre tienen este permiso. Cada descuento queda registrado.
              </small>
            </label>
            <label className="checkbox-field">
              <input
                type="checkbox"
                name="cashExpenses"
                disabled={role === 'admin'}
                checked={role === 'admin' || canExpense}
                onChange={(event) => setCanExpense(event.target.checked)}
              />
              Permitir egresos de caja
              <small>
                Los administradores siempre tienen este permiso. Cada egreso exige motivo y queda
                registrado.
              </small>
            </label>
          </>
        )}
        <div className="modal-actions">
          <button type="button" className="button secondary" disabled={busy} onClick={onClose}>
            Cancelar
          </button>
          <button className="button primary" disabled={busy}>
            {busy ? 'Guardando…' : 'Guardar'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
export function Receipt({
  sale,
  session,
  onClose,
  onQueued,
}: {
  sale: Sale;
  session: Session;
  onClose: () => void;
  onQueued?: (job: Schema['PrintJob']) => void;
}) {
  const company = sale.company?.name ? sale.company : session.company;
  const manuals = sale.discounts.filter((d) => d.kind.startsWith('manual'));
  const settings = useResource<Schema['POSSettings']>('/pos/settings');
  const [printError, setPrintError] = useState(''),
    [printing, setPrinting] = useState(false);
  const printRequested = useRef(false),
    printKey = useRef(crypto.randomUUID());
  useEffect(() => {
    const done = () => {
      if (printRequested.current) onClose();
    };
    window.addEventListener('afterprint', done);
    return () => window.removeEventListener('afterprint', done);
  }, [onClose]);
  async function print() {
    setPrintError('');
    if (settings.data?.printVerified) {
      setPrinting(true);
      try {
        const job = await api<Schema['PrintJob']>(
          `/sales/${sale.id}/print`,
          'POST',
          {},
          { 'Idempotency-Key': printKey.current }
        );
        onQueued?.(job);
        onClose();
      } catch (e) {
        setPrintError((e as Error).message);
      } finally {
        setPrinting(false);
      }
    } else {
      printRequested.current = true;
      window.print();
    }
  }
  return (
    <Modal title={`Comprobante #${String(sale.number).padStart(5, '0')}`} onClose={onClose} receipt>
      <div className="receipt-success no-print">
        <CheckCircle2 size={20} />
        Venta registrada correctamente
      </div>
      <div className="receipt-company">
        <h3>{company.name}</h3>
        <span>Comprobante interno · Venta al contado</span>
        <small>{date(sale.createdAt, company.timezone)}</small>
      </div>
      <table className="receipt-table">
        <thead>
          <tr>
            <th>Producto</th>
            <th>Cantidad</th>
            <th className="numeric">Importe</th>
          </tr>
        </thead>
        <tbody>
          {sale.lines.map((line) => (
            <tr key={line.id}>
              <td>
                <strong>{line.name}</strong>
                {line.kind === 'gift' ? (
                  <small className="positive">Regalo · Gratis</small>
                ) : (
                  <small>
                    {money(line.priceMinor, company)} / {line.unit}
                  </small>
                )}
                {line.promotionDiscountMinor !== '0' && line.kind !== 'gift' && (
                  <small>Oferta: − {money(line.promotionDiscountMinor, company)}</small>
                )}
                {(line.manualDiscountMinor !== '0' || line.orderDiscountMinor !== '0') && (
                  <small>
                    Descuento: −{' '}
                    {money(
                      (
                        BigInt(line.manualDiscountMinor) + BigInt(line.orderDiscountMinor)
                      ).toString(),
                      company
                    )}
                  </small>
                )}
              </td>
              <td>{quantity(line.quantity)}</td>
              <td className="numeric">{money(line.totalMinor, company)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="receipt-breakdown">
        <div>
          <span>Subtotal</span>
          <span>{money(sale.subtotalMinor, company)}</span>
        </div>
        {sale.promotionDiscountMinor !== '0' && (
          <div>
            <span>Ofertas y regalos</span>
            <span>− {money(sale.promotionDiscountMinor, company)}</span>
          </div>
        )}
        {sale.manualDiscountMinor !== '0' && (
          <div>
            <span>Descuentos manuales</span>
            <span>− {money(sale.manualDiscountMinor, company)}</span>
          </div>
        )}
      </div>
      <div className="receipt-total">
        <span>Total pagado</span>
        <strong>{money(sale.totalMinor, company)}</strong>
      </div>
      <p className="receipt-legal">
        Este comprobante corresponde a un registro de gestión interna y no tiene validez tributaria.
      </p>
      {manuals.length > 0 && (
        <details className="no-print internal-discounts">
          <summary>Detalle interno de descuentos</summary>
          {manuals.map((d, i) => (
            <p key={i}>
              <strong>
                {d.label} · {money(d.amountMinor, company)}
              </strong>
              <br />
              {d.reason}
              <br />
              <small>Responsable: {d.actorName || d.actorId}</small>
              {d.specification && (
                <small>
                  {' · Solicitado: '}
                  {d.specification.type === 'percentage'
                    ? `${d.specification.value}%`
                    : `${money(d.specification.value, company)} ${d.specification.basis === 'unit' ? 'por unidad' : 'por línea'}`}
                </small>
              )}
            </p>
          ))}
        </details>
      )}
      <div className="no-print">
        <ErrorMessage message={printError || settings.error} />
      </div>
      <div className="modal-actions no-print">
        <button className="button secondary" onClick={onClose}>
          Cerrar
        </button>
        <button
          className="button primary"
          disabled={printing || settings.loading || !!settings.error}
          onClick={print}
        >
          <Printer size={16} />
          {printing
            ? 'Encolando…'
            : settings.data?.printVerified
              ? 'Reimprimir ticket'
              : 'Imprimir'}
        </button>
      </div>
    </Modal>
  );
}
