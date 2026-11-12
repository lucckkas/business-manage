import { useState } from 'react';
import { api, money, stockLabel, type Product, type Session, type Schema } from './api/client';
import { ErrorMessage, Loading, Modal, PageError, useResource, useSearch } from './ui';
import { BarcodeManager } from './Barcodes';
import { useProductNames } from './ProductSelector';

export function productBody(p: Product) {
  return {
    sku: p.sku,
    name: p.name,
    unit: p.unit,
    priceMinor: p.priceMinor,
    sellable: p.sellable,
    stockMode: p.stockMode,
    stockPolicy: p.stockPolicy,
    components: p.components,
    version: p.version,
    barcodes: p.barcodes,
  };
}
export default function ProductDetails({
  id,
  session,
  onClose,
  onEdit,
  onStock,
  onAdjust,
  onChange,
}: {
  id: string;
  session: Session;
  onClose: () => void;
  onEdit: (p: Product) => void;
  onStock: (p: Product) => void;
  onAdjust: (p: Product) => void;
  onChange: () => void;
}) {
  const resource = useResource<Product>(`/products/${id}`),
    p = resource.data;
  const names = useProductNames(p?.components.map((c) => c.productId) ?? []);
  return (
    <Modal title="Detalle del producto" onClose={onClose} className="detail-dialog">
      {resource.loading ? (
        <Loading />
      ) : resource.error ? (
        <PageError message={resource.error} retry={resource.retry} />
      ) : (
        p && (
          <>
            <h3>{p.name}</h3>
            <p className="form-note">SKU: {p.sku} · Datos actuales del producto</p>
            <dl className="detail-facts">
              <dt>Precio</dt>
              <dd>{money(p.priceMinor, session.company)}</dd>
              <dt>Existencias</dt>
              <dd>{stockLabel(p)}</dd>
              <dt>Disponible para venta</dt>
              <dd>{p.sellable ? 'Sí' : 'No · Insumo'}</dd>
            </dl>
            {p.components.length > 0 && (
              <div className="detail-components">
                <h4>Consumo por unidad</h4>
                {p.components.map((c) => (
                  <p key={c.productId}>
                    {names[c.productId] ?? 'Cargando insumo…'} × {c.quantity}
                  </p>
                ))}
              </div>
            )}
            {session.user.role === 'admin' && (
              <div className="row-actions">
                <button className="button secondary" onClick={() => onEdit(p)}>
                  Editar producto
                </button>
                {p.stockMode === 'own' && p.stockPolicy !== 'untracked' && (
                  <>
                    <button className="button secondary" onClick={() => onStock(p)}>
                      Ingresar existencias
                    </button>
                    <button className="button secondary" onClick={() => onAdjust(p)}>
                      Ajustar existencias
                    </button>
                  </>
                )}
              </div>
            )}
            <BarcodeManager
              product={p}
              admin={session.user.role === 'admin'}
              onChange={() => {
                resource.retry();
                onChange();
              }}
            />
          </>
        )
      )}
    </Modal>
  );
}
export function UnknownBarcode({
  code,
  admin,
  onClose,
  onCreate,
  onLinked,
}: {
  code: string;
  admin: boolean;
  onClose: () => void;
  onCreate: () => void;
  onLinked: () => void;
}) {
  const search = useSearch(),
    result = useResource<Schema['ProductPage']>(`/products?${search.query}`);
  const [binding, setBinding] = useState(false),
    [busy, setBusy] = useState(''),
    [error, setError] = useState('');
  async function link(id: string) {
    setBusy(id);
    setError('');
    try {
      const p = await api<Product>(`/products/${id}`);
      await api(`/products/${id}`, 'PATCH', {
        ...productBody(p),
        barcodes: [
          ...p.barcodes,
          { value: code, format: 'CODE128', primary: !p.barcodes.length, generated: false },
        ],
      });
      onLinked();
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  return (
    <Modal title="Código sin registrar" onClose={onClose} busy={!!busy} className="detail-dialog">
      <p>
        No hay un producto asociado al código <strong>{code}</strong>.
      </p>
      <ErrorMessage message={error} />
      {admin ? (
        <>
          <div className="row-actions">
            <button className="button primary" onClick={onCreate}>
              Crear producto con este código
            </button>
            <button className="button secondary" onClick={() => setBinding(true)}>
              Vincular a un producto existente
            </button>
          </div>
          {binding && (
            <section className="barcode-link">
              <label>
                Buscar producto
                <input
                  aria-label="Buscar producto para vincular"
                  value={search.search}
                  onChange={(e) => search.setSearch(e.target.value)}
                />
              </label>
              {result.loading ? (
                <Loading />
              ) : result.error ? (
                <ErrorMessage message={result.error} />
              ) : (
                result.data?.items.map((p) => (
                  <button
                    className="link-product"
                    key={p.id}
                    disabled={!!busy}
                    onClick={() => link(p.id)}
                  >
                    <strong>{p.name}</strong>
                    <span>{p.sku}</span>
                    <span>Vincular</span>
                  </button>
                ))
              )}
              <p className="form-note">
                Se muestran hasta veinte resultados. Busca por nombre o SKU para localizar el
                producto.
              </p>
            </section>
          )}
        </>
      ) : (
        <p className="form-note">Un administrador puede registrar o vincular este código.</p>
      )}
      <p className="form-note">
        Después de guardarlo, vuelve a escanear para continuar. Esta lectura no modifica
        existencias.
      </p>
    </Modal>
  );
}
