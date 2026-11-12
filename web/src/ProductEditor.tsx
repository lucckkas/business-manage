import { useState, type FormEvent } from 'react';
import { api, priceInput, type Product, type Schema, type Session } from './api/client';
import { ErrorMessage, Field, Modal } from './ui';
import NumericInput from './NumericInput';
import ProductSelector, { useProductNames } from './ProductSelector';
import { BarcodeInputs } from './Barcodes';
export function minorInput(value: string, scale: number) {
  const n = BigInt(value || '0'),
    factor = 10n ** BigInt(scale);
  return scale ? `${n / factor}.${(n % factor).toString().padStart(scale, '0')}` : n.toString();
}
export default function ProductEditor({
  initial,
  session,
  onClose,
  onSuccess,
  barcode,
}: {
  initial?: Product;
  session: Session;
  onClose: () => void;
  onSuccess: (s: string) => void;
  barcode?: string;
}) {
  const [name, setName] = useState(initial?.name ?? ''),
    [sku, setSku] = useState(initial?.sku ?? ''),
    [unit, setUnit] = useState<Schema['CreateProduct']['unit']>(initial?.unit ?? 'un'),
    [price, setPrice] = useState(
      minorInput(initial?.priceMinor ?? '0', session.company.currencyScale)
    ),
    [sellable, setSellable] = useState(initial?.sellable ?? true),
    [mode, setMode] = useState(initial?.stockMode ?? 'own'),
    [policy, setPolicy] = useState(initial?.stockPolicy ?? 'block'),
    [components, setComponents] = useState(initial?.components ?? []),
    [barcodes, setBarcodes] = useState<Schema['Barcode'][]>(
      initial?.barcodes ??
        (barcode ? [{ value: barcode, format: 'CODE128', primary: true, generated: false }] : [])
    ),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [count, setCount] = useState('0'),
    [reason, setReason] = useState('');
  const ids = components.map((c) => c.productId),
    names = useProductNames(ids);
  const needsCount =
    !!initial &&
    ((mode === 'own' &&
      policy !== 'untracked' &&
      (initial.stockMode === 'components' || initial.stockPolicy === 'untracked')) ||
      (mode === 'components' && initial.stockMode === 'own' && initial.stock !== '0.000000'));
  async function save(event: FormEvent) {
    event.preventDefault();
    setError('');
    setBusy(true);
    try {
      const body: Schema['UpdateProduct'] = {
        name,
        sku,
        unit,
        priceMinor: priceInput(price, session.company.currencyScale),
        sellable,
        stockMode: mode as 'own' | 'components',
        stockPolicy: (mode === 'components' ? 'block' : policy) as Schema['Product']['stockPolicy'],
        components: mode === 'components' ? components : [],
        barcodes,
        version: initial?.version ?? 0,
        ...(needsCount
          ? { countedStock: mode === 'components' ? '0' : count, adjustmentReason: reason }
          : {}),
      };
      await api(
        initial ? `/products/${initial.id}` : '/products',
        initial ? 'PATCH' : 'POST',
        body
      );
      onSuccess(
        initial
          ? 'Producto actualizado. Las ventas anteriores conservan sus datos.'
          : 'Producto creado.'
      );
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title={initial ? 'Editar producto' : 'Nuevo producto'} onClose={onClose} busy={busy}>
      <form onSubmit={save}>
        <ErrorMessage message={error} />
        <Field label="Nombre del producto">
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            minLength={2}
            maxLength={160}
            required
            autoFocus
          />
        </Field>
        <div className="form-grid">
          <Field label="Código SKU">
            <input value={sku} onChange={(e) => setSku(e.target.value)} required maxLength={64} />
          </Field>
          <Field
            label="Unidad"
            hint={initial ? 'La unidad se conserva cuando existe historial.' : undefined}
          >
            <select value={unit} onChange={(e) => setUnit(e.target.value as typeof unit)}>
              <option value="un">Unidad</option>
              <option value="kg">Kilogramo</option>
              <option value="lt">Litro</option>
              <option value="mt">Metro</option>
            </select>
          </Field>
        </div>
        <Field
          label={`Precio de venta (${session.company.currency})`}
          hint="Sin separador de miles."
        >
          <NumericInput
            label="Precio de venta"
            value={price}
            onValueChange={setPrice}
            precision={session.company.currencyScale}
            maximum="1000000000000000"
            required
          />
        </Field>
        <label className="checkbox-field">
          <input
            type="checkbox"
            checked={sellable}
            onChange={(e) => setSellable(e.target.checked)}
          />
          Disponible para venta <small>Desmarca esta opción para insumos de uso interno.</small>
        </label>
        <BarcodeInputs codes={barcodes} onChange={setBarcodes} />
        <Field label="Tipo de inventario">
          <select value={mode} onChange={(e) => setMode(e.target.value as typeof mode)}>
            <option value="own">Inventario propio</option>
            <option value="components">Consume insumos</option>
          </select>
        </Field>
        {mode === 'own' ? (
          <Field label="Control de existencias">
            <select value={policy} onChange={(e) => setPolicy(e.target.value as typeof policy)}>
              <option value="block">Bloquear ventas sin stock suficiente</option>
              <option value="negative">Permitir saldo negativo</option>
              <option value="zero">Mantener mínimo cero</option>
              <option value="untracked">Sin seguimiento de stock</option>
            </select>
          </Field>
        ) : (
          <>
            <ProductSelector
              label="Insumos"
              ownOnly
              ids={ids}
              exclude={initial ? [initial.id] : []}
              onChange={(next) =>
                setComponents(
                  next.map(
                    (id) =>
                      components.find((c) => c.productId === id) ?? { productId: id, quantity: '1' }
                  )
                )
              }
            />
            {components.map((c) => (
              <Field
                key={c.productId}
                label={`Consumo por unidad · ${names[c.productId] ?? 'Cargando insumo…'}`}
              >
                <NumericInput
                  label={`Consumo de ${names[c.productId] ?? 'insumo'}`}
                  value={c.quantity}
                  minimum="0.001"
                  required
                  onValueChange={(quantity) =>
                    setComponents((old) =>
                      old.map((item) =>
                        item.productId === c.productId ? { ...item, quantity } : item
                      )
                    )
                  }
                />
              </Field>
            ))}
            <p className="form-note">
              La disponibilidad depende de los insumos. Cada insumo conserva su propia política de
              faltantes.
            </p>
          </>
        )}
        {policy === 'zero' && initial?.stock?.startsWith('-') && (
          <p className="form-note">El saldo negativo se ajustará a cero y quedará registrado.</p>
        )}
        {needsCount && (
          <div className="count-fields">
            {mode === 'own' && (
              <Field
                label="Existencias actuales"
                hint="Indica el conteo al volver a controlar el inventario."
              >
                <NumericInput
                  label="Existencias actuales"
                  precision={6}
                  minimum={policy === 'negative' ? '-1000000000' : '0'}
                  value={count}
                  onValueChange={setCount}
                  required
                />
              </Field>
            )}
            <Field
              label="Motivo del cambio de existencias"
              hint={
                mode === 'components'
                  ? 'El stock propio se ajustará a cero para pasar a consumo por insumos.'
                  : undefined
              }
            >
              <input
                required
                minLength={3}
                maxLength={240}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </Field>
          </div>
        )}
        <div className="modal-actions">
          <button type="button" className="button secondary" onClick={onClose} disabled={busy}>
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
