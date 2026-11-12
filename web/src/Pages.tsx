import { useEffect, useState, type ReactNode } from 'react';
import ScanField, { useScanReceiver } from './Scanner';
import {
  ArrowDownToLine,
  ArrowRight,
  ArrowUpRight,
  Banknote,
  CircleDollarSign,
  FileText,
  Package,
  Plus,
  Search,
  ShieldCheck,
  ShoppingBag,
  Users,
} from 'lucide-react';
import {
  api,
  money,
  date,
  quantity,
  stockLabel,
  type Page,
  type Product,
  type Schema,
  type Session,
  type Sale,
  type User,
} from './api/client';
import { Empty, ErrorMessage, Loading, PageError, Pagination, useResource, useSearch } from './ui';

type Common = { session: Session; revision: number };
export function Heading({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow: string;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {action}
    </div>
  );
}
export function SearchBox({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (s: string) => void;
  placeholder: string;
}) {
  return (
    <label className="search-box">
      <Search size={17} />
      <input
        aria-label={placeholder}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
      />
    </label>
  );
}
function SaleRows({
  items,
  session,
  onReceipt,
}: {
  items: Sale[];
  session: Session;
  onReceipt: (s: Sale) => void;
}) {
  const [error, setError] = useState(''),
    [busy, setBusy] = useState('');
  async function open(id: string) {
    setBusy(id);
    setError('');
    try {
      onReceipt(await api<Sale>(`/sales/${id}`));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  return (
    <>
      <ErrorMessage message={error} />
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Comprobante</th>
              <th>Fecha</th>
              <th>Estado</th>
              <th className="numeric">Total</th>
              <th>
                <span className="sr-only">Acciones</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {items.map((s) => (
              <tr key={s.id}>
                <td>
                  <span className="table-document">
                    <FileText size={16} />
                    <strong>Venta #{String(s.number).padStart(5, '0')}</strong>
                  </span>
                </td>
                <td className="muted">{date(s.createdAt, session.company.timezone)}</td>
                <td>
                  <span className="badge green">
                    <span /> Pagada
                  </span>
                </td>
                <td className="numeric strong">{money(s.totalMinor, session.company)}</td>
                <td className="numeric">
                  <button
                    className="text-button"
                    disabled={busy === s.id}
                    onClick={() => open(s.id)}
                    aria-label={`Ver venta ${s.number}`}
                  >
                    {busy === s.id ? 'Cargando…' : 'Ver'}
                    <ArrowUpRight size={15} />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

export function Dashboard({
  session,
  revision,
  navigate,
  onProduct,
  onReceipt,
}: Common & {
  navigate: (page: string) => void;
  onProduct: () => void;
  onReceipt: (s: Sale) => void;
}) {
  const summary = useResource<Schema['Summary']>('/reports/summary', revision),
    sales = useResource<Schema['SalePage']>('/sales?pageSize=5', revision);
  if (summary.loading && !summary.data) return <Loading />;
  if (summary.error) return <PageError message={summary.error} retry={summary.retry} />;
  const s = summary.data!;
  const admin = session.user.role === 'admin';
  const stats = [
    {
      title: 'Ventas de hoy',
      value: money(s.todayRevenueMinor, session.company),
      detail: `${s.todaySales} ${s.todaySales === 1 ? 'venta registrada' : 'ventas registradas'}`,
      icon: CircleDollarSign,
      color: 'sage',
    },
    {
      title: 'Saldo de caja',
      value: money(s.cashMinor, session.company),
      detail: 'Ingresos menos egresos registrados',
      icon: Banknote,
      color: 'sand',
    },
    {
      title: 'Productos en catálogo',
      value: s.products.toString(),
      detail: 'Tu oferta, en un solo lugar',
      icon: Package,
      color: 'blue',
    },
    {
      title: 'Productos sin stock',
      value: s.outOfStock.toString(),
      detail: s.outOfStock ? 'Es momento de reponer' : 'Inventario al día',
      icon: ShoppingBag,
      color: 'peach',
    },
  ];
  return (
    <>
      <div className="dashboard-heading">
        <div>
          <span className="eyebrow">EL PULSO DE TU EMPRESA</span>
          <h1>
            Tu negocio, en orden<span>.</span>
          </h1>
          <p>Hola, {session.user.name.split(' ')[0]}. Aquí tienes una mirada a lo que importa.</p>
        </div>
        <button
          className="button primary"
          onClick={() =>
            s.products ? navigate('new-sale') : admin ? onProduct() : navigate('products')
          }
        >
          <Plus size={18} />
          {s.products ? 'Nueva venta' : admin ? 'Nuevo producto' : 'Ver catálogo'}
        </button>
      </div>
      <div className="stats-grid">
        {stats.map(({ title, value, detail, icon: Icon, color }) => (
          <section className="stat-card" key={title}>
            <div className="stat-top">
              <span>{title}</span>
              <span className={`stat-icon ${color}`}>
                <Icon size={18} />
              </span>
            </div>
            <strong>{value}</strong>
            <small>{detail}</small>
          </section>
        ))}
      </div>
      {s.products === 0 && (
        <section className="onboarding">
          <div className="onboarding-mark">
            <Package size={26} />
          </div>
          <div>
            <span className="eyebrow">UN BUEN COMIENZO</span>
            <h2>Tu primera venta comienza con un producto.</h2>
            <p>Agrega tu catálogo, ingresa existencias y empieza a vender.</p>
          </div>
          {admin && (
            <button className="button secondary" onClick={onProduct}>
              Agregar mi primer producto <ArrowRight size={16} />
            </button>
          )}
        </section>
      )}
      <div className="dashboard-columns">
        <section className="panel recent-sales">
          <div className="panel-heading">
            <div>
              <h2>Últimas ventas</h2>
              <p>Los movimientos más recientes de tu negocio.</p>
            </div>
            <button className="text-button" onClick={() => navigate('sales')}>
              Ver todas <ArrowRight size={16} />
            </button>
          </div>
          {sales.loading ? (
            <Loading />
          ) : sales.error ? (
            <PageError message={sales.error} retry={sales.retry} />
          ) : sales.data?.items.length ? (
            <SaleRows items={sales.data.items} session={session} onReceipt={onReceipt} />
          ) : (
            <Empty
              title="Todo listo para tu primera venta"
              description="Las ventas que registres aparecerán aquí."
            />
          )}
          <div className="panel-foot">
            <span className="status-dot" /> Información actualizada desde tu servidor local
          </div>
        </section>
        <aside className="dashboard-aside">
          <section className="quick-panel">
            <span className="eyebrow">A UN PASO</span>
            <h2>Un día más simple.</h2>
            <button onClick={() => navigate('new-sale')}>
              <span className="quick-icon">
                <Plus size={20} />
              </span>
              <span>
                <strong>Registrar una venta</strong>
                <small>Productos, total y comprobante</small>
              </span>
              <ArrowUpRight size={18} />
            </button>
            {admin && (
              <button onClick={() => navigate('products')}>
                <span className="quick-icon">
                  <ArrowDownToLine size={20} />
                </span>
                <span>
                  <strong>Ingresar existencias</strong>
                  <small>Mantén tu inventario al día</small>
                </span>
                <ArrowUpRight size={18} />
              </button>
            )}
            <button onClick={() => navigate('cash')}>
              <span className="quick-icon">
                <Banknote size={20} />
              </span>
              <span>
                <strong>Revisar caja</strong>
                <small>Cada ingreso, en su lugar</small>
              </span>
              <ArrowUpRight size={18} />
            </button>
          </section>
          <section className="privacy-card">
            <ShieldCheck size={27} />
            <h3>
              Cerca de ti.
              <br />
              Bajo tu control.
            </h3>
            <p>Tu información se guarda en tu empresa y está disponible en tu red local.</p>
            <span>
              <span className="status-dot" /> Operación local
            </span>
          </section>
        </aside>
      </div>
    </>
  );
}

export function Products({
  session,
  revision,
  onProduct,
  onStock,
  onEdit,
  onAdjust,
  onView,
  onScan,
}: Common & {
  onProduct: () => void;
  onStock: (p: Product) => void;
  onEdit: (p: Product) => void;
  onAdjust: (p: Product) => void;
  onView: (id: string) => void;
  onScan: (code: string) => Promise<void>;
}) {
  useScanReceiver(onScan);
  const search = useSearch();
  const result = useResource<Schema['ProductPage']>(`/products?${search.query}`, revision);
  const admin = session.user.role === 'admin';
  return (
    <>
      <Heading
        eyebrow="TU OFERTA, ORGANIZADA"
        title="Productos"
        description="Administra tu catálogo y sus existencias"
        action={
          admin && (
            <button className="button primary" onClick={onProduct}>
              <Plus size={17} />
              Nuevo producto
            </button>
          )
        }
      />
      <section className="panel">
        <ScanField label="Escanear en productos" />
        <div className="table-toolbar">
          <SearchBox
            value={search.search}
            onChange={search.setSearch}
            placeholder="Buscar por nombre o código"
          />
          <span className="toolbar-note">{result.data?.total ?? 0} productos</span>
        </div>
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <PageError message={result.error} retry={result.retry} />
        ) : !result.data?.items.length ? (
          <Empty
            title={search.search ? 'No encontramos coincidencias' : 'Tu catálogo empieza aquí'}
            description={
              search.search
                ? 'Prueba con otro nombre o código.'
                : 'Agrega tu primer producto y dale forma a tu inventario.'
            }
            action={
              admin &&
              !search.search && (
                <button className="button secondary" onClick={onProduct}>
                  <Plus size={16} />
                  Agregar producto
                </button>
              )
            }
          />
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th scope="col">Producto</th>
                  <th scope="col">Código</th>
                  <th scope="col">Unidad</th>
                  <th scope="col" className="numeric">
                    Precio de venta
                  </th>
                  <th scope="col" className="numeric">
                    Existencias
                  </th>
                  {admin && (
                    <th scope="col">
                      <span className="sr-only">Acciones</span>
                    </th>
                  )}
                </tr>
              </thead>
              <tbody>
                {result.data.items.map((p) => (
                  <tr key={p.id}>
                    <td>
                      <div className="product-cell">
                        <span className="product-avatar">{p.name.slice(0, 2).toUpperCase()}</span>
                        <div>
                          <button
                            className="product-name-button"
                            onClick={() => onView(p.id)}
                            aria-label={`Ver producto ${p.name}`}
                          >
                            <strong>{p.name}</strong>
                          </button>
                          {!p.sellable && (
                            <small className="block-muted">Insumo · Uso interno</small>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="muted monospace">{p.sku}</td>
                    <td className="muted">{p.unit}</td>
                    <td className="numeric">{money(p.priceMinor, session.company)}</td>
                    <td className="numeric">
                      <span
                        className={`badge ${p.stock?.startsWith('-') || p.availableQuantity === '0.000' ? 'orange' : 'neutral'}`}
                      >
                        {stockLabel(p)}
                      </span>
                    </td>
                    {admin && (
                      <td className="numeric">
                        <div className="row-actions">
                          <button
                            className="text-button"
                            onClick={() => onEdit(p)}
                            aria-label={`Editar ${p.name}`}
                          >
                            Editar
                          </button>
                          {p.stockMode === 'own' && p.stockPolicy !== 'untracked' && (
                            <>
                              <button
                                className="text-button"
                                onClick={() => onStock(p)}
                                aria-label={`Ingresar existencias de ${p.name}`}
                              >
                                <Plus size={15} />
                                Ingresar
                              </button>
                              <button
                                className="text-button"
                                onClick={() => onAdjust(p)}
                                aria-label={`Ajustar existencias de ${p.name}`}
                              >
                                Ajustar
                              </button>
                            </>
                          )}
                        </div>
                      </td>
                    )}
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
    </>
  );
}
export function Sales({
  session,
  revision,
  onReceipt,
  navigate,
}: Common & { onReceipt: (s: Sale) => void; navigate: (p: string) => void }) {
  const search = useSearch(),
    result = useResource<Schema['SalePage']>(`/sales?${search.query}`, revision);
  return (
    <>
      <Heading
        eyebrow="CADA VENTA, CONECTADA"
        title="Ventas"
        description="El historial comercial de tu empresa, siempre a mano."
        action={
          <button className="button primary" onClick={() => navigate('new-sale')}>
            <Plus size={17} />
            Nueva venta
          </button>
        }
      />
      <section className="panel">
        <div className="table-toolbar">
          <SearchBox
            value={search.search}
            onChange={search.setSearch}
            placeholder="Buscar por número de venta"
          />
          <span className="toolbar-note">Ventas al contado</span>
        </div>
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <PageError message={result.error} retry={result.retry} />
        ) : result.data?.items.length ? (
          <SaleRows items={result.data.items} session={session} onReceipt={onReceipt} />
        ) : (
          <Empty
            title="Tu próxima venta empieza aquí"
            description="Registra una venta y consulta su comprobante cuando lo necesites."
          />
        )}
        {result.data && (
          <Pagination page={search.page} total={result.data.total} onPage={search.setPage} />
        )}
      </section>
    </>
  );
}
export function Administration({
  session,
  revision,
  onUser,
  onChange,
  notify,
}: Common & { onUser: () => void; onChange: () => void; notify: (s: string) => void }) {
  const search = useSearch(),
    users = useResource<Schema['UserPage']>(`/users?${search.query}`, revision),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(''),
    [permissionOverrides, setPermissionOverrides] = useState<Record<string, boolean>>({});
  useEffect(() => setPermissionOverrides({}), [users.data]);
  async function toggle(user: User) {
    setBusy(user.id);
    setError('');
    try {
      await api(`/users/${user.id}`, 'PATCH', { active: !user.active });
      onChange();
      notify(user.active ? 'Acceso desactivado y sesiones cerradas.' : 'Acceso activado.');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  async function backup() {
    setBusy('backup');
    setError('');
    try {
      const result = await api<Schema['Backup']>('/system/backups', 'POST');
      notify(`Respaldo guardado en el servidor: ${result.filename}`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  async function permission(
    user: User,
    field: 'canApplyManualDiscounts' | 'canRecordCashExpenses'
  ) {
    setBusy(user.id);
    setError('');
    const key = `${user.id}:${field}`;
    const allowed = !(permissionOverrides[key] ?? user[field]);
    setPermissionOverrides((current) => ({ ...current, [key]: allowed }));
    try {
      await api(`/users/${user.id}`, 'PATCH', {
        [field]: allowed,
      });
      onChange();
      notify('Permiso actualizado y registrado en auditoría.');
    } catch (e) {
      setPermissionOverrides((current) => {
        const next = { ...current };
        delete next[key];
        return next;
      });
      setError((e as Error).message);
    } finally {
      setBusy('');
    }
  }
  return (
    <>
      <Heading
        eyebrow="UN ESPACIO BIEN CUIDADO"
        title="Administración"
        description="Las personas y la información que dan vida a tu empresa."
        action={
          <button className="button primary" onClick={onUser}>
            <Plus size={17} />
            Nuevo usuario
          </button>
        }
      />
      <ErrorMessage message={error} />
      <div className="settings-grid">
        <section className="panel company-panel">
          <span className="settings-icon">
            <Package size={21} />
          </span>
          <div>
            <span className="eyebrow">TU EMPRESA</span>
            <h2>{session.company.name}</h2>
            <p>
              {session.company.currency} · {session.company.timezone}
            </p>
          </div>
        </section>
        <section className="panel backup-panel">
          <span className="settings-icon">
            <ShieldCheck size={21} />
          </span>
          <div>
            <h3>Una copia para estar tranquilo.</h3>
            <p>Se genera una copia diaria. Conservamos las últimas siete.</p>
          </div>
          <button className="button secondary" disabled={busy === 'backup'} onClick={backup}>
            <ArrowDownToLine size={16} />
            {busy === 'backup' ? 'Respaldando…' : 'Crear respaldo'}
          </button>
        </section>
      </div>
      <section className="panel">
        <div className="panel-heading">
          <div>
            <h2>Personas y accesos</h2>
            <p>Cada persona tiene su propio usuario.</p>
          </div>
          <Users size={20} />
        </div>
        <div className="table-toolbar">
          <SearchBox
            value={search.search}
            onChange={search.setSearch}
            placeholder="Buscar usuario"
          />
        </div>
        {users.loading ? (
          <Loading />
        ) : users.error ? (
          <PageError message={users.error} retry={users.retry} />
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Persona</th>
                  <th>Usuario</th>
                  <th>Rol</th>
                  <th>Estado</th>
                  <th>Descuentos manuales</th>
                  <th>Egresos de caja</th>
                  <th>
                    <span className="sr-only">Acciones</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {users.data?.items.map((u) => (
                  <tr key={u.id}>
                    <td>
                      <strong>{u.name}</strong>
                      {u.id === session.user.id && <small className="inline-muted"> Tú</small>}
                    </td>
                    <td className="muted">{u.username}</td>
                    <td>{u.role === 'admin' ? 'Administrador' : 'Operador'}</td>
                    <td>
                      <span className={`badge ${u.active ? 'green' : 'neutral'}`}>
                        {u.active ? 'Activo' : 'Inactivo'}
                      </span>
                    </td>
                    <td className="numeric">
                      <input
                        type="checkbox"
                        aria-label={`Permitir descuentos a ${u.name}`}
                        checked={
                          permissionOverrides[`${u.id}:canApplyManualDiscounts`] ??
                          u.canApplyManualDiscounts
                        }
                        disabled={u.role === 'admin' || busy === u.id}
                        onChange={() => permission(u, 'canApplyManualDiscounts')}
                      />
                    </td>
                    <td className="numeric">
                      <input
                        type="checkbox"
                        aria-label={`Permitir egresos a ${u.name}`}
                        checked={
                          permissionOverrides[`${u.id}:canRecordCashExpenses`] ??
                          u.canRecordCashExpenses
                        }
                        disabled={u.role === 'admin' || busy === u.id}
                        onChange={() => permission(u, 'canRecordCashExpenses')}
                      />
                    </td>
                    <td className="numeric">
                      {u.id !== session.user.id && (
                        <button
                          className="text-button"
                          disabled={busy === u.id}
                          onClick={() => toggle(u)}
                        >
                          {u.active ? 'Desactivar' : 'Activar'}
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {users.data && (
          <Pagination page={search.page} total={users.data.total} onPage={search.setPage} />
        )}
      </section>
      <p className="admin-contract">
        <a href="/api/v1/openapi.json" target="_blank" rel="noreferrer">
          Descargar contrato de integración (OpenAPI)
        </a>
      </p>
    </>
  );
}
