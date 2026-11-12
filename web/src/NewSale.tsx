import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ArrowLeft, Check, Gift, LockKeyhole, Plus, ShoppingBag, Tag, Trash2 } from 'lucide-react';
import {
  ApiError,
  api,
  money,
  quantity,
  quantityMilli,
  priceInput,
  stockLabel,
  type Product,
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
import NumericInput, { decimal, decimalText } from './NumericInput';
import { minorInput } from './ProductEditor';
import { useProductNames } from './ProductSelector';
import ScanField, { useScanReceiver } from './Scanner';
type CartLine = { product: Product; quantity: string };
type Cart = {
  key: string;
  lines: CartLine[];
  submitted: boolean;
  lineDiscounts?: Schema['LineDiscount'][];
  totalDiscount?: Schema['Discount'];
  discountReason?: string;
  rewards?: Schema['RewardSelection'][];
  quoteFingerprint?: string;
  quoted?: Schema['SaleQuote'];
};
function fresh(): Cart {
  return { key: crypto.randomUUID(), lines: [], submitted: false };
}
function request(cart: Cart): Schema['SaleRequest'] {
  return {
    lines: cart.lines.map((l) => ({ productId: l.product.id, quantity: l.quantity })),
    ...(cart.lineDiscounts?.length ? { lineDiscounts: cart.lineDiscounts } : {}),
    ...(cart.totalDiscount ? { totalDiscount: cart.totalDiscount } : {}),
    ...(cart.lineDiscounts?.length || cart.totalDiscount
      ? { discountReason: cart.discountReason ?? '' }
      : {}),
    ...(cart.rewards?.length ? { rewards: cart.rewards } : {}),
  };
}
function DiscountEditor({
  session,
  initial,
  initialReason,
  title,
  onSave,
  onClose,
  total = false,
}: {
  session: Session;
  initial?: Schema['Discount'];
  initialReason: string;
  title: string;
  onSave: (d: Schema['Discount'], reason: string) => void;
  onClose: () => void;
  total?: boolean;
}) {
  const [type, setType] = useState(initial?.type ?? 'percentage'),
    [value, setValue] = useState(
      initial
        ? initial.type === 'amount'
          ? minorInput(initial.value, session.company.currencyScale)
          : initial.value
        : '10'
    ),
    [reason, setReason] = useState(initialReason),
    [basis, setBasis] = useState(initial?.basis ?? 'line'),
    [error, setError] = useState('');
  function save(event: FormEvent) {
    event.preventDefault();
    try {
      const amount =
        type === 'amount'
          ? priceInput(value, session.company.currencyScale)
          : decimalText(decimal(value, 2), 2);
      if (decimal(amount, type === 'percentage' ? 2 : 0) <= 0n)
        throw new Error('Indica un descuento mayor que cero.');
      if (reason.trim().length < 3 || reason.trim().length > 240)
        throw new Error('Indica un motivo de 3 a 240 caracteres.');
      onSave(
        { type, value: amount, basis: total || type === 'percentage' ? 'line' : basis },
        reason.trim()
      );
      onClose();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <Modal title={title} onClose={onClose}>
      <form onSubmit={save}>
        <ErrorMessage message={error} />
        <Field label="Tipo de descuento">
          <select
            value={type}
            onChange={(e) => {
              setType(e.target.value as typeof type);
              setValue('10');
            }}
          >
            <option value="percentage">Porcentaje</option>
            <option value="amount">Monto fijo</option>
          </select>
        </Field>
        <Field label={type === 'percentage' ? 'Porcentaje' : `Monto (${session.company.currency})`}>
          <NumericInput
            label="Valor del descuento"
            precision={type === 'percentage' ? 2 : session.company.currencyScale}
            maximum={type === 'percentage' ? '100' : '1000000000'}
            value={value}
            onValueChange={setValue}
            required
            autoFocus
          />
        </Field>
        <Field
          label="Motivo del descuento"
          hint="Este motivo se aplica a todos los descuentos manuales de esta venta."
        >
          <textarea
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            required
            minLength={3}
            maxLength={240}
            rows={2}
            placeholder="Ej. Acuerdo comercial con el cliente"
          />
        </Field>
        {!total && type === 'amount' && (
          <Field label="Aplicar el monto">
            <select value={basis} onChange={(e) => setBasis(e.target.value as 'line' | 'unit')}>
              <option value="line">A toda la línea</option>
              <option value="unit">Por cada unidad</option>
            </select>
          </Field>
        )}
        <div className="modal-actions">
          <button type="button" className="button secondary" onClick={onClose}>
            Cancelar
          </button>
          <button className="button primary">Aplicar descuento</button>
        </div>
      </form>
    </Modal>
  );
}
function GiftOption({
  option,
  disabled,
  onAdd,
}: {
  option: Schema['RewardOption'];
  disabled: boolean;
  onAdd: (id: string, quantity: string) => void;
}) {
  const remaining = quantityMilli(option.maximumQuantity) - quantityMilli(option.selectedQuantity);
  const [id, setId] = useState(option.products.length === 1 ? option.products[0].productId : ''),
    [amount, setAmount] = useState(decimalText(remaining < 1000n ? remaining : 1000n, 3));
  let valid = false;
  try {
    valid = !!id && quantityMilli(amount) > 0n && quantityMilli(amount) <= remaining;
  } catch {}
  if (remaining <= 0n || !option.products.length) return null;
  return (
    <div className="gift-option">
      <strong>
        <Gift size={16} />
        {option.name}
      </strong>
      <p>Regalos disponibles: {quantity(decimalText(remaining, 3))}</p>
      <select
        aria-label={`Regalo de ${option.name}`}
        value={id}
        onChange={(e) => setId(e.target.value)}
        disabled={disabled}
      >
        <option value="">Elige un producto</option>
        {option.products.map((p) => (
          <option key={p.productId} value={p.productId}>
            {p.name}
          </option>
        ))}
      </select>
      <NumericInput
        label={`Cantidad de regalo ${option.name}`}
        value={amount}
        onValueChange={setAmount}
        minimum="0.001"
        maximum={decimalText(remaining, 3)}
        disabled={disabled}
      />
      <button
        type="button"
        className="button secondary full"
        disabled={disabled || !valid}
        onClick={() => onAdd(id, amount)}
      >
        <Gift size={15} />
        Agregar regalo
      </button>
    </div>
  );
}
export default function NewSale({
  session,
  revision,
  onReceipt,
  onChange,
  navigate,
  onUnknown,
}: {
  session: Session;
  revision: number;
  onReceipt: (s: Sale) => void;
  onChange: () => void;
  navigate: (p: string) => void;
  onUnknown: (code: string) => void;
}) {
  const storageKey = `raiz-cart:${session.user.id}`;
  const [cart, setCart] = useState<Cart>(() => {
      try {
        const saved = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null') as Cart | null;
        return saved && Array.isArray(saved.lines) && typeof saved.key === 'string'
          ? saved
          : fresh();
      } catch {
        return fresh();
      }
    }),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [quote, setQuote] = useState<Schema['SaleQuote'] | null>(null),
    [quoting, setQuoting] = useState(false),
    [quoteError, setQuoteError] = useState(''),
    [quoteRetry, setQuoteRetry] = useState(0),
    [discountTarget, setDiscountTarget] = useState<string | null>(null);
  const search = useSearch(),
    products = useResource<Schema['ProductPage']>(
      `/products?${search.query}&sellable=true`,
      revision
    );
  const giftNames = useProductNames((cart.rewards ?? []).map((r) => r.productId));
  const body = JSON.stringify(request(cart)),
    canDiscount = session.user.role === 'admin' || session.user.canApplyManualDiscounts;

  const latestCart = useRef(cart);
  latestCart.current = cart;
  useScanReceiver(async (code) => {
    if (latestCart.current.submitted || busy) {
      setError('Primero recupera la venta pendiente de confirmación.');
      return;
    }
    try {
      const p = await api<Product>(`/products/lookup?barcode=${encodeURIComponent(code)}`);
      if (!p.sellable) {
        setError('Este código corresponde a un insumo que no está disponible para venta.');
        return;
      }
      if (p.availableQuantity != null && quantityMilli(p.availableQuantity) <= 0n) {
        setError(`Existencias insuficientes para ${p.name}.`);
        return;
      }
      add(p);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) onUnknown(code);
      else setError((e as Error).message);
    }
  });
  function update(next: Cart) {
    latestCart.current = next;
    sessionStorage.setItem(storageKey, JSON.stringify(next));
    setCart(next);
  }
  function edit(next: Cart) {
    if (JSON.stringify(request(next)) !== body) {
      setQuote(null);
      setQuoting(next.lines.length > 0);
      setQuoteError('');
    }
    update({
      ...next,
      key: crypto.randomUUID(),
      submitted: false,
      quoteFingerprint: undefined,
      quoted: undefined,
    });
    setError('');
  }
  useEffect(() => {
    if (cart.submitted || !cart.lines.length) {
      setQuoting(false);
      setQuote(null);
      return;
    }
    const controller = new AbortController();
    setQuoting(true);
    setQuote(null);
    setQuoteError('');
    const timer = setTimeout(() => {
      api<Schema['SaleQuote']>('/sales/quote', 'POST', JSON.parse(body), {}, controller.signal)
        .then((q) => {
          if (controller.signal.aborted) return;
          setQuote(q);
          setCart((current) => {
            const next = { ...current, quoteFingerprint: q.fingerprint, quoted: q };
            sessionStorage.setItem(storageKey, JSON.stringify(next));
            return next;
          });
        })
        .catch((e) => {
          if (!controller.signal.aborted) setQuoteError((e as Error).message);
        })
        .finally(() => {
          if (!controller.signal.aborted) setQuoting(false);
        });
    }, 200);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [body, cart.submitted, revision, quoteRetry, storageKey]);
  function add(p: Product) {
    const current = latestCart.current;
    if (current.submitted) return;
    try {
      const found = current.lines.find((l) => l.product.id === p.id);
      const lines = found
        ? current.lines.map((l) =>
            l.product.id === p.id
              ? { ...l, quantity: decimalText(quantityMilli(l.quantity) + 1000n, 3) }
              : l
          )
        : [...current.lines, { product: p, quantity: '1' }];
      edit({ ...current, lines });
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function confirm() {
    setError('');
    if (!cart.submitted && (!quote || quoting)) return;
    setBusy(true);
    const pending = {
      ...cart,
      submitted: true,
      quoteFingerprint: quote?.fingerprint ?? cart.quoteFingerprint,
    };
    update(pending);
    try {
      const sale = await api<Sale>(
        '/sales',
        'POST',
        {
          ...request(pending),
          ...(pending.quoteFingerprint ? { quoteFingerprint: pending.quoteFingerprint } : {}),
        },
        { 'Idempotency-Key': pending.key }
      );
      update(fresh());
      setQuote(null);
      onChange();
      products.retry();
      onReceipt(sale);
      requestAnimationFrame(() =>
        document.querySelector<HTMLInputElement>('[data-scanner]')?.focus()
      );
    } catch (e) {
      setError((e as Error).message);
      if (e instanceof ApiError && e.status >= 400 && e.status < 500 && e.status !== 429) {
        update({ ...pending, submitted: false, quoteFingerprint: undefined, quoted: undefined });
        setQuote(null);
        setQuoteRetry((n) => n + 1);
      }
    } finally {
      setBusy(false);
    }
  }
  function addGift(promotionId: string, productId: string, amount: string) {
    const prior = cart.rewards?.find(
      (r) => r.promotionId === promotionId && r.productId === productId
    );
    const rewards = (cart.rewards ?? []).filter((r) => r !== prior);
    rewards.push({
      promotionId,
      productId,
      quantity: decimalText(quantityMilli(prior?.quantity ?? '0') + quantityMilli(amount), 3),
    });
    edit({ ...cart, rewards });
  }
  const shown = quote ?? cart.quoted;
  const insufficient = quote?.availability.filter((a) => !a.sufficient) ?? [];
  const targetLine = cart.lines.find((l) => l.product.id === discountTarget);
  return (
    <>
      <button className="back-button" onClick={() => navigate('sales')}>
        <ArrowLeft size={16} />
        Volver a ventas
      </button>
      <Heading
        eyebrow="CONVIERTE UN BUEN DÍA EN UNA VENTA"
        title="Nueva venta"
        description="Productos, ofertas y descuentos con el total confirmado por tu servidor."
      />
      <div className="sale-layout">
        <section className="panel sale-catalog">
          <ScanField disabled={cart.submitted || busy} />
          <div className="table-toolbar">
            <SearchBox
              value={search.search}
              onChange={search.setSearch}
              placeholder="Buscar productos para vender"
            />
          </div>
          {products.loading ? (
            <Loading />
          ) : products.error ? (
            <PageError message={products.error} retry={products.retry} />
          ) : products.data?.items.length ? (
            <div className="product-grid">
              {products.data.items.map((p) => (
                <button
                  key={p.id}
                  className="sale-product"
                  disabled={
                    cart.submitted ||
                    (p.availableQuantity != null && quantityMilli(p.availableQuantity) <= 0n)
                  }
                  onClick={() => add(p)}
                  aria-label={`Agregar ${p.name}`}
                >
                  <div className="sale-product-top">
                    <span className="product-avatar">{p.name.slice(0, 2).toUpperCase()}</span>
                    <span className="sale-add">
                      <Plus size={16} />
                    </span>
                  </div>
                  <small>{p.sku}</small>
                  <strong>{p.name}</strong>
                  <div>
                    <b>{money(p.priceMinor, session.company)}</b>
                    <span>{stockLabel(p)}</span>
                  </div>
                </button>
              ))}
            </div>
          ) : (
            <Empty
              title="Aún no hay productos disponibles"
              description="Agrega productos vendibles y configura sus existencias o insumos."
            />
          )}
          {products.data && (
            <Pagination page={search.page} total={products.data.total} onPage={search.setPage} />
          )}
        </section>
        <aside className="panel cart">
          <div className="panel-heading">
            <h2>Detalle de venta</h2>
            <span className="badge neutral">
              {cart.lines.length} {cart.lines.length === 1 ? 'producto' : 'productos'}
            </span>
          </div>
          <div className="cart-body">
            <ErrorMessage message={error} />
            <ErrorMessage message={quoteError} />
            {quoteError && (
              <button className="text-button" onClick={() => setQuoteRetry((n) => n + 1)}>
                Actualizar cotización
              </button>
            )}
            {cart.submitted && (
              <div className="pending-message">
                <LockKeyhole size={18} />
                <span>
                  La venta está pendiente de confirmación. Reintenta para recuperar el resultado de
                  la misma operación.
                </span>
              </div>
            )}
            {cart.lines.length ? (
              cart.lines.map((line) => {
                const priced = shown?.lines.find(
                  (l) => l.kind === 'regular' && l.productId === line.product.id
                );
                const discounted = cart.lineDiscounts?.some((d) => d.productId === line.product.id);
                return (
                  <div className="cart-line" key={line.product.id}>
                    <div>
                      <strong>{line.product.name}</strong>
                      <small>
                        {priced
                          ? money(priced.priceMinor, session.company)
                          : money(line.product.priceMinor, session.company)}{' '}
                        / {line.product.unit}
                      </small>
                    </div>
                    <button
                      className="icon-button"
                      aria-label={`Quitar ${line.product.name}`}
                      disabled={cart.submitted || busy}
                      onClick={() =>
                        edit({
                          ...cart,
                          lines: cart.lines.filter((l) => l.product.id !== line.product.id),
                          lineDiscounts: cart.lineDiscounts?.filter(
                            (d) => d.productId !== line.product.id
                          ),
                          rewards: [],
                        })
                      }
                    >
                      <Trash2 size={15} />
                    </button>
                    <div className="cart-quantity">
                      <span>Cantidad</span>
                      <NumericInput
                        label={`Cantidad de ${line.product.name}`}
                        aria-label={`Cantidad de ${line.product.name}`}
                        minimum="0.001"
                        value={line.quantity}
                        disabled={cart.submitted || busy}
                        onValueChange={(quantity) =>
                          edit({
                            ...cart,
                            lines: cart.lines.map((l) => (l === line ? { ...l, quantity } : l)),
                          })
                        }
                      />
                    </div>
                    <b>{priced ? money(priced.totalMinor, session.company) : '—'}</b>
                    {(canDiscount || discounted) && (
                      <div className="cart-line-actions">
                        {canDiscount && (
                          <button
                            className="text-button"
                            disabled={cart.submitted}
                            onClick={() => setDiscountTarget(line.product.id)}
                            aria-label={`Aplicar descuento a ${line.product.name}`}
                          >
                            <Tag size={13} />
                            {discounted ? 'Editar descuento' : 'Descuento'}
                          </button>
                        )}
                        {discounted && (
                          <button
                            className="text-button"
                            disabled={cart.submitted}
                            onClick={() =>
                              edit({
                                ...cart,
                                lineDiscounts: cart.lineDiscounts?.filter(
                                  (d) => d.productId !== line.product.id
                                ),
                              })
                            }
                          >
                            Quitar descuento
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                );
              })
            ) : (
              <Empty
                title="Una venta por construir"
                description="Selecciona productos del catálogo para empezar."
              />
            )}
            {(cart.rewards ?? []).map((r, i) => (
              <div className="cart-gift" key={`${r.promotionId}:${r.productId}`}>
                <span>
                  <Gift size={16} />
                  {giftNames[r.productId] ?? 'Regalo'}
                  <small>Gratis · {quantity(r.quantity)}</small>
                </span>
                <button
                  className="icon-button"
                  aria-label={`Quitar regalo ${giftNames[r.productId] ?? ''}`}
                  disabled={cart.submitted}
                  onClick={() =>
                    edit({ ...cart, rewards: cart.rewards?.filter((_, index) => index !== i) })
                  }
                >
                  <Trash2 size={15} />
                </button>
                <NumericInput
                  label={`Cantidad gratis de ${giftNames[r.productId] ?? 'regalo'}`}
                  value={r.quantity}
                  minimum="0.001"
                  disabled={cart.submitted}
                  onValueChange={(quantity) =>
                    edit({
                      ...cart,
                      rewards: cart.rewards?.map((item, index) =>
                        index === i ? { ...item, quantity } : item
                      ),
                    })
                  }
                />
              </div>
            ))}
            {quote?.rewardOptions.map((option) => (
              <GiftOption
                key={option.promotionId}
                option={option}
                disabled={cart.submitted}
                onAdd={(id, quantity) => addGift(option.promotionId, id, quantity)}
              />
            ))}
            {insufficient.length > 0 && (
              <ErrorMessage
                message={`Existencias insuficientes: ${insufficient.map((a) => a.name).join(', ')}.`}
              />
            )}
          </div>
          <div className="cart-footer">
            <div className="payment-method">
              <span>
                <ShoppingBag size={17} />
                Pago al contado
              </span>
              <Check size={16} />
            </div>
            {(canDiscount || cart.totalDiscount) && (
              <div
                className="cart-total-discount-actions"
                role="group"
                aria-label="Descuento sobre el total"
              >
                {canDiscount && (
                  <button
                    type="button"
                    className="text-button"
                    disabled={cart.submitted || !cart.lines.length}
                    onClick={() => setDiscountTarget('total')}
                  >
                    <Tag size={14} />
                    <span>
                      {cart.totalDiscount ? 'Editar descuento total' : 'Descuento sobre el total'}
                    </span>
                  </button>
                )}
                {cart.totalDiscount && (
                  <button
                    type="button"
                    className="text-button remove-total-discount"
                    aria-label="Quitar descuento total"
                    disabled={cart.submitted}
                    onClick={() => edit({ ...cart, totalDiscount: undefined })}
                  >
                    <Trash2 size={14} />
                    <span>Quitar</span>
                  </button>
                )}
              </div>
            )}
            {(!!cart.lineDiscounts?.length || !!cart.totalDiscount) && (
              <p className="discount-reason-summary">
                <strong>Motivo del descuento:</strong>{' '}
                {cart.discountReason || 'Edita el descuento para indicar el motivo.'}
              </p>
            )}
            {shown && (
              <div className="quote-breakdown">
                <div>
                  <span>Subtotal</span>
                  <span>{money(shown.subtotalMinor, session.company)}</span>
                </div>
                {shown.promotionDiscountMinor !== '0' && (
                  <div>
                    <span>Ofertas y regalos</span>
                    <span>− {money(shown.promotionDiscountMinor, session.company)}</span>
                  </div>
                )}
                {shown.manualDiscountMinor !== '0' && (
                  <div>
                    <span>Descuentos manuales</span>
                    <span>− {money(shown.manualDiscountMinor, session.company)}</span>
                  </div>
                )}
                <div className="applied-offers">
                  {[
                    ...new Set(
                      shown.discounts.filter((d) => d.kind === 'promotion').map((d) => d.label)
                    ),
                  ].map((label) => (
                    <small key={label}>{label}</small>
                  ))}
                </div>
              </div>
            )}
            <div className="cart-total">
              <span>Total</span>
              <strong>{shown ? money(shown.totalMinor, session.company) : '—'}</strong>
            </div>
            <button
              className="button primary full"
              onClick={confirm}
              disabled={
                busy ||
                !cart.lines.length ||
                (!cart.submitted && (!quote || quoting || !!quoteError || insufficient.length > 0))
              }
            >
              {busy
                ? 'Confirmando…'
                : cart.submitted
                  ? 'Reintentar confirmación'
                  : quoting
                    ? 'Cotizando…'
                    : 'Confirmar venta'}
              <ArrowLeft className="arrow-forward" size={18} />
            </button>
            <p>El servidor verifica precios, permisos e insumos al confirmar.</p>
          </div>
        </aside>
      </div>
      {discountTarget && (
        <DiscountEditor
          session={session}
          total={discountTarget === 'total'}
          title={
            discountTarget === 'total'
              ? 'Descuento sobre el total'
              : `Descuento · ${targetLine?.product.name ?? ''}`
          }
          initial={
            discountTarget === 'total'
              ? cart.totalDiscount
              : cart.lineDiscounts?.find((d) => d.productId === discountTarget)?.discount
          }
          initialReason={cart.discountReason ?? ''}
          onClose={() => setDiscountTarget(null)}
          onSave={(discount, reason) => {
            if (discountTarget === 'total')
              edit({ ...cart, totalDiscount: discount, discountReason: reason });
            else
              edit({
                ...cart,
                discountReason: reason,
                lineDiscounts: [
                  ...(cart.lineDiscounts ?? []).filter((d) => d.productId !== discountTarget),
                  { productId: discountTarget, discount },
                ],
              });
          }}
        />
      )}
    </>
  );
}
