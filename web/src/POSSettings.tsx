import { useEffect, useState, type FormEvent } from 'react';
import { Printer } from 'lucide-react';
import { api, date, type Schema, type Session } from './api/client';
import { ErrorMessage, Field, Loading, useResource } from './ui';
import NumericInput from './NumericInput';
import { jobKinds, jobLabels, usePrintJob } from './Printing';
type Config = Schema['POSSettings'];
export default function POSSettings({ session }: { session: Session }) {
  const resource = useResource<Config>('/pos/settings'),
    devices = useResource<Schema['POSDevices']>('/pos/devices');
  const [config, setConfig] = useState<Config | null>(null),
    [saved, setSaved] = useState<Config | null>(null),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [revision, setRevision] = useState(0),
    [testId, setTestId] = useState(''),
    [reason, setReason] = useState(''),
    [manualId, setManualId] = useState('');
  const test = usePrintJob(testId),
    manual = usePrintJob(manualId),
    jobs = useResource<Schema['PrintJobPage']>('/pos/jobs?pageSize=20', revision);
  useEffect(() => {
    if (resource.data) {
      setConfig(resource.data);
      setSaved(resource.data);
    }
  }, [resource.data]);
  useEffect(() => {
    if (test.job && !['pending', 'sending'].includes(test.job.state)) setRevision((n) => n + 1);
  }, [test.job]);
  useEffect(() => {
    if (manual.job && !['pending', 'sending'].includes(manual.job.state)) setRevision((n) => n + 1);
  }, [manual.job]);
  const storageKey = `raiz-pos-requests:${session.user.id}`;
  const [requestKeys] = useState(() => {
    try {
      return new Map<string, string>(JSON.parse(sessionStorage.getItem(storageKey) ?? '[]'));
    } catch {
      return new Map<string, string>();
    }
  });
  function change(next: Config, physical = false) {
    setConfig(
      physical
        ? {
            ...next,
            enabled: false,
            drawerEnabled: false,
            printVerified: false,
            drawerVerified: false,
          }
        : next
    );
    if (physical) setTestId('');
  }
  async function save(event: FormEvent) {
    event.preventDefault();
    if (!config) return;
    setBusy(true);
    setError('');
    try {
      const cfg = await api<Config>('/pos/settings', 'PUT', config);
      setConfig(cfg);
      setSaved(cfg);
      setTestId('');
      setRevision((n) => n + 1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function request(kind: 'test_print' | 'test_drawer' | 'drawer') {
    setBusy(true);
    setError('');
    const semantic = JSON.stringify([kind, kind === 'drawer' ? reason : '', config?.version]);
    const key = requestKeys.get(semantic) ?? crypto.randomUUID();
    requestKeys.set(semantic, key);
    sessionStorage.setItem(storageKey, JSON.stringify([...requestKeys]));
    try {
      const job = await api<Schema['PrintJob']>(
        kind === 'drawer' ? '/pos/drawer/open' : '/pos/tests',
        'POST',
        { kind, reason: kind === 'drawer' ? reason : '' },
        { 'Idempotency-Key': key }
      );
      if (kind === 'drawer') setManualId(job.id);
      else setTestId(job.id);
      requestKeys.delete(semantic);
      sessionStorage.setItem(storageKey, JSON.stringify([...requestKeys]));
      setRevision((n) => n + 1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function verify() {
    setBusy(true);
    setError('');
    try {
      const cfg = await api<Config>('/pos/verify', 'POST', { jobId: testId });
      setConfig(cfg);
      setSaved(cfg);
      setTestId('');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  if (resource.loading) return <Loading />;
  if (resource.error) return <ErrorMessage message={resource.error} />;
  if (!config) return null;
  const dirty = JSON.stringify(config) !== JSON.stringify(saved);
  const testPending = !!testId && (!test.job || ['pending', 'sending'].includes(test.job.state));
  const manualPending =
    !!manualId && (!manual.job || ['pending', 'sending'].includes(manual.job.state));
  return (
    <section className="panel pos-settings">
      <div className="panel-heading">
        <h2>
          <Printer size={19} />
          Periféricos de caja
        </h2>
      </div>
      <ErrorMessage message={error || devices.error || test.error || manual.error} />
      <p className="form-note">
        La Xprinter debe estar conectada por USB al equipo servidor. El cajón se conecta a la
        impresora. En el controlador, desactiva la apertura automática: el ERP enviará el pulso
        cuando corresponda.
      </p>
      <form onSubmit={save}>
        <Field label="Impresora del servidor">
          <select
            value={config.deviceId}
            onChange={(e) =>
              change(
                {
                  ...config,
                  deviceId: e.target.value,
                  deviceName: devices.data?.find((d) => d.id === e.target.value)?.name ?? '',
                },
                true
              )
            }
          >
            <option value="">Selecciona una impresora</option>
            {config.deviceId && !devices.data?.some((d) => d.id === config.deviceId) && (
              <option value={config.deviceId}>{config.deviceName} · No detectada</option>
            )}
            {devices.data?.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </select>
        </Field>
        <button type="button" className="text-button" onClick={devices.retry}>
          Actualizar impresoras
        </button>
        {!devices.loading && !devices.data?.length && (
          <p className="form-note">
            No se detectaron impresoras. Instala el controlador de Xprinter en Windows; en Linux
            comprueba el dispositivo USB y los permisos del servicio.
          </p>
        )}
        <div className="form-grid">
          <Field label="Ancho del papel">
            <select
              value={config.width}
              onChange={(e) =>
                change({ ...config, width: Number(e.target.value) as Config['width'] }, true)
              }
            >
              <option value={58}>58 mm</option>
              <option value={80}>80 mm</option>
            </select>
          </Field>
          <Field label="Conector del cajón">
            <select
              value={config.drawerPin}
              onChange={(e) =>
                change(
                  { ...config, drawerPin: Number(e.target.value) as Config['drawerPin'] },
                  true
                )
              }
            >
              <option value={0}>Pin 2</option>
              <option value={1}>Pin 5</option>
            </select>
          </Field>
        </div>
        <label className="checkbox-field">
          <input
            type="checkbox"
            checked={config.cutter}
            onChange={(e) => change({ ...config, cutter: e.target.checked }, true)}
          />
          Corte automático disponible en este modelo
        </label>
        <div className="form-grid">
          <Field label="Pulso de apertura (ms)">
            <NumericInput
              label="Pulso"
              precision={0}
              minimum="2"
              maximum="510"
              value={String(config.pulseOnMs)}
              onValueChange={(v) => change({ ...config, pulseOnMs: Number(v) }, true)}
            />
          </Field>
          <Field label="Reposo del pulso (ms)" hint="Tiempos pares entre 2 y 510 ms.">
            <NumericInput
              label="Reposo"
              precision={0}
              minimum="2"
              maximum="510"
              value={String(config.pulseOffMs)}
              onValueChange={(v) => change({ ...config, pulseOffMs: Number(v) }, true)}
            />
          </Field>
        </div>
        <label className="checkbox-field">
          <input
            type="checkbox"
            checked={config.enabled}
            disabled={!config.printVerified}
            onChange={(e) =>
              change({
                ...config,
                enabled: e.target.checked,
                drawerEnabled: e.target.checked && config.drawerEnabled,
              })
            }
          />
          Imprimir automáticamente cada venta nueva
        </label>
        <label className="checkbox-field">
          <input
            type="checkbox"
            checked={config.drawerEnabled}
            disabled={!config.drawerVerified || !config.enabled}
            onChange={(e) => change({ ...config, drawerEnabled: e.target.checked })}
          />
          Abrir el cajón al confirmar ventas y registrar egresos
        </label>
        <button className="button primary" disabled={busy}>
          Guardar periféricos
        </button>
      </form>
      <section className="pos-tests">
        <h3>Verificar el equipo</h3>
        <p className="form-note">
          Guarda primero el perfil. Confirma cada prueba solo después de comprobar físicamente el
          resultado.
        </p>
        <div className="row-actions">
          <button
            className="button secondary"
            disabled={busy || dirty || testPending || !config.deviceId}
            onClick={() => request('test_print')}
          >
            Imprimir ticket de prueba
          </button>
          <button
            className="button secondary"
            disabled={busy || dirty || testPending || !config.printVerified}
            onClick={() => request('test_drawer')}
          >
            Probar apertura del cajón
          </button>
        </div>
        <p className="form-note">
          Ticket: {config.printVerified ? 'Verificado' : 'Sin verificar'} · Cajón:{' '}
          {config.drawerVerified ? 'Verificado' : 'Sin verificar'}
        </p>
        {test.job && (
          <div className="test-result">
            <strong>
              {jobKinds[test.job.kind]} · {jobLabels[test.job.state]}
            </strong>
            <p>{test.job.error || 'Comprueba el resultado en el equipo.'}</p>
            {test.job.state === 'sent' && (
              <button className="button secondary" disabled={busy} onClick={verify}>
                {test.job.kind === 'test_drawer'
                  ? 'Confirmar que el cajón abrió correctamente'
                  : 'Confirmar que el ticket salió correctamente'}
              </button>
            )}
          </div>
        )}
      </section>
      {config.drawerVerified && (
        <section className="manual-drawer">
          <h3>Apertura manual del cajón</h3>
          <Field label="Motivo de apertura">
            <input
              value={reason}
              minLength={3}
              maxLength={240}
              onChange={(e) => setReason(e.target.value)}
            />
          </Field>
          <button
            className="button secondary"
            disabled={busy || dirty || manualPending || reason.trim().length < 3}
            onClick={() => request('drawer')}
          >
            Abrir cajón
          </button>
          {manual.job && (
            <p role="status">
              {jobLabels[manual.job.state]} · {manual.job.error || manual.job.reason}
            </p>
          )}
        </section>
      )}
      <div className="panel-heading">
        <h3>Últimos trabajos</h3>
        <button className="text-button" onClick={() => setRevision((n) => n + 1)}>
          Actualizar estados
        </button>
      </div>
      <ErrorMessage message={jobs.error} />
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th scope="col">Trabajo</th>
              <th scope="col">Fecha</th>
              <th scope="col">Estado</th>
              <th scope="col">Detalle</th>
            </tr>
          </thead>
          <tbody>
            {jobs.data?.items.map((j) => (
              <tr key={j.id}>
                <td>
                  {jobKinds[j.kind]}
                  {j.saleNumber ? ` · Venta #${String(j.saleNumber).padStart(5, '0')}` : ''}
                </td>
                <td>{date(j.createdAt, session.company.timezone)}</td>
                <td>{jobLabels[j.state]}</td>
                <td>{j.error || (j.opensDrawer ? 'Incluye apertura' : 'Sin apertura')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="form-note">
        Enviado significa entregado al dispositivo o a la cola del sistema. Ante un resultado
        incierto, revisa el papel antes de pedir una copia desde Ventas.
      </p>
    </section>
  );
}
