import { useRef, useState, type FormEvent } from 'react';
import { ArrowDownToLine, ArrowUpFromLine, Banknote, Plus } from 'lucide-react';
import {
  ApiError,
  api,
  date,
  money,
  priceInput,
  type Sale,
  type Schema,
  type Session,
} from './api/client';
import {
  Empty,
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
import NumericInput from './NumericInput';
import { minorInput } from './ProductEditor';
import { jobLabels, usePrintJob } from './Printing';

type Movement = Schema['CashMovement'];
type Draft = {
  key: string;
  amount: string;
  reason: string;
  submitted: boolean;
  request?: Schema['CashExpense'];
};
const storageKey = (userId: string) => `erp-cash-expense:${userId}`;
function readDraft(userId: string): Draft {
  try {
    const draft = JSON.parse(localStorage.getItem(storageKey(userId)) ?? 'null') as Draft | null;
    if (
      draft &&
      typeof draft.key === 'string' &&
      typeof draft.amount === 'string' &&
      typeof draft.reason === 'string' &&
      typeof draft.submitted === 'boolean'
    )
      return draft;
  } catch {
    /* A new draft can still be prepared; persistence is checked before sending. */
  }
  return { key: crypto.randomUUID(), amount: '', reason: '', submitted: false };
}
function DrawerStatus({ id }: { id: string }) {
  const { job, error } = usePrintJob(id);
  return (
    <div className="choice-warning" role="status">
      <strong>Apertura del cajón · {job ? jobLabels[job.state] : 'Consultando…'}</strong>
      <p>
        {error ||
          job?.error ||
          (job?.state === 'sent'
            ? 'La orden fue enviada. Comprueba que el cajón se haya abierto.'
            : 'El egreso está registrado. El cajón se procesa por separado.')}
      </p>
    </div>
  );
}
export function CashDetail({
  id,
  session,
  onClose,
}: {
  id: string;
  session: Session;
  onClose: () => void;
}) {
  const result = useResource<Movement>(`/cash/movements/${id}`);
  const m = result.data;
  return (
    <Modal title={m?.reference ?? 'Detalle de caja'} onClose={onClose} className="detail-dialog">
      {result.loading ? (
        <Loading />
      ) : result.error ? (
        <PageError message={result.error} retry={result.retry} />
      ) : (
        m && (
          <>
            <dl className="detail-facts">
              <dt>Monto</dt>
              <dd>{money(m.amountMinor, session.company)}</dd>
              <dt>Motivo</dt>
              <dd>{m.reason || 'Venta al contado'}</dd>
              <dt>Responsable</dt>
              <dd>{m.actorName}</dd>
              <dt>Fecha</dt>
              <dd>{date(m.createdAt, session.company.timezone)}</dd>
              {m.balanceBefore !== undefined && (
                <>
                  <dt>Saldo anterior</dt>
                  <dd>{money(m.balanceBefore, session.company)}</dd>
                </>
              )}
              {m.balanceAfter !== undefined && (
                <>
                  <dt>Saldo después del egreso</dt>
                  <dd>{money(m.balanceAfter, session.company)}</dd>
                </>
              )}
            </dl>
            {m.balanceAfter !== undefined && BigInt(m.balanceAfter) < 0n && (
              <p className="choice-warning">
                El saldo registrado quedó negativo. Revisa el efectivo disponible y los movimientos
                registrados.
              </p>
            )}
            {m.drawerJobId && <DrawerStatus id={m.drawerJobId} />}
            <button className="button secondary" onClick={onClose}>
              Cerrar
            </button>
          </>
        )
      )}
    </Modal>
  );
}
function ExpenseForm({
  session,
  balance,
  onClose,
  onComplete,
}: {
  session: Session;
  balance?: string;
  onClose: () => void;
  onComplete: (m: Movement) => void;
}) {
  const [draft, setDraft] = useState(() => readDraft(session.user.id));
  const [error, setError] = useState(''),
    [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  function persist(next: Draft) {
    localStorage.setItem(storageKey(session.user.id), JSON.stringify(next));
    setDraft(next);
  }
  function edit(next: Draft) {
    try {
      persist({ ...next, key: crypto.randomUUID() });
      setError('');
    } catch {
      setDraft(next);
      setError(
        'No se pudo guardar el borrador en este navegador. Libera espacio antes de registrar el egreso.'
      );
    }
  }
  let projected: bigint | null = null;
  try {
    if (!draft.submitted && balance !== undefined && draft.amount)
      projected = BigInt(balance) - BigInt(priceInput(draft.amount, session.company.currencyScale));
  } catch {
    /* Input validation appears on submit. */
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError('');
    try {
      const body =
        draft.submitted && draft.request
          ? draft.request
          : {
              amountMinor: priceInput(draft.amount, session.company.currencyScale),
              reason: draft.reason.trim(),
            };
      if (BigInt(body.amountMinor) <= 0n)
        throw new Error('El monto del egreso debe ser mayor que cero.');
      if (body.reason.length < 3) throw new Error('Indica un motivo de al menos 3 caracteres.');
      const pending = { ...draft, submitted: true, request: body };
      persist(pending);
      const result = await api<Movement>('/cash/expenses', 'POST', body, {
        'Idempotency-Key': draft.key,
      });
      localStorage.removeItem(storageKey(session.user.id));
      onComplete(result);
    } catch (e) {
      if (e instanceof ApiError && [400, 403].includes(e.status)) {
        try {
          persist({ ...draft, submitted: false, request: undefined });
        } catch {
          /* Keep the in-memory recovery key. */
        }
      }
      setError((e as Error).message);
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }
  return (
    <Modal title="Registrar egreso de caja" onClose={onClose} busy={busy}>
      <form onSubmit={submit}>
        <ErrorMessage message={error} />
        {draft.submitted && (
          <p className="choice-warning" role="status">
            El egreso está pendiente de confirmación. Reintenta con estos mismos datos para
            recuperar el resultado sin duplicarlo.
          </p>
        )}
        <Field
          label={`Monto (${session.company.currency})`}
          hint="Dinero en efectivo que sale de la caja."
        >
          <NumericInput
            label="Monto del egreso"
            value={draft.amount}
            onValueChange={(amount) => edit({ ...draft, amount })}
            precision={session.company.currencyScale}
            minimum={minorInput('1', session.company.currencyScale)}
            maximum={minorInput('1000000000000000', session.company.currencyScale)}
            required
            autoFocus
            disabled={busy || draft.submitted}
          />
        </Field>
        <Field label="Motivo del egreso">
          <textarea
            value={draft.reason}
            onChange={(event) => edit({ ...draft, reason: event.target.value })}
            required
            minLength={3}
            maxLength={240}
            rows={3}
            disabled={busy || draft.submitted}
            placeholder="Ej. Compra de papel higiénico"
          />
        </Field>
        {projected !== null && (
          <p className={projected < 0n ? 'choice-warning' : 'form-note'} role="status">
            Saldo después del egreso:{' '}
            <strong>{money(projected.toString(), session.company)}</strong>.
            {projected < 0n &&
              ' Quedará negativo. Puedes registrar el gasto; revisa el efectivo disponible.'}
          </p>
        )}
        <p className="form-note">
          Si la apertura automática está habilitada y verificada, se enviará una orden al cajón
          después de registrar el egreso.
        </p>
        <div className="modal-actions">
          <button type="button" className="button secondary" disabled={busy} onClick={onClose}>
            {draft.submitted ? 'Cerrar y conservar pendiente' : 'Cancelar'}
          </button>
          <button className="button primary" disabled={busy}>
            {busy ? 'Registrando…' : draft.submitted ? 'Reintentar registro' : 'Registrar egreso'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
export default function Cash({
  session,
  revision,
  onChange,
  onReceipt,
  onMovement,
}: {
  session: Session;
  revision: number;
  onChange: () => void;
  onReceipt: (s: Sale) => void;
  onMovement: (id: string) => void;
}) {
  const search = useSearch(),
    [kind, setKind] = useState('');
  const [open, setOpen] = useState(() => readDraft(session.user.id).submitted),
    [error, setError] = useState('');
  const result = useResource<Schema['CashPage']>(
    `/cash/movements?${search.query}&kind=${kind}`,
    revision
  );
  const summary = useResource<Schema['Summary']>('/reports/summary', revision);
  const pending = readDraft(session.user.id).submitted;
  const allowed = session.user.role === 'admin' || session.user.canRecordCashExpenses;
  async function view(m: Movement) {
    setError('');
    try {
      if (m.saleId) onReceipt(await api<Sale>(`/sales/${m.saleId}`));
      else onMovement(m.id);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <>
      <Heading
        eyebrow="EL EFECTIVO DE TU NEGOCIO"
        title="Caja"
        description="Ingresos por ventas y egresos para los gastos de tu empresa."
        action={
          (allowed || pending) && (
            <button className="button primary" onClick={() => setOpen(true)}>
              <Plus size={17} />
              {pending ? 'Recuperar egreso pendiente' : 'Registrar egreso'}
            </button>
          )
        }
      />
      <div className="cash-summary">
        {[
          { label: 'Ingresos', value: summary.data?.cashIncomeMinor },
          { label: 'Egresos', value: summary.data?.cashExpenseMinor },
          { label: 'Saldo registrado', value: summary.data?.cashMinor },
        ].map((item) => (
          <div className="cash-banner" key={item.label}>
            <span className="cash-banner-icon">
              <Banknote size={23} />
            </span>
            <div>
              <span>{item.label}</span>
              <strong>{item.value !== undefined ? money(item.value, session.company) : '—'}</strong>
            </div>
          </div>
        ))}
      </div>
      {summary.error && <PageError message={summary.error} retry={summary.retry} />}
      <ErrorMessage message={error} />
      <section className="panel">
        <div className="table-toolbar cash-toolbar">
          <SearchBox
            value={search.search}
            onChange={search.setSearch}
            placeholder="Buscar referencia o motivo"
          />
          <select
            aria-label="Tipo de movimiento"
            value={kind}
            onChange={(event) => {
              setKind(event.target.value);
              search.setPage(1);
            }}
          >
            <option value="">Todos los movimientos</option>
            <option value="sale">Ingresos</option>
            <option value="expense">Egresos</option>
          </select>
        </div>
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <PageError message={result.error} retry={result.retry} />
        ) : result.data?.items.length ? (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th scope="col">Movimiento</th>
                  <th scope="col">Fecha</th>
                  <th scope="col">Responsable</th>
                  <th scope="col">Motivo</th>
                  <th scope="col" className="numeric">
                    Importe
                  </th>
                </tr>
              </thead>
              <tbody>
                {result.data.items.map((m) => (
                  <tr key={m.id}>
                    <td>
                      <button className="text-button table-document" onClick={() => void view(m)}>
                        {m.kind === 'expense' ? (
                          <ArrowUpFromLine size={16} />
                        ) : (
                          <ArrowDownToLine size={16} />
                        )}
                        <strong>{m.reference}</strong>
                      </button>
                    </td>
                    <td className="muted">{date(m.createdAt, session.company.timezone)}</td>
                    <td>{m.actorName}</td>
                    <td className="cash-reason">{m.reason || 'Venta al contado'}</td>
                    <td
                      className={`numeric ${m.kind === 'expense' ? 'cash-negative' : 'positive'}`}
                    >
                      {m.kind === 'sale' ? '+ ' : '− '}
                      {money(m.amountMinor.replace(/^-/, ''), session.company)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title="Sin movimientos para mostrar"
            description="Los ingresos por ventas y los egresos registrados aparecerán aquí."
          />
        )}
        {result.data && (
          <Pagination page={search.page} total={result.data.total} onPage={search.setPage} />
        )}
      </section>
      {open && (
        <ExpenseForm
          session={session}
          balance={summary.data?.cashMinor}
          onClose={() => setOpen(false)}
          onComplete={(m) => {
            setOpen(false);
            onChange();
            onMovement(m.id);
          }}
        />
      )}
    </>
  );
}
