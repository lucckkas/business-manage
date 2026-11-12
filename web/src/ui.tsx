import {
  Children,
  cloneElement,
  isValidElement,
  useEffect,
  useId,
  useRef,
  useState,
  type ReactElement,
  type ReactNode,
} from 'react';
import { AlertCircle, ArrowLeft, ArrowRight, LoaderCircle, X, PackageOpen } from 'lucide-react';
import { api } from './api/client';

export function useResource<T>(path: string, revision = 0) {
  const [data, setData] = useState<T | null>(null),
    [error, setError] = useState(''),
    [loading, setLoading] = useState(true),
    [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    api<T>(path, 'GET', undefined, {}, controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) setData(value);
      })
      .catch((error) => {
        if (!controller.signal.aborted) setError(error.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [path, revision, retry]);
  return { data, error, loading, retry: () => setRetry((n) => n + 1) };
}
export function Field({
  label,
  children,
  hint,
}: {
  label: string;
  children: ReactNode;
  hint?: string;
}) {
  const id = useId();
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {Children.map(children, (child) =>
        isValidElement(child) &&
        (typeof child.type !== 'string' || ['input', 'select', 'textarea'].includes(child.type))
          ? cloneElement(child as ReactElement<{ id?: string; 'aria-describedby'?: string }>, {
              id,
              'aria-describedby': hint ? `${id}-hint` : undefined,
            })
          : child
      )}
      {hint && <small id={`${id}-hint`}>{hint}</small>}
    </div>
  );
}
export function ErrorMessage({ message }: { message: string }) {
  return message ? (
    <div className="error-message" role="alert">
      <AlertCircle size={18} />
      <span>{message}</span>
    </div>
  ) : null;
}
export function Loading() {
  return (
    <div className="loading" role="status">
      <LoaderCircle className="spin" size={20} /> Cargando información…
    </div>
  );
}
export function Empty({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty">
      <span className="empty-icon">
        <PackageOpen size={25} />
      </span>
      <h3>{title}</h3>
      <p>{description}</p>
      {action}
    </div>
  );
}
export function PageError({ message, retry }: { message: string; retry: () => void }) {
  return (
    <div className="page-error">
      <ErrorMessage message={message} />
      <button className="button secondary" onClick={retry}>
        Volver a intentar
      </button>
    </div>
  );
}
export function Pagination({
  page,
  total,
  size = 20,
  onPage,
}: {
  page: number;
  total: number;
  size?: number;
  onPage: (n: number) => void;
}) {
  const pages = Math.max(1, Math.ceil(total / size));
  return (
    <div className="pagination">
      <span>
        {total === 0
          ? 'Sin registros'
          : `${(page - 1) * size + 1}–${Math.min(page * size, total)} de ${total} registros`}
      </span>
      <div>
        <button
          className="icon-button"
          aria-label="Página anterior"
          disabled={page <= 1}
          onClick={() => onPage(page - 1)}
        >
          <ArrowLeft size={16} />
        </button>
        <span>
          {page} / {pages}
        </span>
        <button
          className="icon-button"
          aria-label="Página siguiente"
          disabled={page >= pages}
          onClick={() => onPage(page + 1)}
        >
          <ArrowRight size={16} />
        </button>
      </div>
    </div>
  );
}
export function Modal({
  title,
  children,
  onClose,
  busy = false,
  receipt = false,
  className = '',
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
  busy?: boolean;
  receipt?: boolean;
  className?: string;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current!;
    dialog.showModal();
    return () => dialog.close();
  }, []);
  return (
    <dialog
      ref={ref}
      className={`modal ${receipt ? 'receipt' : ''} ${className}`}
      aria-labelledby="dialog-title"
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onClose();
      }}
    >
      <div className="modal-header">
        <h2 id="dialog-title">{title}</h2>
        <button
          className="icon-button no-print"
          aria-label="Cerrar"
          disabled={busy}
          onClick={onClose}
        >
          <X size={20} />
        </button>
      </div>
      {children}
    </dialog>
  );
}
export function useSearch() {
  const [search, setSearch] = useState(''),
    [debounced, setDebounced] = useState(''),
    [page, setPage] = useState(1);
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebounced(search);
      setPage(1);
    }, 200);
    return () => clearTimeout(timer);
  }, [search]);
  return {
    search,
    setSearch,
    page,
    setPage,
    query: `search=${encodeURIComponent(debounced)}&page=${page}&pageSize=20`,
  };
}
