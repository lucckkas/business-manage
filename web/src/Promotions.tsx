import { useState, type FormEvent } from 'react';
import { Plus, Tag } from 'lucide-react';
import { api, date, money, priceInput, quantity, type Schema, type Session } from './api/client';
import {
  ErrorMessage,
  Field,
  Loading,
  Modal,
  PageError,
  Pagination,
  useResource,
  useSearch,
} from './ui';
import { Heading, SearchBox } from './Pages';
import ProductSelector from './ProductSelector';
import NumericInput from './NumericInput';
import { minorInput } from './ProductEditor';
type Rule = Schema['Promotion'];
export function companyLocal(iso: string, timezone: string) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(new Date(iso));
  const values = Object.fromEntries(parts.map((p) => [p.type, p.value]));
  return `${values.year}-${values.month}-${values.day}T${values.hour}:${values.minute}`;
}
export function companyInstant(local: string, timezone: string): string | null {
  if (!local) return null;
  const target = Date.parse(`${local}:00Z`);
  if (!Number.isFinite(target)) throw new Error('Fecha inválida.');
  let candidate = target;
  for (let i = 0; i < 5; i++) {
    const actual = companyLocal(new Date(candidate).toISOString(), timezone);
    if (actual === local) return new Date(candidate).toISOString();
    candidate += target - Date.parse(`${actual}:00Z`);
  }
  throw new Error(
    'Esta hora no existe en la zona de la empresa debido al cambio de horario. Elige otra hora.'
  );
}
function ruleStatus(r: Rule) {
  if (!r.enabled) return 'Pausada';
  const now = Date.now();
  if (r.startsAt && Date.parse(r.startsAt) > now) return 'Programada';
  if (r.endsAt && Date.parse(r.endsAt) <= now) return 'Finalizada';
  return 'Activa';
}
function PromotionEditor({
  initial,
  session,
  onClose,
  onSuccess,
}: {
  initial?: Rule;
  session: Session;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [name, setName] = useState(initial?.name ?? ''),
    [kind, setKind] = useState<string>(initial?.kind ?? 'discount'),
    [scope, setScope] = useState<string>(initial?.scope ?? 'products'),
    [ids, setIds] = useState(initial?.productIds ?? []),
    [rewards, setRewards] = useState(initial?.rewardProductIds ?? []),
    [discountType, setDiscountType] = useState<string>(initial?.discount?.type ?? 'percentage'),
    [value, setValue] = useState(
      initial?.discount?.type === 'amount'
        ? minorInput(initial.discount.value, session.company.currencyScale)
        : (initial?.discount?.value ?? '10')
    ),
    [basis, setBasis] = useState<string>(initial?.discount?.basis ?? 'unit'),
    [buy, setBuy] = useState(initial?.buyQuantity ?? '3'),
    [pay, setPay] = useState(initial?.payQuantity ?? '2'),
    [gift, setGift] = useState(initial?.giftQuantity ?? '1'),
    [priority, setPriority] = useState(String(initial?.priority ?? 100)),
    [enabled, setEnabled] = useState(initial?.enabled ?? true),
    [stackable, setStackable] = useState(initial?.stackable ?? false),
    [start, setStart] = useState(
      initial?.startsAt ? companyLocal(initial.startsAt, session.company.timezone) : ''
    ),
    [end, setEnd] = useState(
      initial?.endsAt ? companyLocal(initial.endsAt, session.company.timezone) : ''
    ),
    [permanent, setPermanent] = useState(!initial?.endsAt),
    [busy, setBusy] = useState(false),
    [error, setError] = useState('');
  async function save(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const body: Schema['SavePromotion'] = {
        name,
        kind: kind as Rule['kind'],
        scope: (kind === 'discount' ? scope : 'products') as Rule['scope'],
        productIds: scope === 'cart' && kind === 'discount' ? [] : ids,
        rewardProductIds: kind === 'buy_get' ? rewards : [],
        startsAt: companyInstant(start, session.company.timezone),
        endsAt: permanent ? null : companyInstant(end, session.company.timezone),
        enabled,
        stackable,
        priority: Number(priority),
        version: initial?.version ?? 0,
        ...(kind === 'discount'
          ? {
              discount: {
                type: discountType as 'percentage' | 'amount',
                value:
                  discountType === 'amount'
                    ? priceInput(value, session.company.currencyScale)
                    : value,
                basis:
                  discountType === 'percentage' || scope === 'cart'
                    ? 'line'
                    : (basis as 'line' | 'unit'),
              },
            }
          : kind === 'buy_pay'
            ? { buyQuantity: buy, payQuantity: pay }
            : { buyQuantity: buy, giftQuantity: gift }),
      };
      await api(
        initial ? `/promotions/${initial.id}` : '/promotions',
        initial ? 'PATCH' : 'POST',
        body
      );
      onSuccess();
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title={initial ? 'Editar oferta' : 'Nueva oferta'} onClose={onClose} busy={busy}>
      <form onSubmit={save}>
        <ErrorMessage message={error} />
        <Field label="Nombre de la oferta">
          <input
            required
            minLength={2}
            maxLength={160}
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>
        <Field label="Tipo de oferta">
          <select
            value={kind}
            onChange={(e) => {
              setKind(e.target.value);
              setScope('products');
              setBuy(e.target.value === 'buy_get' ? '10' : '3');
            }}
          >
            <option value="discount">Descuento porcentual o de monto fijo</option>
            <option value="buy_pay">Lleva X, paga Y</option>
            <option value="buy_get">Compra N y recibe J adicionales gratis</option>
          </select>
        </Field>
        {kind === 'discount' && (
          <>
            <Field label="Aplicar oferta a">
              <select value={scope} onChange={(e) => setScope(e.target.value)}>
                <option value="products">Productos seleccionados</option>
                <option value="cart">Total de la venta</option>
              </select>
            </Field>
            <div className="form-grid">
              <Field label="Tipo de descuento">
                <select
                  value={discountType}
                  onChange={(e) => {
                    setDiscountType(e.target.value);
                    setValue('10');
                  }}
                >
                  <option value="percentage">Porcentaje</option>
                  <option value="amount">Monto fijo</option>
                </select>
              </Field>
              <Field
                label={
                  discountType === 'percentage'
                    ? 'Porcentaje'
                    : `Monto (${session.company.currency})`
                }
              >
                <NumericInput
                  label="Valor de la oferta"
                  value={value}
                  onValueChange={setValue}
                  precision={discountType === 'percentage' ? 2 : session.company.currencyScale}
                  maximum={discountType === 'percentage' ? '100' : '1000000000'}
                  required
                />
              </Field>
            </div>
            {discountType === 'amount' && scope === 'products' && (
              <Field label="Aplicar monto">
                <select value={basis} onChange={(e) => setBasis(e.target.value)}>
                  <option value="unit">Por cada unidad</option>
                  <option value="line">Por toda la línea</option>
                </select>
              </Field>
            )}
          </>
        )}
        {(scope === 'products' || kind !== 'discount') && (
          <ProductSelector
            label="Productos participantes"
            sellingOnly
            ids={ids}
            onChange={setIds}
          />
        )}{' '}
        {kind === 'buy_pay' && (
          <div className="form-grid">
            <Field label="Lleva">
              <NumericInput
                label="Cantidad entregada"
                value={buy}
                onValueChange={setBuy}
                minimum="0.001"
                required
              />
            </Field>
            <Field label="Paga">
              <NumericInput
                label="Cantidad pagada"
                value={pay}
                onValueChange={setPay}
                minimum="0.001"
                required
              />
            </Field>
          </div>
        )}
        {kind === 'buy_get' && (
          <>
            <div className="form-grid">
              <Field label="Por cada unidades compradas">
                <NumericInput
                  label="Cantidad de compra"
                  value={buy}
                  onValueChange={setBuy}
                  minimum="0.001"
                  required
                />
              </Field>
              <Field label="Unidades adicionales gratis">
                <NumericInput
                  label="Cantidad gratis"
                  value={gift}
                  onValueChange={setGift}
                  minimum="0.001"
                  required
                />
              </Field>
            </div>
            <ProductSelector
              label="Productos de regalo"
              sellingOnly
              ids={rewards}
              onChange={setRewards}
            />
            <p className="form-note">
              La caja sugerirá los regalos para que el vendedor elija. Los regalos no generan otros
              regalos.
            </p>
          </>
        )}
        <Field label="Prioridad" hint="Las prioridades más altas se aplican primero.">
          <NumericInput
            label="Prioridad"
            precision={0}
            maximum="100000"
            value={priority}
            onValueChange={setPriority}
            required
          />
        </Field>
        <label className="checkbox-field">
          <input
            type="checkbox"
            checked={stackable}
            onChange={(e) => setStackable(e.target.checked)}
          />
          Acumulable con otras promociones
          <small>Los descuentos manuales autorizados pueden añadirse igualmente.</small>
        </label>
        <label className="checkbox-field">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Oferta habilitada
        </label>
        <label className="checkbox-field">
          <input
            type="checkbox"
            checked={permanent}
            onChange={(e) => {
              setPermanent(e.target.checked);
              if (!e.target.checked && !start)
                setStart(companyLocal(new Date().toISOString(), session.company.timezone));
            }}
          />
          Sin fecha final
        </label>
        <Field
          label="Inicio"
          hint={`Horario de la empresa: ${session.company.timezone}. El inicio se incluye.`}
        >
          <input
            type="datetime-local"
            value={start}
            required={!permanent}
            onChange={(e) => setStart(e.target.value)}
          />
        </Field>
        {!permanent && (
          <Field label="Fin" hint="La oferta deja de aplicarse a partir de esta hora.">
            <input
              type="datetime-local"
              required
              value={end}
              onChange={(e) => setEnd(e.target.value)}
            />
          </Field>
        )}
        <div className="modal-actions">
          <button type="button" className="button secondary" disabled={busy} onClick={onClose}>
            Cancelar
          </button>
          <button className="button primary" disabled={busy}>
            {busy ? 'Guardando…' : 'Guardar oferta'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
export default function Promotions({
  session,
  revision,
  onChange,
  notify,
}: {
  session: Session;
  revision: number;
  onChange: () => void;
  notify: (s: string) => void;
}) {
  const search = useSearch(),
    result = useResource<Schema['PromotionPage']>(`/promotions?${search.query}`, revision),
    [editing, setEditing] = useState<Rule | null | undefined>(undefined),
    [error, setError] = useState(''),
    [busy, setBusy] = useState('');
  async function toggle(rule: Rule) {
    setBusy(rule.id);
    setError('');
    try {
      await api(`/promotions/${rule.id}`, 'PATCH', { ...rule, enabled: !rule.enabled });
      onChange();
      notify(rule.enabled ? 'Oferta pausada.' : 'Oferta habilitada.');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  const label = (r: Rule) =>
    r.kind === 'buy_pay'
      ? `Lleva ${quantity(r.buyQuantity)}, paga ${quantity(r.payQuantity)}`
      : r.kind === 'buy_get'
        ? `Cada ${quantity(r.buyQuantity)} + ${quantity(r.giftQuantity)} gratis`
        : r.discount?.type === 'percentage'
          ? `${quantity(r.discount.value)} %`
          : r.discount
            ? money(r.discount.value, session.company)
            : 'Descuento';
  return (
    <>
      <Heading
        eyebrow="MÁS OPCIONES PARA VENDER"
        title="Descuentos y ofertas"
        description="Reglas comerciales con prioridad, vigencia y trazabilidad."
        action={
          <button className="button primary" onClick={() => setEditing(null)}>
            <Plus size={17} />
            Nueva oferta
          </button>
        }
      />
      <ErrorMessage message={error} />
      <section className="panel">
        <div className="table-toolbar">
          <SearchBox
            value={search.search}
            onChange={search.setSearch}
            placeholder="Buscar ofertas"
          />
          <span className="toolbar-note">
            <Tag size={14} /> Reglas automáticas
          </span>
        </div>
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <PageError message={result.error} retry={result.retry} />
        ) : !result.data?.items.length ? (
          <div className="empty">
            <Tag size={30} />
            <h3>Tu primera oferta empieza aquí</h3>
            <p>Configura descuentos, lleva y paga o regalos adicionales.</p>
          </div>
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Oferta</th>
                  <th>Beneficio</th>
                  <th>Estado</th>
                  <th>Prioridad</th>
                  <th>Vigencia</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {result.data.items.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <strong>{r.name}</strong>
                      <small className="block-muted">
                        {r.stackable ? 'Acumulable' : 'No acumulable con otras ofertas'}
                      </small>
                    </td>
                    <td>{label(r)}</td>
                    <td>
                      <span className={`badge ${ruleStatus(r) === 'Activa' ? 'green' : 'neutral'}`}>
                        {ruleStatus(r)}
                      </span>
                    </td>
                    <td>{r.priority}</td>
                    <td>
                      {r.endsAt
                        ? `Hasta ${date(r.endsAt, session.company.timezone)}`
                        : 'Sin fecha final'}
                    </td>
                    <td>
                      <div className="row-actions">
                        <button className="text-button" onClick={() => setEditing(r)}>
                          Editar
                        </button>
                        <button
                          className="text-button"
                          disabled={busy === r.id}
                          onClick={() => toggle(r)}
                        >
                          {r.enabled ? 'Pausar' : 'Activar'}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {result.data && (
          <Pagination page={search.page} total={result.data.total} onPage={search.setPage} />
        )}
      </section>
      {editing !== undefined && (
        <PromotionEditor
          initial={editing ?? undefined}
          session={session}
          onClose={() => setEditing(undefined)}
          onSuccess={() => {
            onChange();
            notify('Oferta guardada. Las ventas anteriores conservan su beneficio.');
          }}
        />
      )}
    </>
  );
}
