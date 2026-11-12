import { useEffect, useState } from 'react';
import { api, type Product, type Schema } from './api/client';
import { ErrorMessage, Loading, Pagination, useResource, useSearch } from './ui';
import { SearchBox } from './Pages';
export function useProductNames(ids: string[]) {
  const [names, setNames] = useState<Record<string, string>>({});
  const key = [...ids].sort().join(',');
  useEffect(() => {
    const controller = new AbortController();
    Promise.all(
      key
        .split(',')
        .filter(Boolean)
        .map(async (id) => {
          try {
            const p = await api<Schema['Product']>(
              `/products/${id}`,
              'GET',
              undefined,
              {},
              controller.signal
            );
            return [id, p.name] as const;
          } catch {
            return [id, 'Producto no disponible'] as const;
          }
        })
    ).then((items) => {
      if (!controller.signal.aborted) setNames(Object.fromEntries(items));
    });
    return () => controller.abort();
  }, [key]);
  return names;
}
export default function ProductSelector({
  label,
  ids,
  onChange,
  ownOnly = false,
  sellingOnly = false,
  exclude = [],
}: {
  label: string;
  ids: string[];
  onChange: (ids: string[]) => void;
  ownOnly?: boolean;
  sellingOnly?: boolean;
  exclude?: string[];
}) {
  const search = useSearch(),
    result = useResource<Schema['ProductPage']>(
      `/products?${search.query}${ownOnly ? '&stockMode=own' : ''}${sellingOnly ? '&sellable=true' : ''}`
    );
  const names = useProductNames(ids);
  const toggle = (p: Product) =>
    onChange(ids.includes(p.id) ? ids.filter((id) => id !== p.id) : [...ids, p.id]);
  return (
    <fieldset className="product-selector">
      <legend>{label}</legend>
      <SearchBox
        value={search.search}
        onChange={search.setSearch}
        placeholder={`Buscar ${label.toLowerCase()}`}
      />
      {result.loading ? (
        <Loading />
      ) : (
        <>
          <ErrorMessage message={result.error} />
          <div className="selector-options">
            {result.data?.items
              .filter((p) => !exclude.includes(p.id))
              .map((p) => (
                <label key={p.id}>
                  <input type="checkbox" checked={ids.includes(p.id)} onChange={() => toggle(p)} />
                  <span>
                    {p.name}
                    <small>
                      {p.sku} · {p.unit}
                    </small>
                  </span>
                </label>
              ))}
          </div>
          {result.data && (
            <Pagination page={search.page} total={result.data.total} onPage={search.setPage} />
          )}
        </>
      )}
      <div className="selected-chips">
        {ids.map((id) => (
          <button
            type="button"
            key={id}
            onClick={() => onChange(ids.filter((value) => value !== id))}
            aria-label={`Quitar ${names[id] ?? 'producto'}`}
          >
            {names[id] ?? 'Cargando…'} ×
          </button>
        ))}
      </div>
    </fieldset>
  );
}
