import type { components } from './schema';

export type Schema = components['schemas'];
export type Session = Schema['Session'];
export type Product = Schema['ProductView'];
export type Sale = Schema['Sale'];
export type User = Schema['User'];
export type Page<T> = { items: T[]; total: number; page: number; pageSize: number };
let csrfToken = '';
export function setSession(session: Session | null) {
  csrfToken = session?.csrfToken ?? '';
}
export class ApiError extends Error {
  constructor(
    message: string,
    public status = 0,
    public code = 'network'
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  method = 'GET',
  body?: unknown,
  headers: Record<string, string> = {},
  signal?: AbortSignal
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api/v1${path}`, {
      method,
      credentials: 'same-origin',
      cache: 'no-store',
      signal,
      headers: {
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
        ...(method !== 'GET' && csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
        ...headers,
      },
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error;
    throw new ApiError(
      'No pudimos conectar con el servidor. Comprueba la red local e inténtalo nuevamente.'
    );
  }
  if (!response.ok) {
    const data = (await response.json().catch(() => null)) as Schema['Error'] | null;
    if (response.status === 401 && path !== '/auth/login' && path !== '/auth/me')
      window.dispatchEvent(new Event('session-expired'));
    throw new ApiError(
      data?.error?.message ?? 'No fue posible completar la operación.',
      response.status,
      data?.error?.code
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
export function money(minor: string, company: Schema['Company']): string {
  const signed = BigInt(minor);
  const n = signed < 0n ? -signed : signed;
  const factor = 10n ** BigInt(company.currencyScale);
  const whole = new Intl.NumberFormat('es-CL', { maximumFractionDigits: 0 }).format(n / factor);
  const fraction = company.currencyScale
    ? `,${(n % factor).toString().padStart(company.currencyScale, '0')}`
    : '';
  return `${company.currency === 'CLP' ? '$' : company.currency} ${signed < 0n ? '−' : ''}${whole}${fraction}`;
}
export function priceInput(value: string, scale: number): string {
  const pattern = scale ? new RegExp(`^\\d+(?:[.,]\\d{1,${scale}})?$`) : /^\d+$/;
  if (!pattern.test(value))
    throw new Error(
      `Ingresa un precio válido, sin separador de miles${scale ? ` y con hasta ${scale} decimales` : ''}.`
    );
  const [whole, fraction = ''] = value.replace(',', '.').split('.');
  return (
    BigInt(whole) * 10n ** BigInt(scale) +
    BigInt(fraction.padEnd(scale, '0') || '0')
  ).toString();
}
export function quantityMilli(value: string): bigint {
  if (!/^\d+(?:\.\d{1,3})?$/.test(value))
    throw new Error('Ingresa una cantidad con hasta tres decimales.');
  const [whole, fraction = ''] = value.split('.');
  return BigInt(whole) * 1000n + BigInt(fraction.padEnd(3, '0'));
}
export function quantity(value: string | null | undefined): string {
  if (value == null) return 'Sin seguimiento';
  return value
    .replace(/(\.\d*?)0+$/, '$1')
    .replace(/\.$/, '')
    .replace('.', ',');
}
export function stockLabel(p: Product) {
  if (p.stockMode === 'components')
    return p.availableQuantity == null
      ? 'Según insumos'
      : `${quantity(p.availableQuantity)} ${p.unit} por insumos`;
  return p.stock == null ? 'Sin seguimiento' : `${quantity(p.stock)} ${p.unit}`;
}
export function lineTotal(price: string, count: string): bigint {
  try {
    return (BigInt(price) * quantityMilli(count) + 500n) / 1000n;
  } catch {
    return 0n;
  }
}
export function date(value: string, timezone: string, time = true): string {
  return new Intl.DateTimeFormat('es-CL', {
    timeZone: timezone,
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    ...(time ? { hour: '2-digit', minute: '2-digit' } : {}),
  }).format(new Date(value));
}
