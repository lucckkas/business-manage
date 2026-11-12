import { useEffect, useState } from 'react';
import {
  ArrowUpRight,
  Banknote,
  CheckCircle2,
  ChevronRight,
  ClipboardList,
  LayoutDashboard,
  LogOut,
  Package,
  Settings2,
  ShieldCheck,
  ShoppingBag,
  Sprout,
  X,
  Tag,
} from 'lucide-react';
import { api, setSession, type Product, type Sale, type Schema, type Session } from './api/client';
import Auth from './Auth';
import { EditForm, Receipt, type FormKind } from './Forms';
import { Administration, Dashboard, Products, Sales } from './Pages';
import Audit from './Audit';
import Cash, { CashDetail } from './Cash';
import ProductDetails, { UnknownBarcode } from './ProductDetails';
import { PrintNotice } from './Printing';
import { resetScanner, useGlobalScanner } from './Scanner';
import NewSale from './NewSale';
import Promotions from './Promotions';
import Settings from './Settings';
import { Loading, PageError } from './ui';

const navigation = [
  { page: 'dashboard', label: 'Resumen', icon: LayoutDashboard },
  { page: 'sales', label: 'Ventas', icon: ShoppingBag },
  { page: 'products', label: 'Productos', icon: Package },
  { page: 'cash', label: 'Caja', icon: Banknote },
];
const labels: Record<string, string> = {
  dashboard: 'Resumen',
  sales: 'Ventas',
  products: 'Productos',
  cash: 'Caja',
  admin: 'Administración',
  audit: 'Auditoría',
  'new-sale': 'Nueva venta',
  promotions: 'Descuentos y ofertas',
  settings: 'Ajustes',
};
function route() {
  const key = location.hash.replace(/^#\/?/, '');
  if (key === 'inventory') {
    history.replaceState(history.state, '', `${location.pathname}${location.search}#/products`);
    return 'products';
  }
  return labels[key] ? key : 'dashboard';
}
const installationToken = location.hash.startsWith('#setup-token=') ? location.hash.slice(13) : '';
if (installationToken) history.replaceState(null, '', location.pathname + location.search);
export default function App() {
  const [phase, setPhase] = useState<'loading' | 'setup' | 'login' | 'ready' | 'error'>('loading'),
    [session, updateSession] = useState<Session | null>(null),
    [local, setLocal] = useState(false),
    [token] = useState(installationToken),
    [connectionError, setConnectionError] = useState(''),
    [retry, setRetry] = useState(0),
    [page, setPage] = useState(route),
    [revision, setRevision] = useState(0),
    [form, setForm] = useState<FormKind | null>(null),
    [receipt, setReceipt] = useState<Sale | null>(null),
    [productID, setProductID] = useState(''),
    [cashID, setCashID] = useState(''),
    [unknownBarcode, setUnknownBarcode] = useState(''),
    [printNotice, setPrintNotice] = useState<{ sale: Sale; id: string } | null>(null),
    [toast, setToast] = useState('');
  useGlobalScanner(phase === 'ready', () => {
    if (!['products', 'new-sale'].includes(page)) navigate('new-sale');
  });
  useEffect(() => {
    const failed = (event: Event) => setToast((event as CustomEvent<string>).detail);
    window.addEventListener('scanner-error', failed);
    return () => window.removeEventListener('scanner-error', failed);
  }, []);
  useEffect(() => {
    resetScanner();
    setProductID('');
    setCashID('');
    setUnknownBarcode('');
    setPrintNotice(null);
  }, [session?.user.id]);
  useEffect(() => {
    const controller = new AbortController();
    setPhase('loading');
    (async () => {
      try {
        const setup = await api<Schema['SetupStatus']>(
          '/setup',
          'GET',
          undefined,
          {},
          controller.signal
        );
        if (controller.signal.aborted) return;
        setLocal(setup.localSetupAllowed);
        if (!setup.initialized) {
          setPhase('setup');
          return;
        }
        try {
          const s = await api<Session>('/auth/me', 'GET', undefined, {}, controller.signal);
          if (!controller.signal.aborted) {
            setSession(s);
            updateSession(s);
            setPhase('ready');
          }
        } catch (error) {
          if (!controller.signal.aborted) {
            if ((error as { status?: number }).status === 401) setPhase('login');
            else throw error;
          }
        }
      } catch (error) {
        if (!controller.signal.aborted) {
          setConnectionError((error as Error).message);
          setPhase('error');
        }
      }
    })();
    return () => controller.abort();
  }, [retry]);
  useEffect(() => {
    function change() {
      setPage(route());
    }
    window.addEventListener('hashchange', change);
    return () => window.removeEventListener('hashchange', change);
  }, []);
  useEffect(() => {
    function expired() {
      setSession(null);
      updateSession(null);
      setForm(null);
      setReceipt(null);
      setPhase('login');
    }
    window.addEventListener('session-expired', expired);
    return () => window.removeEventListener('session-expired', expired);
  }, []);
  useEffect(() => {
    if (!toast) return;
    const timeout = setTimeout(() => setToast(''), 6500);
    return () => clearTimeout(timeout);
  }, [toast]);
  function navigate(next: string) {
    const target = next === 'inventory' ? 'products' : next;
    location.hash = `/${target}`;
    setPage(labels[target] ? target : 'dashboard');
  }
  function changed() {
    setRevision((n) => n + 1);
  }
  async function catalogScan(code: string) {
    try {
      const p = await api<Product>(`/products/lookup?barcode=${encodeURIComponent(code)}`);
      setProductID(p.id);
    } catch (e) {
      if ((e as { status?: number }).status === 404) setUnknownBarcode(code);
      else setToast((e as Error).message);
    }
  }
  function signedOut() {
    setSession(null);
    updateSession(null);
    setForm(null);
    setReceipt(null);
    setPhase('login');
  }
  async function logout() {
    try {
      await api('/auth/logout', 'POST');
      setSession(null);
      updateSession(null);
      setForm(null);
      setReceipt(null);
      setPhase('login');
    } catch (e) {
      setToast((e as Error).message);
    }
  }
  if (phase === 'loading')
    return (
      <div className="boot">
        <Sprout size={36} />
        <Loading />
      </div>
    );
  if (phase === 'error')
    return (
      <div className="boot">
        <Sprout size={36} />
        <PageError message={connectionError} retry={() => setRetry((n) => n + 1)} />
      </div>
    );
  if (phase === 'setup' || phase === 'login')
    return (
      <Auth
        key={phase}
        setup={phase === 'setup'}
        local={local}
        token={token}
        onSetup={() => {
          setPhase('login');
          setToast('Tu espacio está listo. Inicia sesión.');
        }}
        onLogin={(s) => {
          setSession(s);
          updateSession(s);
          setPhase('ready');
          changed();
        }}
      />
    );
  if (!session) return null;
  const admin = session.user.role === 'admin';
  const common = { session, revision };
  const navItem = (item: (typeof navigation)[number]) => (
    <a
      key={item.page}
      href={`#/${item.page}`}
      aria-label={item.label}
      title={item.label}
      className={
        page === item.page || (item.page === 'sales' && page === 'new-sale') ? 'active' : ''
      }
      aria-current={page === item.page ? 'page' : undefined}
    >
      <item.icon size={19} />
      <span>{item.label}</span>
      {page === item.page && <ChevronRight size={15} />}
    </a>
  );
  return (
    <div className="app-layout">
      <a
        className="skip-link"
        href="#main-content"
        onClick={(event) => {
          event.preventDefault();
          document.getElementById('main-content')?.focus();
        }}
      >
        Saltar al contenido
      </a>
      <aside className="sidebar">
        <a className="brand" href="#/dashboard" aria-label="Raíz, ir al resumen">
          <span className="brand-symbol">
            <Sprout size={25} />
          </span>
          <span>
            raíz<span className="brand-dot">.</span>
          </span>
        </a>
        <div className="workspace">
          <span className="workspace-avatar">{session.company.name.slice(0, 1).toUpperCase()}</span>
          <div>
            <strong>{session.company.name}</strong>
            <span>Mi espacio de trabajo</span>
          </div>
        </div>
        <nav aria-label="Navegación principal">
          <span className="nav-caption">GESTIÓN COMERCIAL</span>
          {navigation.map(navItem)}
          {navItem({ page: 'settings', label: 'Ajustes', icon: Settings2 })}
          {admin && (
            <>
              <span className="nav-caption nav-second">TU EMPRESA</span>
              {navItem({ page: 'admin', label: 'Administración', icon: Settings2 })}
              {navItem({ page: 'promotions', label: 'Descuentos y ofertas', icon: Tag })}
              {navItem({ page: 'audit', label: 'Auditoría', icon: ClipboardList })}
            </>
          )}
        </nav>
        <div className="sidebar-bottom">
          <div className="sidebar-local">
            <span className="status-dot" />
            <span>Servidor local</span>
            <ShieldCheck size={16} />
          </div>
          <div className="user-menu">
            <span className="user-avatar">{session.user.name.slice(0, 1).toUpperCase()}</span>
            <div>
              <strong>{session.user.name}</strong>
              <small>{admin ? 'Administrador' : 'Operador'}</small>
            </div>
            <button aria-label="Cerrar sesión" onClick={logout}>
              <LogOut size={17} />
            </button>
          </div>
        </div>
      </aside>
      <div className="app-body">
        <header className="topbar">
          <div>
            <span>Mi empresa</span>
            <ChevronRight size={14} />
            <strong>{labels[page]}</strong>
          </div>
          <div className="topbar-right">
            <span className="header-date">
              {new Intl.DateTimeFormat('es-CL', {
                timeZone: session.company.timezone,
                day: 'numeric',
                month: 'long',
                year: 'numeric',
              }).format(new Date())}
            </span>
            <span className="header-local">
              <span className="status-dot" /> En tu red
            </span>
          </div>
        </header>
        <main id="main-content" className="main-content" tabIndex={-1}>
          {printNotice && (
            <PrintNotice
              id={printNotice.id}
              saleNumber={printNotice.sale.number}
              onView={() => setReceipt(printNotice.sale)}
              onClose={() => setPrintNotice(null)}
            />
          )}
          {page === 'dashboard' && (
            <Dashboard
              {...common}
              navigate={navigate}
              onProduct={() => setForm({ type: 'product' })}
              onReceipt={setReceipt}
            />
          )}
          {page === 'products' && (
            <Products
              {...common}
              onProduct={() => setForm({ type: 'product' })}
              onStock={(product: Product) => setForm({ type: 'stock', product })}
              onEdit={(product: Product) => setForm({ type: 'product', product })}
              onAdjust={(product: Product) => setForm({ type: 'adjust', product })}
              onView={setProductID}
              onScan={catalogScan}
            />
          )}
          {page === 'sales' && <Sales {...common} navigate={navigate} onReceipt={setReceipt} />}
          {page === 'new-sale' && (
            <NewSale
              {...common}
              navigate={navigate}
              onReceipt={(sale) => {
                if (sale.printJobId) setPrintNotice({ sale, id: sale.printJobId });
                else setReceipt(sale);
              }}
              onChange={changed}
              onUnknown={setUnknownBarcode}
            />
          )}
          {page === 'cash' && (
            <Cash
              key={session.user.id}
              {...common}
              onChange={changed}
              onReceipt={setReceipt}
              onMovement={setCashID}
            />
          )}
          {page === 'admin' && admin && (
            <Administration
              {...common}
              onUser={() => setForm({ type: 'user' })}
              onChange={changed}
              notify={setToast}
            />
          )}
          {page === 'audit' && admin && (
            <Audit {...common} onReceipt={setReceipt} onProduct={setProductID} onCash={setCashID} />
          )}
          {page === 'promotions' && admin && (
            <Promotions {...common} onChange={changed} notify={setToast} />
          )}
          {page === 'settings' && (
            <Settings
              session={session}
              onSession={(s) => {
                setSession(s);
                updateSession(s);
                changed();
              }}
              onSignedOut={signedOut}
              onLogout={logout}
              notify={setToast}
            />
          )}
          {(page === 'admin' || page === 'audit' || page === 'promotions') && !admin && (
            <PageError
              message="Esta sección está disponible para administradores."
              retry={() => navigate('dashboard')}
            />
          )}
          <footer className="main-footer">
            <span>
              raíz. <span>Un lugar para hacer crecer tu negocio.</span>
            </span>
            <span>
              Gestión local <ArrowUpRight size={12} />
            </span>
          </footer>
        </main>
      </div>
      {cashID && <CashDetail id={cashID} session={session} onClose={() => setCashID('')} />}
      {form && (
        <EditForm
          kind={form}
          session={session}
          onClose={() => setForm(null)}
          onSuccess={(message) => {
            changed();
            setToast(message);
          }}
        />
      )}
      {receipt && (
        <Receipt
          sale={receipt}
          session={session}
          onClose={() => setReceipt(null)}
          onQueued={(job) => setPrintNotice({ sale: receipt, id: job.id })}
        />
      )}
      {productID && (
        <ProductDetails
          key={productID}
          id={productID}
          session={session}
          onClose={() => setProductID('')}
          onChange={changed}
          onEdit={(p) => {
            setProductID('');
            setCashID('');
            setForm({ type: 'product', product: p });
          }}
          onStock={(p) => {
            setProductID('');
            setCashID('');
            setForm({ type: 'stock', product: p });
          }}
          onAdjust={(p) => {
            setProductID('');
            setCashID('');
            setForm({ type: 'adjust', product: p });
          }}
        />
      )}
      {unknownBarcode && (
        <UnknownBarcode
          key={unknownBarcode}
          code={unknownBarcode}
          admin={admin}
          onClose={() => setUnknownBarcode('')}
          onCreate={() => {
            setForm({ type: 'product', barcode: unknownBarcode });
            setUnknownBarcode('');
          }}
          onLinked={() => {
            changed();
            setToast('Código vinculado. Vuelve a escanear para continuar.');
          }}
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <CheckCircle2 size={18} />
          <span>{toast}</span>
          <button aria-label="Cerrar aviso" onClick={() => setToast('')}>
            <X size={17} />
          </button>
        </div>
      )}
    </div>
  );
}
