import { useId, useState } from 'react';
import { ArrowUpRight, Search, X } from 'lucide-react';
import { api, date, money, quantity, type Sale, type Schema, type Session } from './api/client';
import { ErrorMessage, Loading, Modal, PageError, Pagination, useResource, useSearch } from './ui';
import { Heading } from './Pages';
import { jobKinds, jobLabels } from './Printing';
type Entry = Schema['AuditEntry'];
type RecordValue = Record<string, unknown>;
const record = (v: unknown): RecordValue =>
  v && typeof v === 'object' && !Array.isArray(v) ? (v as RecordValue) : {};
const labels: Record<string, string> = {
  name: 'Nombre',
  sku: 'SKU',
  unit: 'Unidad',
  priceMinor: 'Precio de venta',
  sellable: 'Disponible para venta',
  stockMode: 'Inventario',
  stockPolicy: 'Política de existencias',
  components: 'Consumo por unidad',
  barcodes: 'Códigos de barras',
  username: 'Usuario',
  role: 'Rol',
  active: 'Acceso activo',
  canApplyManualDiscounts: 'Permitir descuentos manuales',
  canRecordCashExpenses: 'Permitir egresos de caja',
  timezone: 'Zona horaria',
  currency: 'Moneda',
  kind: 'Tipo',
  scope: 'Aplicación',
  productIds: 'Productos participantes',
  rewardProductIds: 'Productos de regalo',
  discount: 'Descuento',
  buyQuantity: 'Cantidad de compra',
  payQuantity: 'Cantidad pagada',
  giftQuantity: 'Cantidad gratis',
  priority: 'Prioridad',
  enabled: 'Habilitado',
  stackable: 'Acumulable',
  startsAt: 'Inicio',
  endsAt: 'Fin',
  deviceName: 'Impresora',
  width: 'Ancho del papel (mm)',
  cutter: 'Corte automático',
  drawerPin: 'Conector del cajón',
  pulseOnMs: 'Pulso (ms)',
  pulseOffMs: 'Reposo (ms)',
  drawerEnabled: 'Apertura automática',
  printVerified: 'Ticket verificado',
  drawerVerified: 'Cajón verificado',
  reason: 'Motivo',
};
const values: Record<string, string> = {
  own: 'Stock propio',
  components: 'Consumo de insumos',
  block: 'Bloquear faltantes',
  negative: 'Permitir negativos',
  zero: 'Mantener mínimo cero',
  untracked: 'Sin seguimiento',
  admin: 'Administrador',
  operator: 'Operador',
  discount: 'Descuento',
  buy_pay: 'Lleva X, paga Y',
  buy_get: 'Compra N y recibe J gratis',
  products: 'Productos seleccionados',
  cart: 'Total de la venta',
};
const fold = (v: string) =>
  v
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase();

function Detail({
  id,
  session,
  onClose,
  onReceipt,
  onProduct,
  onCash,
}: {
  id: string;
  session: Session;
  onClose: () => void;
  onReceipt: (sale: Sale) => void;
  onProduct: (id: string) => void;
  onCash: (id: string) => void;
}) {
  const resource = useResource<Schema['AuditDetail']>(`/audit/${id}`);
  const response = resource.data,
    e = response?.entry,
    d = record(e?.details),
    sale = response?.sale;
  const names = record(d.productNames);
  function value(key: string, v: unknown): string {
    if (v === null || v === undefined || v === '') return '—';
    if (typeof v === 'boolean') return v ? 'Sí' : 'No';
    if (['priceMinor', 'amountMinor', 'balanceBefore', 'balanceAfter'].includes(key))
      return money(String(v), sale?.company ?? session.company);
    if (key === 'drawerPin') return Number(v) === 0 ? 'Pin 2' : 'Pin 5';
    if (key === 'startsAt' || key === 'endsAt') return date(String(v), session.company.timezone);
    if (key === 'components' && Array.isArray(v))
      return (
        v
          .map((c) => {
            const component = record(c);
            return `${names[String(component.productId)] ?? 'Insumo registrado'} × ${quantity(String(component.quantity))}`;
          })
          .join('; ') || 'Sin insumos'
      );
    if (key === 'barcodes' && Array.isArray(v))
      return (
        v
          .map((b) => {
            const code = record(b);
            return `${code.value} (${code.format})${code.primary ? ' · Principal' : ''}`;
          })
          .join('; ') || 'Sin códigos'
      );
    if ((key === 'productIds' || key === 'rewardProductIds') && Array.isArray(v))
      return v.map((id) => names[String(id)] ?? 'Producto registrado').join('; ') || '—';
    if (key === 'discount') {
      const spec = record(v);
      return spec.type === 'percentage'
        ? `${spec.value}%`
        : `${money(String(spec.value), session.company)} ${spec.basis === 'unit' ? 'por unidad' : 'por línea'}`;
    }
    if (typeof v === 'string' || typeof v === 'number') return values[String(v)] ?? String(v);
    return 'Datos registrados';
  }
  const before = record(d.before),
    after = record(d.after);
  const changes = Object.keys(labels)
    .filter(
      (k) => (k in before || k in after) && JSON.stringify(before[k]) !== JSON.stringify(after[k])
    )
    .map((k) => ({ label: labels[k], before: value(k, before[k]), after: value(k, after[k]) }));
  if (
    d.stockBefore !== undefined &&
    d.stockAfter !== undefined &&
    (d.stockBefore !== d.stockAfter || e?.action.startsWith('inventory.'))
  )
    changes.push({
      label: 'Existencias',
      before: quantity(String(d.stockBefore)),
      after: quantity(String(d.stockAfter)),
    });
  if (
    d.before !== undefined &&
    d.after !== undefined &&
    !Object.keys(before).length &&
    !Object.keys(after).length &&
    (typeof d.before !== 'object' || typeof d.after !== 'object')
  )
    changes.push({
      label:
        e?.action === 'user.discount_permission'
          ? 'Descuentos manuales'
          : e?.action === 'user.expense_permission'
            ? 'Egresos de caja'
            : e?.action === 'user.active'
              ? 'Acceso activo'
              : 'Existencias',
      before: value('scalar', d.before),
      after: value('scalar', d.after),
    });
  const facts = [] as { label: string; value: string }[];
  if (!Object.keys(after).length && e?.action.endsWith('.create')) {
    const source = Object.keys(record(d.product)).length
      ? record(d.product)
      : Object.keys(record(d.promotion)).length
        ? record(d.promotion)
        : d;
    for (const k of Object.keys(labels))
      if (k in source) facts.push({ label: labels[k], value: value(k, source[k]) });
  }
  const receipt = record(d.receipt),
    reason = d.reason ?? receipt.reason ?? d.adjustmentReason;
  if (receipt.quantity)
    facts.push({ label: 'Cantidad ingresada', value: quantity(String(receipt.quantity)) });
  for (const k of ['amountMinor', 'balanceBefore', 'balanceAfter'])
    if (d[k] !== undefined && e?.subjectType === 'cash')
      facts.push({
        label:
          k === 'amountMinor'
            ? 'Importe'
            : k === 'balanceBefore'
              ? 'Saldo anterior'
              : 'Saldo después del egreso',
        value: value(k, d[k]),
      });
  if (d.barcode) facts.push({ label: 'Código generado', value: String(d.barcode) });
  if (reason) facts.push({ label: 'Motivo', value: String(reason) });
  if (d.state)
    facts.push({
      label: 'Estado',
      value: jobLabels[String(d.state) as keyof typeof jobLabels] ?? String(d.state),
    });
  if (d.kind && e?.action.startsWith('pos.'))
    facts.push({
      label: 'Trabajo',
      value: jobKinds[String(d.kind) as keyof typeof jobKinds] ?? String(d.kind),
    });
  if (d.printer) facts.push({ label: 'Impresora', value: String(d.printer) });
  if (d.opensDrawer !== undefined)
    facts.push({ label: 'Incluye apertura del cajón', value: value('opensDrawer', d.opensDrawer) });
  if (d.error) facts.push({ label: 'Resultado', value: String(d.error) });
  return (
    <Modal
      title={e?.label ?? 'Detalle de la operación'}
      onClose={onClose}
      className="detail-dialog"
    >
      {resource.loading ? (
        <Loading />
      ) : resource.error ? (
        <PageError message={resource.error} retry={resource.retry} />
      ) : (
        e && (
          <>
            <p className="audit-reference">{e.reference}</p>
            <dl className="detail-facts">
              <dt>Persona</dt>
              <dd>{e.actorName}</dd>
              <dt>Fecha</dt>
              <dd>{date(e.createdAt, session.company.timezone)}</dd>
            </dl>
            {e.historical && (
              <p className="history-note">
                Registro anterior al detalle ampliado. Se muestran los datos históricos disponibles;
                algunas referencias y nombres de usuario corresponden a los datos actuales.
              </p>
            )}
            {sale && (
              <>
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        <th scope="col">Producto</th>
                        <th scope="col">Cantidad</th>
                        <th scope="col">Precio unitario</th>
                        <th scope="col">Importe final</th>
                      </tr>
                    </thead>
                    <tbody>
                      {sale.lines.map((l) => (
                        <tr key={l.id}>
                          <td>
                            {l.name}
                            {l.kind === 'gift' && (
                              <small className="block-muted">Regalo · Gratis</small>
                            )}
                          </td>
                          <td>
                            {quantity(l.quantity)} {l.unit}
                          </td>
                          <td>{money(l.priceMinor, sale.company)}</td>
                          <td>{money(l.totalMinor, sale.company)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <dl className="detail-facts">
                  <dt>Subtotal</dt>
                  <dd>{money(sale.subtotalMinor, sale.company)}</dd>
                  <dt>Ofertas y regalos</dt>
                  <dd>{money(sale.promotionDiscountMinor, sale.company)}</dd>
                  <dt>Descuentos manuales</dt>
                  <dd>{money(sale.manualDiscountMinor, sale.company)}</dd>
                  <dt>Total cobrado</dt>
                  <dd>
                    <strong>{money(sale.totalMinor, sale.company)}</strong>
                  </dd>
                </dl>
                {sale.discounts
                  .filter((v) => v.kind.startsWith('manual'))
                  .map((v, i) => (
                    <p className="form-note" key={i}>
                      {v.label}: {money(v.amountMinor, sale.company)} · {v.reason} ·{' '}
                      {v.actorName || e.actorName}
                    </p>
                  ))}
                <button
                  className="button secondary"
                  onClick={() => {
                    onClose();
                    onReceipt(sale);
                  }}
                >
                  Ver comprobante de venta
                </button>
              </>
            )}
            {changes.length > 0 && (
              <div className="table-scroll audit-changes">
                <table>
                  <thead>
                    <tr>
                      <th scope="col">Campo</th>
                      <th scope="col">Antes</th>
                      <th scope="col">Después</th>
                    </tr>
                  </thead>
                  <tbody>
                    {changes.map((c, i) => (
                      <tr key={i}>
                        <th scope="row">{c.label}</th>
                        <td>{c.before}</td>
                        <td>{c.after}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {facts.length > 0 && (
              <dl className="detail-facts">
                {facts.map((f, i) => (
                  <div className="fact-pair" key={i}>
                    <dt>{f.label}</dt>
                    <dd>{f.value}</dd>
                  </div>
                ))}
              </dl>
            )}
            {!sale && !changes.length && !facts.length && (
              <p className="form-note">
                {e.historical
                  ? 'Este registro no conservó los campos modificados.'
                  : e.action.includes('password')
                    ? 'Se actualizó la contraseña y se revocaron las sesiones del usuario.'
                    : e.summary || 'Operación registrada.'}
              </p>
            )}
            {e.subjectType === 'cash' && (
              <button
                className="button secondary"
                onClick={() => {
                  onClose();
                  onCash(e.subjectId);
                }}
              >
                Ver egreso
              </button>
            )}
            {e.subjectType === 'product' && (
              <button
                className="button secondary"
                onClick={() => {
                  onClose();
                  onProduct(e.subjectId);
                }}
              >
                Ver producto actual
              </button>
            )}
          </>
        )
      )}
    </Modal>
  );
}
export default function Audit({
  session,
  revision,
  onReceipt,
  onProduct,
  onCash,
}: {
  session: Session;
  revision: number;
  onReceipt: (s: Sale) => void;
  onProduct: (id: string) => void;
  onCash: (id: string) => void;
}) {
  const search = useSearch(),
    [action, setAction] = useState(''),
    [open, setOpen] = useState(false),
    [active, setActive] = useState(0),
    [selected, setSelected] = useState(''),
    [error, setError] = useState('');
  const result = useResource<Schema['AuditPage']>(
      `/audit?${search.query}&action=${encodeURIComponent(action)}`,
      revision
    ),
    operations = useResource<Schema['AuditOperations']>('/audit/operations');
  const listId = useId(),
    suggestions = (operations.data ?? []).filter(
      (o) =>
        fold(o.label).includes(fold(search.search)) || fold(o.code).includes(fold(search.search))
    );
  function choose(op: Schema['AuditOperation']) {
    setAction(op.code);
    search.setSearch(op.label);
    search.setPage(1);
    setOpen(false);
  }
  async function related(e: Entry) {
    setError('');
    if (e.subjectType === 'cash') onCash(e.subjectId);
    else if (e.subjectType === 'product') onProduct(e.subjectId);
    else if (e.subjectType === 'sale') {
      try {
        onReceipt(await api<Sale>(`/sales/${e.subjectId}`));
      } catch (err) {
        setError((err as Error).message);
      }
    }
  }
  return (
    <>
      <Heading
        eyebrow="UNA HISTORIA TRANSPARENTE"
        title="Auditoría"
        description="Qué ocurrió, qué cambió y quién lo realizó."
      />
      <ErrorMessage message={error} />
      <section className="panel">
        <div className="table-toolbar">
          <div className="operation-search">
            <label className="search-box">
              <Search size={17} />
              <input
                role="combobox"
                aria-label="Buscar operación"
                aria-autocomplete="list"
                aria-controls={listId}
                aria-expanded={open && !!suggestions.length}
                aria-activedescendant={
                  open && suggestions[active] ? `${listId}-${active}` : undefined
                }
                autoComplete="off"
                placeholder="Buscar operación"
                value={search.search}
                onFocus={() => setOpen(true)}
                onBlur={() => setOpen(false)}
                onChange={(e) => {
                  search.setSearch(e.target.value);
                  setAction('');
                  setActive(0);
                  setOpen(true);
                }}
                onKeyDown={(event) => {
                  if (event.key === 'ArrowDown') {
                    event.preventDefault();
                    setOpen(true);
                    setActive((i) => Math.min(i + 1, suggestions.length - 1));
                  } else if (event.key === 'ArrowUp') {
                    event.preventDefault();
                    setActive((i) => Math.max(0, i - 1));
                  } else if (event.key === 'Enter' && open && suggestions[active]) {
                    event.preventDefault();
                    choose(suggestions[active]);
                  } else if (event.key === 'Escape') setOpen(false);
                }}
              />
              {search.search && (
                <button
                  className="icon-button"
                  aria-label="Limpiar filtro de auditoría"
                  onClick={() => {
                    search.setSearch('');
                    search.setPage(1);
                    setAction('');
                    setOpen(false);
                  }}
                >
                  <X size={15} />
                </button>
              )}
            </label>
            {open && suggestions.length > 0 && (
              <ul id={listId} className="operation-options" role="listbox">
                {suggestions.map((o, i) => (
                  <li
                    role="option"
                    aria-selected={i === active}
                    id={`${listId}-${i}`}
                    key={o.code}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => choose(o)}
                  >
                    {o.label}
                  </li>
                ))}
              </ul>
            )}
          </div>
          <span className="toolbar-note">
            {action ? 'Operación seleccionada' : 'Historial de operaciones'}
          </span>
        </div>
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <PageError message={result.error} retry={result.retry} />
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th scope="col">Operación</th>
                  <th scope="col">Persona</th>
                  <th scope="col">Fecha</th>
                  <th scope="col">Referencia</th>
                </tr>
              </thead>
              <tbody>
                {result.data?.items.map((e) => (
                  <tr key={e.id}>
                    <td>
                      <button
                        className="audit-operation"
                        aria-label={`Ver detalle de ${e.label} ${e.reference}`}
                        onClick={() => setSelected(e.id)}
                      >
                        <strong>
                          {e.label}
                          <ArrowUpRight size={14} />
                        </strong>
                        <small>{e.summary}</small>
                      </button>
                    </td>
                    <td>{e.actorName}</td>
                    <td className="muted">{date(e.createdAt, session.company.timezone)}</td>
                    <td>
                      {['sale', 'product', 'cash'].includes(e.subjectType) ? (
                        <button className="text-button" onClick={() => related(e)}>
                          {e.reference}
                        </button>
                      ) : (
                        e.reference
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!result.data?.items.length && (
              <p className="empty-list">No hay operaciones que coincidan con el filtro.</p>
            )}
          </div>
        )}
        {result.data && (
          <Pagination page={search.page} total={result.data.total} onPage={search.setPage} />
        )}
      </section>
      {selected && (
        <Detail
          id={selected}
          session={session}
          onClose={() => setSelected('')}
          onReceipt={onReceipt}
          onProduct={onProduct}
          onCash={onCash}
        />
      )}
    </>
  );
}
